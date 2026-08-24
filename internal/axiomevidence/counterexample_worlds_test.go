package axiomevidence

import (
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

func TestVerifyCounterexampleWorldsRejectsDeclarationAndWFDrift(t *testing.T) {
	tests := []struct {
		name     string
		scenario string
		mutate   func(*testing.T, *Document)
	}{
		{
			name:     "wrong world cardinality",
			scenario: "ax-b01-wrong-add",
			mutate: func(t *testing.T, document *Document) {
				mutateFailedCounterexample(t, document, func(counterexample *counterexampleDefinition) {
					counterexample.worlds = append(counterexample.worlds, counterexample.worlds[0])
				})
			},
		},
		{
			name:     "unknown input interface",
			scenario: "ax-b01-wrong-add",
			mutate: func(t *testing.T, document *Document) {
				mutateFailedCounterexample(t, document, func(counterexample *counterexampleDefinition) {
					counterexample.worlds[0].Tables[0].Name = "unknown"
				})
			},
		},
		{
			name:     "integer outside IR range",
			scenario: "ax-b01-wrong-add",
			mutate: func(t *testing.T, document *Document) {
				mutateFailedCounterexample(t, document, func(counterexample *counterexampleDefinition) {
					setConcreteField(t, &counterexample.worlds[0].Tables[0].Rows[0], "subtotal_cents", axiomir.ConcreteValue{
						Kind: "int", Integer: "999999999999999999999999999999",
					})
				})
			},
		},
		{
			name:     "unknown enum member",
			scenario: "ax-b01-wrong-add",
			mutate: func(t *testing.T, document *Document) {
				mutateFailedCounterexample(t, document, func(counterexample *counterexampleDefinition) {
					row := &counterexample.worlds[0].Tables[0].Rows[0]
					value := concreteField(t, row, "state")
					value.EnumMember = "unknown"
					setConcreteField(t, row, "state", value)
				})
			},
		},
		{
			name:     "record field omitted",
			scenario: "ax-b01-wrong-add",
			mutate: func(t *testing.T, document *Document) {
				mutateFailedCounterexample(t, document, func(counterexample *counterexampleDefinition) {
					row := &counterexample.worlds[0].Tables[0].Rows[0]
					row.Fields = row.Fields[1:]
				})
			},
		},
		{
			name:     "record type differs from input table",
			scenario: "ax-b01-wrong-add",
			mutate: func(t *testing.T, document *Document) {
				mutateFailedCounterexample(t, document, func(counterexample *counterexampleDefinition) {
					counterexample.worlds[0].Tables[0].Rows[0].RecordType = protocol.Digest{}
				})
			},
		},
		{
			name:     "duplicate primary key",
			scenario: "ax-b02-wrong-region-join",
			mutate: func(t *testing.T, document *Document) {
				mutateFailedCounterexample(t, document, func(counterexample *counterexampleDefinition) {
					table := &counterexample.worlds[0].Tables[0]
					first := concreteField(t, &table.Rows[0], "customer_id")
					setConcreteField(t, &table.Rows[1], "customer_id", first)
				})
			},
		},
		{
			name:     "proof counterexample omits assume set",
			scenario: "ax-b01-wrong-add",
			mutate: func(t *testing.T, document *Document) {
				mutateFailedCounterexample(t, document, func(counterexample *counterexampleDefinition) {
					counterexample.preconditions = nil
				})
			},
		},
		{
			name:     "precondition is not an IR assume",
			scenario: "ax-b01-wrong-add",
			mutate: func(t *testing.T, document *Document) {
				mutateFailedCounterexample(t, document, func(counterexample *counterexampleDefinition) {
					counterexample.preconditions = []protocol.Digest{{}}
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document, ir := completenessFixture(t, test.scenario)
			test.mutate(t, &document)
			assertCompletenessCode(t, document.VerifyCounterexampleWorlds(ir), rejection.CounterexampleInvalid)
		})
	}
}

func TestVerifyCounterexampleWorldsAllowsExpectedInputWFFailure(t *testing.T) {
	for _, scenario := range []string{"ax-b01-invalid-input", "ax-b02-invalid-input", "ax-b03-invalid-input", "ax-b04-invalid-input"} {
		t.Run(scenario, func(t *testing.T) {
			document, ir := completenessFixture(t, scenario)
			if err := document.VerifyCounterexampleWorlds(ir); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestVerifyCounterexampleWorldsIsStableAcrossRepeatedValidation(t *testing.T) {
	document, ir := completenessFixture(t, "ax-b04-wrong-sensitive-filter")
	for iteration := 0; iteration < 100; iteration++ {
		if err := document.VerifyCounterexampleWorlds(ir); err != nil {
			t.Fatalf("iteration %d: %v", iteration, err)
		}
	}
}

func mutateFailedCounterexample(t *testing.T, document *Document, mutate func(*counterexampleDefinition)) {
	t.Helper()
	id, _, result := findObligationResult(t, document, func(definition axiomir.ObligationDefinition, result obligationResult) bool {
		return definition.Expectation == "prove" && result.kind == "failed" && result.counterexample != nil
	})
	mutate(result.counterexample)
	document.results[id] = result
}

func concreteField(t *testing.T, row *axiomir.ConcreteRecord, name string) axiomir.ConcreteValue {
	t.Helper()
	for _, field := range row.Fields {
		if field.Name == name {
			return field.Value
		}
	}
	t.Fatalf("concrete field %q not found", name)
	return axiomir.ConcreteValue{}
}

func setConcreteField(t *testing.T, row *axiomir.ConcreteRecord, name string, value axiomir.ConcreteValue) {
	t.Helper()
	for index := range row.Fields {
		if row.Fields[index].Name == name {
			row.Fields[index].Value = value
			return
		}
	}
	t.Fatalf("concrete field %q not found", name)
}
