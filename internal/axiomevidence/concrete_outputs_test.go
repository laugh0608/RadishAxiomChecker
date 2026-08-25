package axiomevidence

import (
	"sort"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

func TestVerifyConcreteOutputsChecksLockedCorrectAndMismatchScenarios(t *testing.T) {
	tests := []struct {
		scenario           string
		hostExecutions     int
		checkedComparisons int
		failedComparisons  int
		replayed           int
		artifacts          int
	}{
		{"ax-b01-correct", 2, 2, 0, 0, 2},
		{"ax-b02-correct", 2, 2, 0, 0, 2},
		{"ax-b03-correct", 2, 2, 0, 0, 2},
		{"ax-b04-correct", 2, 1, 0, 0, 1},
		{"chk-concrete-01", 1, 0, 1, 3, 2},
	}
	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			document, ir, verified := concreteInputFixture(t, test.scenario)
			check, err := document.VerifyConcreteOutputs(
				ir, verified.ReadArtifact, concreteInputLimits(verified.Request),
			)
			if err != nil {
				t.Fatal(err)
			}
			if check.HostExecutions != test.hostExecutions ||
				check.CheckedComparisons != test.checkedComparisons ||
				check.FailedComparisons != test.failedComparisons ||
				check.ReplayedMismatches != test.replayed ||
				len(check.Artifacts) != test.artifacts {
				t.Fatalf("unexpected concrete output summary: %+v", check)
			}
		})
	}

	document, ir, verified := concreteInputFixture(t, "ax-b01-backend-timeout")
	check, err := document.VerifyConcreteOutputs(
		ir, verified.ReadArtifact, concreteInputLimits(verified.Request),
	)
	if err != nil {
		t.Fatal(err)
	}
	if check.UnknownObligations == 0 || check.HostExecutions != 0 ||
		check.CheckedComparisons != 0 || check.FailedComparisons != 0 ||
		len(check.Artifacts) != 0 {
		t.Fatalf("unknown output obligations were upgraded to a concrete decision: %+v", check)
	}
}

func TestVerifyConcreteOutputsRejectsBindingAndSemanticDrift(t *testing.T) {
	tests := []struct {
		name     string
		scenario string
		mutate   func(*testing.T, *Document)
		code     rejection.Code
	}{
		{
			name:     "output artifact format drift",
			scenario: "ax-b01-correct",
			mutate: func(t *testing.T, document *Document) {
				artifact := firstHostOutputArtifact(t, document)
				definition := document.artifacts[artifact]
				definition.format = "axiom-host-data"
				document.artifacts[artifact] = definition
			},
			code: rejection.ConcreteCheckMismatch,
		},
		{
			name:     "host output execution role omitted",
			scenario: "ax-b01-correct",
			mutate: func(t *testing.T, document *Document) {
				for id, execution := range document.executions {
					if execution.kind != "execute-host" {
						continue
					}
					execution.outputs[0].role = "host-outpot"
					document.executions[id] = execution
					return
				}
				t.Fatal("execute-host execution not found")
			},
			code: rejection.ConcreteCheckMismatch,
		},
		{
			name:     "checked host outputs swapped",
			scenario: "ax-b01-correct",
			mutate:   swapCheckedHostOutputs,
			code:     rejection.ConcreteCheckMismatch,
		},
		{
			name:     "checked comparison golden changed",
			scenario: "ax-b01-correct",
			mutate:   driftCheckedComparisonGolden,
			code:     rejection.ConcreteCheckMismatch,
		},
		{
			name:     "failed compare roles swapped",
			scenario: "chk-concrete-01",
			mutate: func(t *testing.T, document *Document) {
				for id, execution := range document.executions {
					if execution.kind != "compare-output" {
						continue
					}
					execution.inputs[0].role, execution.inputs[1].role =
						execution.inputs[1].role, execution.inputs[0].role
					document.executions[id] = execution
					return
				}
				t.Fatal("compare-output execution not found")
			},
			code: rejection.ConcreteCheckMismatch,
		},
		{
			name:     "failed observation actual expected swapped",
			scenario: "chk-concrete-01",
			mutate: func(t *testing.T, document *Document) {
				id, result := firstFailedOutputResult(t, document)
				result.counterexample.observed.actual, result.counterexample.observed.expected =
					result.counterexample.observed.expected, result.counterexample.observed.actual
				document.results[id] = result
			},
			code: rejection.CounterexampleInvalid,
		},
		{
			name:     "failed trace obligation drift",
			scenario: "chk-concrete-01",
			mutate: func(t *testing.T, document *Document) {
				id, result := firstFailedOutputResult(t, document)
				result.counterexample.trace[0].ref = protocol.Digest{}
				document.results[id] = result
			},
			code: rejection.CounterexampleInvalid,
		},
		{
			name:     "failed witness input drift",
			scenario: "chk-concrete-01",
			mutate: func(t *testing.T, document *Document) {
				id, result := firstFailedOutputResult(t, document)
				setConcreteField(
					t,
					&result.counterexample.worlds[0].Tables[0].Rows[0],
					"order_id",
					axiomir.ConcreteValue{Kind: "text", Text: "not-in-host-input"},
				)
				document.results[id] = result
			},
			code: rejection.CounterexampleInvalid,
		},
		{
			name:     "failed subject drift",
			scenario: "chk-concrete-01",
			mutate: func(t *testing.T, document *Document) {
				for id, definition := range document.obligations {
					if definition.Kind != "host-conformance" || document.results[id].kind != "failed" {
						continue
					}
					definition.Subject.Artifact = protocol.Digest{}
					document.obligations[id] = definition
					return
				}
				t.Fatal("failed host-conformance obligation not found")
			},
			code: rejection.CounterexampleInvalid,
		},
		{
			name:     "failed and checked share comparison",
			scenario: "chk-concrete-01",
			mutate: func(t *testing.T, document *Document) {
				id, result := firstFailedOutputResult(t, document)
				result.kind = "checked"
				document.results[id] = result
			},
			code: rejection.ConcreteCheckMismatch,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document, ir, verified := concreteInputFixture(t, test.scenario)
			test.mutate(t, &document)
			_, err := document.VerifyConcreteOutputs(
				ir, verified.ReadArtifact, concreteInputLimits(verified.Request),
			)
			assertCompletenessCode(t, err, test.code)
		})
	}
}

