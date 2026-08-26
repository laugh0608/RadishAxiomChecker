package axiomevidence

import (
	"reflect"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

func TestVerifyConclusionMatchesLockedProducerKindsAndRefs(t *testing.T) {
	tests := []struct {
		scenario string
		kind     string
		refs     int
	}{
		{"ax-b01-correct", ConclusionSatisfied, 0},
		{"ax-b01-invalid-input", ConclusionInputRejected, 2},
		{"ax-b01-wrong-add", ConclusionViolated, 1},
		{"ax-b01-backend-timeout", ConclusionInconclusive, 14},
		{"chk-concrete-01", ConclusionImplementationInconsistent, 3},
	}
	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			document, ir := completenessFixture(t, test.scenario)
			if err := document.VerifyObligationCompleteness(ir); err != nil {
				t.Fatal(err)
			}
			if err := document.VerifyStateSupport(); err != nil {
				t.Fatal(err)
			}
			check, err := document.VerifyConclusion(conclusionTestLimits())
			if err != nil {
				t.Fatal(err)
			}
			if check.Kind != test.kind || len(check.Refs) != test.refs {
				t.Fatalf("unexpected conclusion: %+v", check)
			}
			for _, ref := range check.Refs {
				if ref.Kind != ConclusionRefObligation {
					t.Fatalf("locked conclusion unexpectedly contains a non-obligation ref: %+v", ref)
				}
			}
		})
	}
}

