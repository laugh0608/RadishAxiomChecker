package axiomevidence

import (
	"sort"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

func TestVerifyStateSupportRejectsClosedRelationshipDrift(t *testing.T) {
	tests := []struct {
		name     string
		scenario string
		mutate   func(*testing.T, *Document)
	}{
		{
			name:     "expectation state mismatch",
			scenario: "ax-b01-correct",
			mutate: func(t *testing.T, document *Document) {
				id, _, result := findObligationResult(t, document, func(definition axiomir.ObligationDefinition, result obligationResult) bool {
					return definition.Expectation == "prove" && result.kind == "proved"
				})
				result.kind = "checked"
				document.results[id] = result
			},
		},
		{
			name:     "missing proof support",
			scenario: "ax-b01-correct",
			mutate: func(t *testing.T, document *Document) {
				id, _, result := findObligationResult(t, document, func(_ axiomir.ObligationDefinition, result obligationResult) bool {
					return result.kind == "proved"
				})
				result.support = proofSupport{}
				document.results[id] = result
			},
		},
		{
			name:     "wrong execution tool role",
			scenario: "ax-b01-wrong-add",
			mutate: func(t *testing.T, document *Document) {
				_, execution := findExecution(t, document, func(execution executionDefinition) bool {
					return execution.kind == "replay-counterexample"
				})
				tool := document.tools[execution.tool]
				delete(tool.roles, "counterexample-replayer")
				document.tools[execution.tool] = tool
			},
		},
		{
			name:     "proof execution incomplete",
			scenario: "ax-b01-correct",
			mutate: func(t *testing.T, document *Document) {
				_, _, result := findObligationResult(t, document, func(_ axiomir.ObligationDefinition, result obligationResult) bool {
					return result.kind == "proved"
				})
				execution := document.executions[result.support.execution]
				execution.result = executionResult{kind: "timeout", code: "synthetic-timeout"}
				document.executions[result.support.execution] = execution
			},
		},
		{
			name:     "attempt reason drift",
			scenario: "ax-b01-backend-timeout",
			mutate: func(t *testing.T, document *Document) {
				id, _, result := findObligationResult(t, document, func(_ axiomir.ObligationDefinition, result obligationResult) bool {
					return result.kind == "unknown" && result.reason == "timeout"
				})
				result.reason = "backend-unavailable"
				document.results[id] = result
			},
		},
		{
			name:     "attestation trust missing from assumptions",
			scenario: "ax-b01-correct",
			mutate: func(t *testing.T, document *Document) {
				id, _, result := findObligationResult(t, document, func(_ axiomir.ObligationDefinition, result obligationResult) bool {
					return result.kind == "proved" && result.support.kind == "backend-attestation"
				})
				result.assumptions = nil
				document.results[id] = result
			},
		},
		{
			name:     "attestation response drift",
			scenario: "ax-b01-correct",
			mutate: func(t *testing.T, document *Document) {
				id, _, result := findObligationResult(t, document, func(_ axiomir.ObligationDefinition, result obligationResult) bool {
					return result.kind == "proved" && result.support.kind == "backend-attestation"
				})
				result.support.response = result.support.query
				document.results[id] = result
			},
		},
		{
			name:     "checked artifact closure omission",
			scenario: "ax-b01-correct",
			mutate: func(t *testing.T, document *Document) {
				id, _, result := findObligationResult(t, document, func(_ axiomir.ObligationDefinition, result obligationResult) bool {
					return result.kind == "checked" && len(result.artifacts) > 1
				})
				result.artifacts = result.artifacts[1:]
				document.results[id] = result
			},
		},
		{
			name:     "trusted scope mismatch",
			scenario: "ax-b01-correct",
			mutate: func(t *testing.T, document *Document) {
				id, definition, result := findObligationResult(t, document, func(_ axiomir.ObligationDefinition, result obligationResult) bool {
					return result.kind == "trusted"
				})
				result.trust = anotherTrust(t, document, definition.Subject.Scope)
				document.results[id] = result
			},
		},
		{
			name:     "failed execution kind mismatch",
			scenario: "ax-b01-wrong-add",
			mutate: func(t *testing.T, document *Document) {
				id, _, result := findObligationResult(t, document, func(_ axiomir.ObligationDefinition, result obligationResult) bool {
					return result.kind == "failed"
				})
				proveID, _ := findExecution(t, document, func(execution executionDefinition) bool {
					return execution.kind == "prove" && execution.result.kind == "completed"
				})
				result.execution = proveID
				document.results[id] = result
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document, _ := completenessFixture(t, test.scenario)
			test.mutate(t, &document)
			assertCompletenessCode(t, document.VerifyStateSupport(), rejection.InvalidStateSupport)
		})
	}
}

func TestVerifyStateSupportIsStableAcrossRepeatedValidation(t *testing.T) {
	document, ir := completenessFixture(t, "ax-b03-correct")
	if err := document.VerifyObligationCompleteness(ir); err != nil {
		t.Fatal(err)
	}
	for iteration := 0; iteration < 100; iteration++ {
		if err := document.VerifyStateSupport(); err != nil {
			t.Fatalf("iteration %d: %v", iteration, err)
		}
	}
}

func findObligationResult(
	t *testing.T,
	document *Document,
	match func(axiomir.ObligationDefinition, obligationResult) bool,
) (protocol.Digest, axiomir.ObligationDefinition, obligationResult) {
	t.Helper()
	ids := make([]protocol.Digest, 0, len(document.obligations))
	for id := range document.obligations {
		ids = append(ids, id)
	}
	sortDigests(ids)
	for _, id := range ids {
		definition := document.obligations[id]
		result := document.results[id]
		if match(definition, result) {
			return id, definition, result
		}
	}
	t.Fatal("matching obligation result not found")
	return protocol.Digest{}, axiomir.ObligationDefinition{}, obligationResult{}
}

func findExecution(
	t *testing.T,
	document *Document,
	match func(executionDefinition) bool,
) (protocol.Digest, executionDefinition) {
	t.Helper()
	ids := make([]protocol.Digest, 0, len(document.executions))
	for id := range document.executions {
		ids = append(ids, id)
	}
	sortDigests(ids)
	for _, id := range ids {
		execution := document.executions[id]
		if match(execution) {
			return id, execution
		}
	}
	t.Fatal("matching execution not found")
	return protocol.Digest{}, executionDefinition{}
}

func anotherTrust(t *testing.T, document *Document, excluded protocol.Digest) protocol.Digest {
	t.Helper()
	ids := make([]protocol.Digest, 0, len(document.trust))
	for id := range document.trust {
		if id != excluded {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(left, right int) bool { return ids[left].String() < ids[right].String() })
	if len(ids) == 0 {
		t.Fatal("another trust entry not found")
	}
	return ids[0]
}
