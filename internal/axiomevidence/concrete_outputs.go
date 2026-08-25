package axiomevidence

import (
	"crypto/sha256"

	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

// ConcreteOutputCheck reports only output artifacts and dynamic relationships
// independently decided by this call. Unknown output obligations remain
// outside the concrete comparison count.
type ConcreteOutputCheck struct {
	Artifacts          []protocol.Digest
	HostExecutions     int
	CheckedComparisons int
	FailedComparisons  int
	CheckedObligations int
	FailedObligations  int
	UnknownObligations int
	ReplayedMismatches int
}

type outputBinding struct {
	id         protocol.Digest
	definition axiomir.ObligationDefinition
	result     obligationResult
}

type hostObservation struct {
	inputArtifact  protocol.Digest
	outputArtifact protocol.Digest
	input          axiomir.BenchmarkInput
	output         axiomir.BenchmarkOutput
	semantic       axiomir.ConcreteWorld
	equal          bool
}

type comparisonObservation struct {
	actualArtifact   protocol.Digest
	expectedArtifact protocol.Digest
	actual           axiomir.BenchmarkOutput
	expected         axiomir.BenchmarkOutput
	equal            bool
}

// VerifyConcreteOutputs reconstructs bound host/golden artifacts, executes the
// retained IR on every relevant host input, and independently decides checked
// and failed host/output conformance relationships. It does not execute a
// production target, check minimality or proof support, recompute conclusion,
// or produce an independent four-state result.
func (document Document) VerifyConcreteOutputs(
	ir axiomir.Document,
	read ArtifactReader,
	limits ConcreteOutputLimits,
) (ConcreteOutputCheck, error) {
	if err := document.VerifyIRSubject(ir.ContentDigest, ir.DomainDigest); err != nil {
		return ConcreteOutputCheck{}, err
	}
	if read == nil {
		return ConcreteOutputCheck{}, rejection.New(rejection.ArtifactMissing, "concrete output artifact reader is unavailable")
	}

	hostBindings := make(map[protocol.Digest][]outputBinding)
	comparisonBindings := make(map[protocol.Digest][]outputBinding)
	summary := ConcreteOutputCheck{}
	obligationIDs := make([]protocol.Digest, 0, len(document.obligations))
	for id := range document.obligations {
		obligationIDs = append(obligationIDs, id)
	}
	sortDigests(obligationIDs)
	for _, id := range obligationIDs {
		definition := document.obligations[id]
		if definition.Kind != "host-conformance" && definition.Kind != "output-conformance" {
			continue
		}
		result, exists := document.results[id]
		if !exists {
			return ConcreteOutputCheck{}, concreteMismatch("output conformance obligation result is unavailable")
		}
		switch result.kind {
		case "unknown":
			summary.UnknownObligations++
			continue
		case "checked":
			summary.CheckedObligations++
		case "failed":
			summary.FailedObligations++
		default:
			return ConcreteOutputCheck{}, concreteMismatch("output conformance result is not checked, failed, or unknown")
		}
		execution, exists := document.executions[result.execution]
		if !exists || execution.result.kind != "completed" {
			return ConcreteOutputCheck{}, concreteMismatch("decidable output conformance does not bind a completed execution")
		}
		binding := outputBinding{id: id, definition: definition, result: result}
		if result.kind == "checked" && definition.Kind == "host-conformance" {
			if execution.kind != "execute-host" {
				return ConcreteOutputCheck{}, concreteMismatch("checked host conformance does not bind execute-host")
			}
			hostBindings[result.execution] = append(hostBindings[result.execution], binding)
			continue
		}
		if execution.kind != "compare-output" {
			return ConcreteOutputCheck{}, concreteMismatch("output comparison result does not bind compare-output")
		}
		comparisonBindings[result.execution] = append(comparisonBindings[result.execution], binding)
	}

	comparisonIDs := make([]protocol.Digest, 0, len(comparisonBindings))
	actualArtifacts := make(map[protocol.Digest]struct{})
	for id := range comparisonBindings {
		comparisonIDs = append(comparisonIDs, id)
		execution := document.executions[id]
		actual, expected, ok := comparisonArtifacts(execution)
		if !ok {
			return ConcreteOutputCheck{}, concreteMismatch("compare-output execution has an invalid actual/golden boundary")
		}
		actualArtifacts[actual] = struct{}{}
		_ = expected
	}
	sortDigests(comparisonIDs)

	hostIDs := make(map[protocol.Digest]struct{}, len(hostBindings))
	for id := range hostBindings {
		hostIDs[id] = struct{}{}
	}
	producers := make(map[protocol.Digest][]protocol.Digest)
	for id, execution := range document.executions {
		if execution.kind != "execute-host" || execution.result.kind != "completed" {
			continue
		}
		_, output, ok := hostExecutionArtifacts(execution)
		if !ok {
			continue
		}
		if _, relevant := actualArtifacts[output]; relevant {
			hostIDs[id] = struct{}{}
			producers[output] = append(producers[output], id)
		}
	}
	for actual := range actualArtifacts {
		if len(producers[actual]) == 0 {
			return ConcreteOutputCheck{}, concreteMismatch("actual output has no completed execute-host producer")
		}
		sortDigests(producers[actual])
	}

	outputArtifacts := make(map[protocol.Digest]struct{})
	inputCache := make(map[protocol.Digest]axiomir.BenchmarkInput)
	outputCache := make(map[protocol.Digest]axiomir.BenchmarkOutput)
	readBytes := func(id protocol.Digest) ([]byte, error) {
		definition, exists := document.artifacts[id]
		if !exists || definition.format != "axiom-benchmark-data" || definition.formatVersion != "0.1" {
			return nil, concreteMismatch("concrete data artifact is not declared as axiom-benchmark-data 0.1")
		}
		data, err := read(id)
		if err != nil {
			return nil, err
		}
		if protocol.Digest(sha256.Sum256(data)) != id {
			return nil, rejection.New(rejection.DigestMismatch, "concrete data bytes do not match their artifact digest")
		}
		return data, nil
	}
	decodeInput := func(id protocol.Digest) (axiomir.BenchmarkInput, error) {
		if cached, exists := inputCache[id]; exists {
			return cached, nil
		}
		data, err := readBytes(id)
		if err != nil {
			return axiomir.BenchmarkInput{}, err
		}
		decoded, err := ir.DecodeBenchmarkInput(data, limits.JSON)
		if err != nil {
			return axiomir.BenchmarkInput{}, err
		}
		if decoded.Role != "input" {
			return axiomir.BenchmarkInput{}, concreteMismatch("execute-host input is not a valid benchmark input")
		}
		if limits.MaxLogicalBytes == 0 || decoded.LogicalBytes() > limits.MaxLogicalBytes {
			return axiomir.BenchmarkInput{}, rejection.New(rejection.ResourceLimit, "concrete host input exceeds its logical-memory limit")
		}
		inputCache[id] = decoded
		return decoded, nil
	}
	decodeOutput := func(id protocol.Digest) (axiomir.BenchmarkOutput, error) {
		if cached, exists := outputCache[id]; exists {
			return cached, nil
		}
		data, err := readBytes(id)
		if err != nil {
			return axiomir.BenchmarkOutput{}, err
		}
		decoded, err := ir.DecodeBenchmarkOutput(data, limits.JSON)
		if err != nil {
			return axiomir.BenchmarkOutput{}, err
		}
		if limits.MaxLogicalBytes == 0 || decoded.LogicalBytes() > limits.MaxLogicalBytes {
			return axiomir.BenchmarkOutput{}, rejection.New(rejection.ResourceLimit, "concrete output exceeds its logical-memory limit")
		}
		if check := ir.CheckCompleteOutputWorld(decoded.World); !check.WellFormed {
			return axiomir.BenchmarkOutput{}, concreteMismatch("concrete output is not a complete WF IR output world")
		}
		outputArtifacts[id] = struct{}{}
		outputCache[id] = decoded
		return decoded, nil
	}

	hostIDList := make([]protocol.Digest, 0, len(hostIDs))
	for id := range hostIDs {
		hostIDList = append(hostIDList, id)
	}
	sortDigests(hostIDList)
	hostObservations := make(map[protocol.Digest]hostObservation, len(hostIDList))
	for _, id := range hostIDList {
		execution := document.executions[id]
		inputArtifact, outputArtifact, ok := hostExecutionArtifacts(execution)
		if !ok {
			return ConcreteOutputCheck{}, concreteMismatch("execute-host execution has an invalid host input/output boundary")
		}
		input, err := decodeInput(inputArtifact)
		if err != nil {
			return ConcreteOutputCheck{}, err
		}
		output, err := decodeOutput(outputArtifact)
		if err != nil {
			return ConcreteOutputCheck{}, err
		}
		semantic, err := ir.Execute(input.World, axiomir.ExecutionLimits{
			MaxLogicalBytes:  limits.MaxLogicalBytes,
			MaxSemanticSteps: limits.MaxSemanticSteps,
		})
		if err != nil {
			if code, ok := rejection.CodeOf(err); ok && code == rejection.ResourceLimit {
				return ConcreteOutputCheck{}, err
			}
			return ConcreteOutputCheck{}, concreteMismatch("host input could not be independently executed to a concrete output")
		}
		observation := hostObservation{
			inputArtifact: inputArtifact, outputArtifact: outputArtifact,
			input: input, output: output, semantic: semantic.Outputs,
			equal: axiomir.ConcreteWorldsEqual(semantic.Outputs, output.World),
		}
		hostObservations[id] = observation
		for _, binding := range hostBindings[id] {
			if binding.definition.Subject.Kind != "artifact" || binding.definition.Subject.Artifact != outputArtifact {
				return ConcreteOutputCheck{}, concreteMismatch("checked host-conformance subject does not bind its host output")
			}
			if !observation.equal {
				return ConcreteOutputCheck{}, concreteMismatch("checked host-conformance differs from independent IR execution")
			}
			if !equalDigestSets(binding.result.artifacts, []protocol.Digest{inputArtifact, outputArtifact}) {
				return ConcreteOutputCheck{}, concreteMismatch("checked host-conformance artifact closure differs from execute-host")
			}
		}
	}
	summary.HostExecutions = len(hostIDList)

	for _, id := range comparisonIDs {
		execution := document.executions[id]
		actualArtifact, expectedArtifact, _ := comparisonArtifacts(execution)
		actual, err := decodeOutput(actualArtifact)
		if err != nil {
			return ConcreteOutputCheck{}, err
		}
		expected, err := decodeOutput(expectedArtifact)
		if err != nil {
			return ConcreteOutputCheck{}, err
		}
		comparison := comparisonObservation{
			actualArtifact: actualArtifact, expectedArtifact: expectedArtifact,
			actual: actual, expected: expected,
			equal: axiomir.ConcreteWorldsEqual(actual.World, expected.World),
		}
		hasChecked := false
		hasFailed := false
		for _, binding := range comparisonBindings[id] {
			hasChecked = hasChecked || binding.result.kind == "checked"
			hasFailed = hasFailed || binding.result.kind == "failed"
		}
		if hasChecked && hasFailed {
			return ConcreteOutputCheck{}, concreteMismatch("one comparison execution cannot support checked and failed output relations")
		}
		if hasChecked {
			if !comparison.equal {
				return ConcreteOutputCheck{}, concreteMismatch("checked output comparison has unequal actual and golden outputs")
			}
			for _, producer := range producers[actualArtifact] {
				if !hostObservations[producer].equal {
					return ConcreteOutputCheck{}, concreteMismatch("checked output comparison actual differs from independent IR execution")
				}
			}
			for _, binding := range comparisonBindings[id] {
				if err := validateCheckedOutputBinding(binding, comparison); err != nil {
					return ConcreteOutputCheck{}, err
				}
			}
			summary.CheckedComparisons++
			continue
		}
		if !hasFailed || comparison.equal {
			return ConcreteOutputCheck{}, concreteMismatch("failed output comparison does not contain unequal actual and golden outputs")
		}
		for _, binding := range comparisonBindings[id] {
			if err := validateFailedOutputBinding(
				binding, comparison, producers[actualArtifact], hostObservations,
			); err != nil {
				return ConcreteOutputCheck{}, err
			}
			summary.ReplayedMismatches++
		}
		summary.FailedComparisons++
	}

	summary.Artifacts = make([]protocol.Digest, 0, len(outputArtifacts))
	for artifact := range outputArtifacts {
		summary.Artifacts = append(summary.Artifacts, artifact)
	}
	sortDigests(summary.Artifacts)
	return summary, nil
}

func hostExecutionArtifacts(execution executionDefinition) (protocol.Digest, protocol.Digest, bool) {
	if execution.kind != "execute-host" || execution.result.kind != "completed" ||
		len(execution.inputs) != 2 || len(execution.outputs) != 1 {
		return protocol.Digest{}, protocol.Digest{}, false
	}
	input, inputOK := singleExecutionArtifact(execution.inputs, "host-input")
	_, targetOK := singleExecutionArtifact(execution.inputs, "target-module")
	output, outputOK := singleExecutionArtifact(execution.outputs, "host-output")
	return input, output, inputOK && targetOK && outputOK
}

func comparisonArtifacts(execution executionDefinition) (protocol.Digest, protocol.Digest, bool) {
	if execution.kind != "compare-output" || execution.result.kind != "completed" ||
		len(execution.inputs) != 2 || len(execution.outputs) != 0 {
		return protocol.Digest{}, protocol.Digest{}, false
	}
	actual, actualOK := singleExecutionArtifact(execution.inputs, "actual-output")
	expected, expectedOK := singleExecutionArtifact(execution.inputs, "golden-output")
	return actual, expected, actualOK && expectedOK
}

func validateCheckedOutputBinding(binding outputBinding, comparison comparisonObservation) error {
	if !equalDigestSets(binding.result.artifacts, uniqueDigests(
		comparison.actualArtifact, comparison.expectedArtifact,
	)) {
		return concreteMismatch("checked output-conformance artifact closure differs from compare-output")
	}
	switch binding.definition.Subject.Kind {
	case "artifact":
		if binding.definition.Subject.Artifact != comparison.actualArtifact &&
			binding.definition.Subject.Artifact != comparison.expectedArtifact {
			return concreteMismatch("checked output-conformance artifact subject is outside its comparison")
		}
	case "interface":
		if binding.definition.Subject.Direction != "output" ||
			!worldHasInterface(comparison.expected.World, binding.definition.Subject.Name) {
			return concreteMismatch("checked output-conformance interface subject is outside its output world")
		}
	default:
		return concreteMismatch("checked output-conformance has an unsupported subject")
	}
	return nil
}

func validateFailedOutputBinding(
	binding outputBinding,
	comparison comparisonObservation,
	producers []protocol.Digest,
	hostObservations map[protocol.Digest]hostObservation,
) error {
	counterexample := binding.result.counterexample
	if counterexample == nil || counterexample.observed.kind != "host-output-mismatch" ||
		counterexample.observed.actual != comparison.actualArtifact ||
		counterexample.observed.expected != comparison.expectedArtifact {
		return invalidCounterexample("output mismatch observation does not bind compare-output actual and golden artifacts")
	}
	if len(counterexample.trace) != 2 ||
		counterexample.trace[0].kind != "obligation" ||
		counterexample.trace[0].ref != binding.id ||
		counterexample.trace[1].kind != "observation" ||
		counterexample.trace[1].value != "failed" {
		return invalidCounterexample("output mismatch trace does not bind obligation and failed observation")
	}
	if len(counterexample.worlds) != 1 {
		return invalidCounterexample("output mismatch must retain one input-world witness")
	}
	switch binding.definition.Kind {
	case "host-conformance":
		if binding.definition.Subject.Kind != "artifact" ||
			binding.definition.Subject.Artifact != comparison.actualArtifact {
			return invalidCounterexample("failed host-conformance subject does not bind actual output")
		}
	case "output-conformance":
		switch binding.definition.Subject.Kind {
		case "artifact":
			if binding.definition.Subject.Artifact != comparison.expectedArtifact {
				return invalidCounterexample("failed output-conformance artifact subject does not bind golden output")
			}
		case "interface":
			if binding.definition.Subject.Direction != "output" ||
				!worldHasInterface(comparison.expected.World, binding.definition.Subject.Name) {
				return invalidCounterexample("failed output-conformance interface subject is outside the golden output")
			}
		default:
			return invalidCounterexample("failed output-conformance has an unsupported subject")
		}
	default:
		return invalidCounterexample("output mismatch is attached to an unsupported obligation kind")
	}
	for _, producer := range producers {
		observation, exists := hostObservations[producer]
		if !exists || !axiomir.WorldContainsProjection(observation.input.World, counterexample.worlds[0]) {
			continue
		}
		if observation.equal {
			continue
		}
		if axiomir.ConcreteWorldsEqual(observation.semantic, comparison.expected.World) {
			return nil
		}
	}
	return invalidCounterexample("output mismatch witness is not supported by independent IR output")
}

func uniqueDigests(values ...protocol.Digest) []protocol.Digest {
	set := make(map[protocol.Digest]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	result := make([]protocol.Digest, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sortDigests(result)
	return result
}

func worldHasInterface(world axiomir.ConcreteWorld, name string) bool {
	for _, table := range world.Tables {
		if table.Name == name {
			return true
		}
	}
	return false
}