func TestRecomputeConclusionKeepsStructureRejectionAsEarlierLayer(t *testing.T) {
	document, _ := completenessFixture(t, "ax-b01-wrong-add")
	id := anotherObligationID(t, &document, protocol.Digest{})
	check, err := recomputeConclusion(
		&conclusionReference{kind: ConclusionRefObligation, value: id},
		document.obligations,
		document.results,
		document.executions,
		conclusionTestLimits(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if check.Kind != ConclusionStructureRejected || len(check.Refs) != 1 || check.Refs[0].Value != id {
		t.Fatalf("structure rejection did not retain priority and ref: %+v", check)
	}
}

func TestVerifyConclusionRejectsProducerDrift(t *testing.T) {
	tests := []struct {
		name     string
		scenario string
		mutate   func(*testing.T, *Document)
	}{
		{
			name:     "kind",
			scenario: "ax-b01-correct",
			mutate: func(_ *testing.T, document *Document) {
				document.conclusion.kind = ConclusionViolated
			},
		},
		{
			name:     "missing ref",
			scenario: "ax-b01-invalid-input",
			mutate: func(_ *testing.T, document *Document) {
				document.conclusion.refs = document.conclusion.refs[:1]
			},
		},
		{
			name:     "extra ref",
			scenario: "ax-b01-wrong-add",
			mutate: func(t *testing.T, document *Document) {
				id := anotherObligationID(t, document, document.conclusion.refs[0].value)
				document.conclusion.refs = append(document.conclusion.refs, conclusionReference{
					kind: ConclusionRefObligation, value: id,
				})
			},
		},
		{
			name:     "wrong ref class",
			scenario: "ax-b01-wrong-add",
			mutate: func(_ *testing.T, document *Document) {
				document.conclusion.refs[0].kind = ConclusionRefExecution
			},
		},
		{
			name:     "wrong ref value",
			scenario: "ax-b01-wrong-add",
			mutate: func(t *testing.T, document *Document) {
				document.conclusion.refs[0].value = anotherObligationID(t, document, document.conclusion.refs[0].value)
			},
		},
		{
			name:     "ref order",
			scenario: "ax-b01-invalid-input",
			mutate: func(_ *testing.T, document *Document) {
				document.conclusion.refs[0], document.conclusion.refs[1] =
					document.conclusion.refs[1], document.conclusion.refs[0]
			},
		},
		{
			name:     "priority",
			scenario: "ax-b01-wrong-add",
			mutate: func(_ *testing.T, document *Document) {
				document.conclusion.kind = ConclusionInconclusive
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document, _ := completenessFixture(t, test.scenario)
			test.mutate(t, &document)
			_, err := document.VerifyConclusion(conclusionTestLimits())
			assertCompletenessCode(t, err, rejection.ConclusionMismatch)
		})
	}
}

func TestVerifyConclusionPrioritizesReplayedFailureOverUnknown(t *testing.T) {
	document, _ := completenessFixture(t, "ax-b01-wrong-add")
	id, _, result := findObligationResult(t, &document, func(definition axiomir.ObligationDefinition, result obligationResult) bool {
		return definition.Expectation == "prove" && result.kind == "proved"
	})
	result = obligationResult{kind: "unknown", reason: "timeout"}
	document.results[id] = result

	check, err := document.VerifyConclusion(conclusionTestLimits())
	if err != nil {
		t.Fatal(err)
	}
	if check.Kind != ConclusionViolated || len(check.Refs) != 1 {
		t.Fatalf("failed conclusion did not retain priority over unknown: %+v", check)
	}
}

func TestVerifyConclusionDoesNotHideOrConsumeProofCoverage(t *testing.T) {
	document, _ := completenessFixture(t, "chk-proof-01")
	check, err := document.VerifyConclusion(conclusionTestLimits())
	if err != nil {
		t.Fatal(err)
	}
	if check.Kind != ConclusionSatisfied || len(check.Refs) != 0 {
		t.Fatalf("unexpected producer conclusion: %+v", check)
	}
	proved := 0
	for _, result := range document.results {
		if result.kind == "proved" {
			proved++
		}
	}
	if proved != 12 {
		t.Fatalf("conclusion verification altered proof claims: got %d", proved)
	}
}

func TestVerifyConclusionFailsClosedOnRequiredExecutionAndBudgets(t *testing.T) {
	document, _ := completenessFixture(t, "ax-b01-correct")
	_, _, result := findObligationResult(t, &document, func(_ axiomir.ObligationDefinition, result obligationResult) bool {
		return result.kind == "proved"
	})
	execution := document.executions[result.support.execution]
	execution.result = executionResult{kind: "timeout", code: "synthetic-timeout"}
	document.executions[result.support.execution] = execution
	_, err := document.VerifyConclusion(conclusionTestLimits())
	assertCompletenessCode(t, err, rejection.ConclusionMismatch)

	document, _ = completenessFixture(t, "ax-b01-correct")
	_, err = document.VerifyConclusion(ConclusionLimits{MaxLogicalBytes: 1, MaxSemanticSteps: 1_000_000})
	assertCompletenessCode(t, err, rejection.ResourceLimit)
	_, err = document.VerifyConclusion(ConclusionLimits{MaxLogicalBytes: 1 << 20, MaxSemanticSteps: 1})
	assertCompletenessCode(t, err, rejection.ResourceLimit)
}

func TestVerifyConclusionIsDeterministic(t *testing.T) {
	document, _ := completenessFixture(t, "ax-b01-backend-timeout")
	want, err := document.VerifyConclusion(conclusionTestLimits())
	if err != nil {
		t.Fatal(err)
	}
	for iteration := 0; iteration < 100; iteration++ {
		got, err := document.VerifyConclusion(conclusionTestLimits())
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatal("conclusion recomputation changed across identical runs")
		}
	}
}

func conclusionTestLimits() ConclusionLimits {
	return ConclusionLimits{MaxLogicalBytes: 1 << 20, MaxSemanticSteps: 1_000_000}
}

func anotherObligationID(t *testing.T, document *Document, excluded protocol.Digest) protocol.Digest {
	t.Helper()
	ids := make([]protocol.Digest, 0, len(document.obligations))
	for id := range document.obligations {
		if id != excluded {
			ids = append(ids, id)
		}
	}
	sortDigests(ids)
	if len(ids) == 0 {
		t.Fatal("another obligation ID not found")
	}
	return ids[0]
}
