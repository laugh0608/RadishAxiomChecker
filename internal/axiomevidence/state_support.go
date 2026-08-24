package axiomevidence

import (
	"sort"

	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

var executionRoles = map[string]string{
	"check-certificate":     "certificate-checker",
	"check-fixture":         "fixture-checker",
	"compare-output":        "output-comparator",
	"execute-host":          "host-executor",
	"generate-obligations":  "obligation-generator",
	"normalize":             "ir-normalizer",
	"prove":                 "prover",
	"replay-counterexample": "counterexample-replayer",
}

// VerifyStateSupport checks the closed state/support relationships exercised by
// the locked Evidence v0.1 profile. It checks expectation/state compatibility,
// execution completion, exact tool roles, attempt reasons, proof-attestation
// bindings, trusted scope identity, and the artifact closure declared by finite
// checked results. It does not replay proof rules, certificates, concrete data,
// or counterexamples, and it does not recompute the Evidence conclusion.
func (d Document) VerifyStateSupport() error {
	if len(d.results) != len(d.obligations) {
		return invalidStateSupport("Axiom Evidence obligation result cardinality mismatch")
	}

	executionIDs := make([]protocol.Digest, 0, len(d.executions))
	for id := range d.executions {
		executionIDs = append(executionIDs, id)
	}
	sortDigests(executionIDs)
	for _, id := range executionIDs {
		execution := d.executions[id]
		role, ok := executionRoles[execution.kind]
		if !ok {
			return invalidStateSupport("Axiom Evidence execution has no closed tool-role mapping")
		}
		tool, ok := d.toolsForStateSupport(execution.tool)
		if !ok {
			return invalidStateSupport("Axiom Evidence execution tool is unavailable")
		}
		if _, ok := tool.roles[role]; !ok {
			return invalidStateSupport("Axiom Evidence execution tool lacks the role required by its kind")
		}
	}

	obligationIDs := make([]protocol.Digest, 0, len(d.obligations))
	for id := range d.obligations {
		obligationIDs = append(obligationIDs, id)
	}
	sortDigests(obligationIDs)
	for _, id := range obligationIDs {
		definition := d.obligations[id]
		result, ok := d.results[id]
		if !ok {
			return invalidStateSupport("Axiom Evidence obligation has no result")
		}
		if !allowedState(definition.Expectation, result.kind) {
			return invalidStateSupport("Axiom Evidence result state cannot satisfy or block its expectation")
		}
		if err := d.verifyResultState(definition, result); err != nil {
			return err
		}
	}
	return nil
}

// toolsForStateSupport deliberately keeps the tool index private to the
// structural document while making direct test mutations fail closed.
func (d Document) toolsForStateSupport(id protocol.Digest) (toolDefinition, bool) {
	tool, ok := d.tools[id]
	return tool, ok
}

func allowedState(expectation, state string) bool {
	switch expectation {
	case "prove":
		return state == "proved" || state == "failed" || state == "unknown"
	case "check":
		return state == "checked" || state == "failed" || state == "unknown"
	case "trust":
		return state == "trusted" || state == "unknown"
	default:
		return false
	}
}

func (d Document) verifyResultState(definition axiomir.ObligationDefinition, result obligationResult) error {
	switch result.kind {
	case "proved":
		if err := d.verifyTrustRefs(result.assumptions); err != nil {
			return err
		}
		return d.verifyProofSupport(result)
	case "checked":
		if err := d.verifyTrustRefs(result.assumptions); err != nil {
			return err
		}
		return d.verifyCheckedResult(definition, result)
	case "unknown":
		return d.verifyUnknownResult(definition, result)
	case "failed":
		if err := d.verifyTrustRefs(result.assumptions); err != nil {
			return err
		}
		return d.verifyFailedResult(definition, result)
	case "trusted":
		return d.verifyTrustedResult(definition, result)
	default:
		return invalidStateSupport("Axiom Evidence result has an unsupported state")
	}
}

func (d Document) verifyProofSupport(result obligationResult) error {
	execution, ok := d.executions[result.support.execution]
	if !ok || execution.kind != "prove" || execution.result.kind != "completed" {
		return invalidStateSupport("Axiom Evidence proved support does not bind a completed prove execution")
	}
	query, queryOK := singleExecutionArtifact(execution.inputs, "query")
	_, obligationSetOK := singleExecutionArtifact(execution.inputs, "obligation-set")
	response, responseOK := singleExecutionArtifact(execution.outputs, "response")
	if !queryOK || !obligationSetOK || !responseOK || len(execution.inputs) != 2 || len(execution.outputs) != 1 {
		return invalidStateSupport("Axiom Evidence proved support execution has an invalid query/response boundary")
	}
	switch result.support.kind {
	case "kernel-replay":
		return nil
	case "backend-attestation":
		if result.support.query != query || result.support.response != response {
			return invalidStateSupport("Axiom Evidence backend attestation does not bind its execution query and response")
		}
		trust, ok := d.trust[result.support.trust]
		if !ok || trust.category != "proof-backend" {
			return invalidStateSupport("Axiom Evidence backend attestation does not bind proof-backend trust")
		}
		if !containsDigest(result.assumptions, result.support.trust) {
			return invalidStateSupport("Axiom Evidence backend attestation trust is absent from assumptions")
		}
		return nil
	default:
		return invalidStateSupport("Axiom Evidence proved result has no supported proof binding")
	}
}

func (d Document) verifyCheckedResult(definition axiomir.ObligationDefinition, result obligationResult) error {
	execution, ok := d.executions[result.execution]
	if !ok || execution.result.kind != "completed" {
		return invalidStateSupport("Axiom Evidence checked result does not bind a completed execution")
	}

	var expectedKind string
	var roles map[string]struct{}
	switch definition.Kind {
	case "ir-structure":
		expectedKind = "check-fixture"
	case "input-conformance":
		expectedKind = "check-fixture"
		roles = roleSet("host-input")
	case "host-conformance":
		expectedKind = "execute-host"
		roles = roleSet("host-input", "host-output")
	case "output-conformance":
		expectedKind = "compare-output"
		roles = roleSet("actual-output", "golden-output")
	default:
		return invalidStateSupport("Axiom Evidence checked result is attached to a non-dynamic obligation")
	}
	if execution.kind != expectedKind {
		return invalidStateSupport("Axiom Evidence checked result uses the wrong execution kind")
	}

	expectedArtifacts := executionArtifactSet(execution, roles)
	if !equalDigestSets(result.artifacts, expectedArtifacts) {
		return invalidStateSupport("Axiom Evidence checked result artifact closure is incomplete or excessive")
	}
	for _, artifact := range result.artifacts {
		if _, ok := d.artifacts[artifact]; !ok {
			return invalidStateSupport("Axiom Evidence checked result references an unknown artifact")
		}
	}
	if definition.Subject.Kind == "artifact" && !containsDigest(result.artifacts, definition.Subject.Artifact) {
		return invalidStateSupport("Axiom Evidence checked result omits its artifact subject")
	}
	return nil
}

func (d Document) verifyUnknownResult(definition axiomir.ObligationDefinition, result obligationResult) error {
	if len(result.attempts) == 0 {
		return invalidStateSupport("Axiom Evidence unknown result has no attempt")
	}
	wantResult, ok := executionResultForUnknownReason(result.reason)
	if !ok {
		return invalidStateSupport("Axiom Evidence unknown reason has no closed execution-result mapping")
	}
	wantKind, ok := unknownExecutionKind(definition)
	if !ok {
		return invalidStateSupport("Axiom Evidence unknown state is unsupported for this obligation")
	}
	for _, attempt := range result.attempts {
		execution, ok := d.executions[attempt]
		if !ok || execution.kind != wantKind || execution.result.kind != wantResult {
			return invalidStateSupport("Axiom Evidence unknown attempt does not match its expectation or reason")
		}
	}
	return nil
}

func (d Document) verifyFailedResult(definition axiomir.ObligationDefinition, result obligationResult) error {
	execution, ok := d.executions[result.execution]
	if !ok || execution.result.kind != "completed" {
		return invalidStateSupport("Axiom Evidence failed result does not bind a completed replay or comparison")
	}
	wantKind := ""
	if definition.Expectation == "prove" || definition.Kind == "input-conformance" {
		wantKind = "replay-counterexample"
	} else if definition.Kind == "host-conformance" || definition.Kind == "output-conformance" {
		wantKind = "compare-output"
	}
	if execution.kind != wantKind || wantKind == "" {
		return invalidStateSupport("Axiom Evidence failed result uses the wrong execution kind")
	}
	if wantKind == "replay-counterexample" {
		if _, ok := singleExecutionArtifact(execution.inputs, "host-input"); !ok || len(execution.outputs) != 0 {
			return invalidStateSupport("Axiom Evidence failed replay has an invalid artifact boundary")
		}
	} else {
		if _, ok := singleExecutionArtifact(execution.inputs, "actual-output"); !ok {
			return invalidStateSupport("Axiom Evidence failed comparison omits actual output")
		}
		if _, ok := singleExecutionArtifact(execution.inputs, "golden-output"); !ok || len(execution.outputs) != 0 {
			return invalidStateSupport("Axiom Evidence failed comparison has an invalid golden-output boundary")
		}
	}
	return nil
}

func (d Document) verifyTrustedResult(definition axiomir.ObligationDefinition, result obligationResult) error {
	if definition.Kind != "trust-boundary" || definition.Subject.Kind != "trust" || result.trust != definition.Subject.Scope {
		return invalidStateSupport("Axiom Evidence trusted result does not complete the same trust-boundary")
	}
	trust, ok := d.trust[result.trust]
	if !ok || trust.category != definition.Subject.Category {
		return invalidStateSupport("Axiom Evidence trusted result category differs from its trust definition")
	}
	return nil
}

func (d Document) verifyTrustRefs(refs []protocol.Digest) error {
	for _, ref := range refs {
		if _, ok := d.trust[ref]; !ok {
			return invalidStateSupport("Axiom Evidence result assumption is not a declared trust entry")
		}
	}
	return nil
}

func unknownExecutionKind(definition axiomir.ObligationDefinition) (string, bool) {
	switch definition.Expectation {
	case "prove":
		return "prove", true
	case "check":
		switch definition.Kind {
		case "ir-structure", "input-conformance":
			return "check-fixture", true
		case "host-conformance":
			return "execute-host", true
		case "output-conformance":
			return "compare-output", true
		}
	}
	return "", false
}

func executionResultForUnknownReason(reason string) (string, bool) {
	switch reason {
	case "timeout":
		return "timeout", true
	case "resource-exhausted":
		return "resource-exhausted", true
	case "backend-unavailable":
		return "unavailable", true
	case "incomplete-certificate", "unsupported":
		return "unsupported", true
	case "indeterminate":
		return "completed", true
	case "operational-error":
		return "error", true
	default:
		return "", false
	}
}

func executionArtifactSet(execution executionDefinition, roles map[string]struct{}) []protocol.Digest {
	values := make(map[protocol.Digest]struct{})
	appendValues := func(items []executionIO) {
		for _, item := range items {
			if roles != nil {
				if _, ok := roles[item.role]; !ok {
					continue
				}
			}
			values[item.artifact] = struct{}{}
		}
	}
	appendValues(execution.inputs)
	appendValues(execution.outputs)
	result := make([]protocol.Digest, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sortDigests(result)
	return result
}

func singleExecutionArtifact(items []executionIO, role string) (protocol.Digest, bool) {
	var result protocol.Digest
	found := false
	for _, item := range items {
		if item.role != role {
			continue
		}
		if found {
			return protocol.Digest{}, false
		}
		result = item.artifact
		found = true
	}
	return result, found
}

func equalDigestSets(left, right []protocol.Digest) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy := append([]protocol.Digest(nil), left...)
	rightCopy := append([]protocol.Digest(nil), right...)
	sortDigests(leftCopy)
	sortDigests(rightCopy)
	for index := range leftCopy {
		if leftCopy[index] != rightCopy[index] {
			return false
		}
	}
	return true
}

func containsDigest(values []protocol.Digest, target protocol.Digest) bool {
	index := sort.Search(len(values), func(index int) bool {
		return values[index].String() >= target.String()
	})
	return index != len(values) && values[index] == target
}

func roleSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func invalidStateSupport(detail string) error {
	return rejection.New(rejection.InvalidStateSupport, detail)
}