func TestVerifyConcreteOutputsRejectsDigestAndResourceDrift(t *testing.T) {
	document, ir, verified := concreteInputFixture(t, "ax-b01-correct")
	_, err := document.VerifyConcreteOutputs(
		ir,
		func(id protocol.Digest) ([]byte, error) {
			data, readErr := verified.ReadArtifact(id)
			return append(data, ' '), readErr
		},
		concreteInputLimits(verified.Request),
	)
	assertCompletenessCode(t, err, rejection.DigestMismatch)

	limits := concreteInputLimits(verified.Request)
	limits.MaxLogicalBytes = 1
	_, err = document.VerifyConcreteOutputs(ir, verified.ReadArtifact, limits)
	assertCompletenessCode(t, err, rejection.ResourceLimit)

	limits = concreteInputLimits(verified.Request)
	limits.MaxSemanticSteps = 1
	_, err = document.VerifyConcreteOutputs(ir, verified.ReadArtifact, limits)
	assertCompletenessCode(t, err, rejection.ResourceLimit)
}

func firstHostOutputArtifact(t *testing.T, document *Document) protocol.Digest {
	t.Helper()
	for _, execution := range document.executions {
		if execution.kind != "execute-host" {
			continue
		}
		if artifact, ok := singleExecutionArtifact(execution.outputs, "host-output"); ok {
			return artifact
		}
	}
	t.Fatal("host output artifact not found")
	return protocol.Digest{}
}

func firstFailedOutputResult(t *testing.T, document *Document) (protocol.Digest, obligationResult) {
	t.Helper()
	for id, definition := range document.obligations {
		result := document.results[id]
		if (definition.Kind == "host-conformance" || definition.Kind == "output-conformance") &&
			result.kind == "failed" {
			return id, result
		}
	}
	t.Fatal("failed output result not found")
	return protocol.Digest{}, obligationResult{}
}

func swapCheckedHostOutputs(t *testing.T, document *Document) {
	t.Helper()
	ids := make([]protocol.Digest, 0, 2)
	for id, execution := range document.executions {
		if execution.kind == "execute-host" && execution.result.kind == "completed" {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(left, right int) bool { return ids[left].String() < ids[right].String() })
	if len(ids) != 2 {
		t.Fatalf("expected two execute-host executions, got %d", len(ids))
	}
	left := document.executions[ids[0]]
	right := document.executions[ids[1]]
	left.outputs[0].artifact, right.outputs[0].artifact =
		right.outputs[0].artifact, left.outputs[0].artifact
	document.executions[ids[0]] = left
	document.executions[ids[1]] = right

	for obligationID, definition := range document.obligations {
		result := document.results[obligationID]
		if definition.Kind != "host-conformance" || result.kind != "checked" {
			continue
		}
		execution := document.executions[result.execution]
		input, output, ok := hostExecutionArtifacts(execution)
		if !ok {
			t.Fatal("mutated execute-host boundary is invalid")
		}
		definition.Subject.Artifact = output
		result.artifacts = uniqueDigests(input, output)
		document.obligations[obligationID] = definition
		document.results[obligationID] = result
	}
}

func driftCheckedComparisonGolden(t *testing.T, document *Document) {
	t.Helper()
	outputs := make([]protocol.Digest, 0, 2)
	for _, execution := range document.executions {
		if execution.kind != "execute-host" {
			continue
		}
		_, output, ok := hostExecutionArtifacts(execution)
		if ok {
			outputs = append(outputs, output)
		}
	}
	sort.Slice(outputs, func(left, right int) bool { return outputs[left].String() < outputs[right].String() })
	if len(outputs) != 2 || outputs[0] == outputs[1] {
		t.Fatal("expected two distinct host outputs")
	}
	for executionID, execution := range document.executions {
		if execution.kind != "compare-output" {
			continue
		}
		actual, expected, ok := comparisonArtifacts(execution)
		if !ok || actual != expected {
			continue
		}
		replacement := outputs[0]
		if replacement == expected {
			replacement = outputs[1]
		}
		for index := range execution.inputs {
			if execution.inputs[index].role == "golden-output" {
				execution.inputs[index].artifact = replacement
			}
		}
		document.executions[executionID] = execution
		for obligationID, definition := range document.obligations {
			result := document.results[obligationID]
			if definition.Kind == "output-conformance" &&
				result.kind == "checked" &&
				result.execution == executionID {
				result.artifacts = uniqueDigests(actual, replacement)
				document.results[obligationID] = result
			}
		}
		return
	}
	t.Fatal("checked self-comparison not found")
}
