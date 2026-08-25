package axiomevidence

import (
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/bundle"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

func TestVerifyConcreteInputsRejectsBindingClassificationAndWorldDrift(t *testing.T) {
	tests := []struct {
		name     string
		scenario string
		mutate   func(*testing.T, *Document)
	}{
		{
			name:     "artifact format drift",
			scenario: "ax-b01-correct",
			mutate: func(t *testing.T, document *Document) {
				artifact, _, _ := inputArtifactResult(t, document)
				definition := document.artifacts[artifact]
				definition.format = "axiom-host-data"
				document.artifacts[artifact] = definition
			},
		},
		{
			name:     "execution host input binding omitted",
			scenario: "ax-b01-correct",
			mutate: func(t *testing.T, document *Document) {
				for id, execution := range document.executions {
					for index := range execution.inputs {
						if execution.inputs[index].role == "host-input" {
							execution.inputs[index].role = "host-inpot"
						}
					}
					document.executions[id] = execution
				}
			},
		},
		{
			name:     "checked result changed to failed",
			scenario: "ax-b01-correct",
			mutate: func(t *testing.T, document *Document) {
				_, id, result := inputArtifactResult(t, document)
				result.kind = "failed"
				document.results[id] = result
			},
		},
		{
			name:     "failed result changed to checked",
			scenario: "ax-b01-invalid-input",
			mutate: func(t *testing.T, document *Document) {
				_, id, result := inputArtifactResult(t, document)
				result.kind = "checked"
				document.results[id] = result
			},
		},
		{
			name:     "counterexample world is not an artifact projection",
			scenario: "ax-b01-invalid-input",
			mutate: func(t *testing.T, document *Document) {
				_, id, result := inputArtifactResult(t, document)
				if result.counterexample == nil {
					t.Fatal("input failure has no counterexample")
				}
				setConcreteField(t, &result.counterexample.worlds[0].Tables[0].Rows[0], "order_id", axiomir.ConcreteValue{
					Kind: "text", Text: "not-in-artifact",
				})
				document.results[id] = result
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document, ir, verified := concreteInputFixture(t, test.scenario)
			test.mutate(t, &document)
			_, err := document.VerifyConcreteInputs(ir, verified.ReadArtifact, concreteInputLimits(verified.Request))
			assertCompletenessCode(t, err, rejection.ConcreteCheckMismatch)
		})
	}
}

func TestVerifyConcreteInputsRejectsDigestAndResourceDrift(t *testing.T) {
	document, ir, verified := concreteInputFixture(t, "ax-b01-correct")
	_, err := document.VerifyConcreteInputs(ir, func(id protocol.Digest) ([]byte, error) {
		data, err := verified.ReadArtifact(id)
		return append(data, ' '), err
	}, concreteInputLimits(verified.Request))
	assertCompletenessCode(t, err, rejection.DigestMismatch)

	limits := concreteInputLimits(verified.Request)
	limits.MaxLogicalBytes = 1
	_, err = document.VerifyConcreteInputs(ir, verified.ReadArtifact, limits)
	assertCompletenessCode(t, err, rejection.ResourceLimit)

	limits = concreteInputLimits(verified.Request)
	limits.MaxSemanticSteps = 1
	_, err = document.VerifyConcreteInputs(ir, verified.ReadArtifact, limits)
	assertCompletenessCode(t, err, rejection.ResourceLimit)
}

func TestVerifyConcreteInputsIsStableAcrossRepeatedValidation(t *testing.T) {
	document, ir, verified := concreteInputFixture(t, "ax-b02-invalid-input")
	for iteration := 0; iteration < 100; iteration++ {
		check, err := document.VerifyConcreteInputs(ir, verified.ReadArtifact, concreteInputLimits(verified.Request))
		if err != nil {
			t.Fatalf("iteration %d: %v", iteration, err)
		}
		if check.Checked != 0 || check.Failed != 1 || len(check.Artifacts) != 1 {
			t.Fatalf("iteration %d: unstable concrete input summary: %+v", iteration, check)
		}
	}
}

func concreteInputFixture(t *testing.T, scenario string) (Document, axiomir.Document, bundle.Verified) {
	t.Helper()
	document, ir := completenessFixture(t, scenario)
	root := concreteScenarioRoot(scenario)
	verified, err := bundle.Verify(root)
	if err != nil {
		t.Fatal(err)
	}
	return document, ir, verified
}

func concreteScenarioRoot(scenario string) string {
	return "../bundle/testdata/upstream/s/" + scenario + "/bundle"
}

func concreteInputLimits(request protocol.Request) ConcreteInputLimits {
	workingMemory, _ := request.Limit("working-memory")
	semanticSteps, _ := request.Limit("semantic-steps")
	return ConcreteInputLimits{
		JSON:             completenessLimits(request),
		MaxLogicalBytes:  workingMemory,
		MaxSemanticSteps: semanticSteps,
	}
}

func inputArtifactResult(t *testing.T, document *Document) (protocol.Digest, protocol.Digest, obligationResult) {
	t.Helper()
	for id, definition := range document.obligations {
		if definition.Kind == "input-conformance" && definition.Subject.Kind == "artifact" {
			return definition.Subject.Artifact, id, document.results[id]
		}
	}
	t.Fatal("input-conformance artifact obligation not found")
	return protocol.Digest{}, protocol.Digest{}, obligationResult{}
}
