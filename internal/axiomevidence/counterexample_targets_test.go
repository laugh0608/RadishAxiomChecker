package axiomevidence

import (
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/bundle"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

func TestVerifyCounterexampleTargetsReplaysAllLockedProofFailures(t *testing.T) {
	scenarios := []string{
		"ax-b01-wrong-add",
		"ax-b01-wrong-drop-zero",
		"ax-b02-wrong-constant-tier",
		"ax-b02-wrong-region-join",
		"ax-b03-wrong-single-group",
		"ax-b03-wrong-unit-sum",
		"ax-b04-wrong-sensitive-filter",
		"ax-b04-wrong-sensitive-priority",
	}
	for _, scenario := range scenarios {
		t.Run(scenario, func(t *testing.T) {
			document, ir, limits := targetFixture(t, scenario)
			check, err := document.VerifyCounterexampleTargets(ir, limits)
			if err != nil {
				t.Fatal(err)
			}
			if check.ReplayedProofs != 1 || check.DeferredComparisons != 0 {
				t.Fatalf("unexpected target replay summary: %+v", check)
			}
		})
	}
}

func TestExecuteLockedCorrectBenchmarks(t *testing.T) {
	for _, scenario := range []string{"ax-b01-correct", "ax-b02-correct", "ax-b03-correct", "ax-b04-correct"} {
		t.Run(scenario, func(t *testing.T) {
			document, ir, verified := concreteInputFixture(t, scenario)
			artifacts := make([]protocol.Digest, 0)
			for id, definition := range document.obligations {
				if definition.Kind == "input-conformance" && definition.Subject.Kind == "artifact" && document.results[id].kind == "checked" {
					artifacts = append(artifacts, definition.Subject.Artifact)
				}
			}
			sortDigests(artifacts)
			if len(artifacts) == 0 {
				t.Fatal("correct benchmark has no checked concrete input artifact")
			}
			for _, artifact := range artifacts {
				data, err := verified.ReadArtifact(artifact)
				if err != nil {
					t.Fatal(err)
				}
				input, err := ir.DecodeBenchmarkInput(data, completenessLimits(verified.Request))
				if err != nil {
					t.Fatal(err)
				}
				execution, err := ir.Execute(input.World, executionLimits(verified.Request))
				if err != nil {
					t.Fatal(err)
				}
				if len(execution.Outputs.Tables) != 1 {
					t.Fatalf("unexpected interpreted output shape: %+v", execution.Outputs)
				}
				if execution.Steps == 0 || execution.LogicalBytes == 0 {
					t.Fatalf("execution did not retain deterministic resource counts: %+v", execution)
				}
			}
		})
	}
}

func TestVerifyCounterexampleTargetsRejectsTraceObservationAndWitnessDrift(t *testing.T) {
	tests := []struct {
		name     string
		scenario string
		mutate   func(*testing.T, *Document)
	}{
		{
			name:     "trace obligation drift",
			scenario: "ax-b01-wrong-add",
			mutate: func(t *testing.T, document *Document) {
				mutateFailedCounterexample(t, document, func(counterexample *counterexampleDefinition) {
					counterexample.trace[1].ref = protocol.Digest{}
				})
			},
		},
		{
			name:     "observed obligation drift",
			scenario: "ax-b01-wrong-add",
			mutate: func(t *testing.T, document *Document) {
				mutateFailedCounterexample(t, document, func(counterexample *counterexampleDefinition) {
					counterexample.observed.obligation = protocol.Digest{}
				})
			},
		},
		{
			name:     "required key drift",
			scenario: "ax-b03-wrong-unit-sum",
			mutate: func(t *testing.T, document *Document) {
				mutateFailedCounterexample(t, document, func(counterexample *counterexampleDefinition) {
					counterexample.observed.requiredKeys = []string{"usage_events:missing"}
				})
			},
		},
		{
			name:     "paired public input drift",
			scenario: "ax-b04-wrong-sensitive-priority",
			mutate: func(t *testing.T, document *Document) {
				mutateFailedCounterexample(t, document, func(counterexample *counterexampleDefinition) {
					setConcreteField(t, &counterexample.worlds[1].Tables[0].Rows[0], "priority", axiomir.ConcreteValue{
						Kind:       "enum",
						EnumType:   counterexample.worlds[0].Tables[0].Rows[0].Fields[3].Value.EnumType,
						EnumMember: "low",
					})
				})
			},
		},
		{
			name:     "target no longer violated",
			scenario: "ax-b04-wrong-sensitive-filter",
			mutate: func(t *testing.T, document *Document) {
				mutateFailedCounterexample(t, document, func(counterexample *counterexampleDefinition) {
					setConcreteField(t, &counterexample.worlds[1].Tables[0].Rows[0], "contact_email", axiomir.ConcreteValue{
						Kind: "text", Text: "a@example.test",
					})
				})
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document, ir, limits := targetFixture(t, test.scenario)
			test.mutate(t, &document)
			_, err := document.VerifyCounterexampleTargets(ir, limits)
			assertCompletenessCode(t, err, rejection.CounterexampleInvalid)
		})
	}
}

func TestVerifyCounterexampleTargetsEnforcesResourceLimits(t *testing.T) {
	document, ir, limits := targetFixture(t, "ax-b03-wrong-unit-sum")
	limits.MaxSemanticSteps = 1
	_, err := document.VerifyCounterexampleTargets(ir, limits)
	assertCompletenessCode(t, err, rejection.ResourceLimit)

	document, ir, limits = targetFixture(t, "ax-b01-wrong-add")
	limits.MaxLogicalBytes = 1
	_, err = document.VerifyCounterexampleTargets(ir, limits)
	assertCompletenessCode(t, err, rejection.ResourceLimit)
}

func targetFixture(t *testing.T, scenario string) (Document, axiomir.Document, axiomir.ExecutionLimits) {
	t.Helper()
	document, ir := completenessFixture(t, scenario)
	verified, err := bundle.Verify(concreteScenarioRoot(scenario))
	if err != nil {
		t.Fatal(err)
	}
	return document, ir, executionLimits(verified.Request)
}

func executionLimits(request protocol.Request) axiomir.ExecutionLimits {
	workingMemory, _ := request.Limit("working-memory")
	semanticSteps, _ := request.Limit("semantic-steps")
	return axiomir.ExecutionLimits{MaxLogicalBytes: workingMemory, MaxSemanticSteps: semanticSteps}
}
