package axiomir_test

import (
	"bytes"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

func TestDecodeBenchmarkInputAndEvaluateLockedB01Pre(t *testing.T) {
	document, err := axiomir.ParseStructure(lockedB01(t), fixtureLimits())
	if err != nil {
		t.Fatal(err)
	}
	input, err := document.DecodeBenchmarkInput(validB01BenchmarkInput(), fixtureLimits())
	if err != nil {
		t.Fatal(err)
	}
	check := document.CheckCompleteInputWorld(input.World)
	if !check.Anchored || !check.WellFormed || len(check.Violations) != 0 {
		t.Fatalf("valid complete benchmark input rejected: %+v", check)
	}
	evaluation, err := document.EvaluateAssumes(input.World, 1_000)
	if err != nil {
		t.Fatal(err)
	}
	if !evaluation.AllTrue || len(evaluation.Failed) != 0 {
		t.Fatalf("valid input unexpectedly failed Pre: %+v", evaluation)
	}

	invalidBytes := bytes.Replace(validB01BenchmarkInput(), []byte(`"discount_cents":"0"`), []byte(`"discount_cents":"101"`), 1)
	invalid, err := document.DecodeBenchmarkInput(invalidBytes, fixtureLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !document.CheckCompleteInputWorld(invalid.World).WellFormed {
		t.Fatal("Pre-violating input should remain WF")
	}
	evaluation, err = document.EvaluateAssumes(invalid.World, 1_000)
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.AllTrue || len(evaluation.Failed) != 1 {
		t.Fatalf("invalid input did not fail its single assume: %+v", evaluation)
	}
}

func TestDecodeBenchmarkInputRejectsEnvelopeDrift(t *testing.T) {
	document, err := axiomir.ParseStructure(lockedB01(t), fixtureLimits())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		old  []byte
		new  []byte
		code rejection.Code
	}{
		{"unknown member", []byte(`"role" : "input"`), []byte(`"rolz" : "input"`), rejection.UnknownMember},
		{"unsupported version", []byte(`"data_version": "0.1"`), []byte(`"data_version": "0.2"`), rejection.UnsupportedVersion},
		{"wrong benchmark ID", []byte(`"benchmark_id": "AX-B01"`), []byte(`"benchmark_id": "AX-B02"`), rejection.ConcreteCheckMismatch},
		{"wrong role", []byte(`"role" : "input"`), []byte(`"role" : "output"`), rejection.UnknownTag},
		{"JSON number", []byte(`"subtotal_cents":"100"`), []byte(`"subtotal_cents":100`), rejection.JSONNumberOrNull},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := bytes.Replace(validB01BenchmarkInput(), test.old, test.new, 1)
			_, err := document.DecodeBenchmarkInput(mutated, fixtureLimits())
			assertCode(t, err, test.code)
		})
	}
}

func TestDecodeBenchmarkInputClassifiesWFDrift(t *testing.T) {
	document, err := axiomir.ParseStructure(lockedB01(t), fixtureLimits())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		old  []byte
		new  []byte
	}{
		{"missing interface", nil, nil},
		{"unknown enum member", []byte(`"state":"settled"`), []byte(`"state":"unknown"`)},
		{"scalar kind drift", []byte(`"subtotal_cents":"100"`), []byte(`"subtotal_cents":true`)},
		{"duplicate primary key", []byte(`"order_id":"O2"`), []byte(`"order_id":"O1"`)},
		{"reverse primary key order", []byte(`"order_id":"O1"`), []byte(`"order_id":"O9"`)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.name == "missing interface" {
				input, err := document.DecodeBenchmarkInput([]byte(`{"benchmark_id":"AX-B01","data_version":"0.1","format":"axiom-benchmark-data","role":"input","tables":[]}`), fixtureLimits())
				if err != nil {
					t.Fatal(err)
				}
				if document.CheckCompleteInputWorld(input.World).WellFormed {
					t.Fatal("missing interface was classified WF")
				}
				return
			}
			mutated := bytes.Replace(validB01BenchmarkInput(), test.old, test.new, 1)
			input, err := document.DecodeBenchmarkInput(mutated, fixtureLimits())
			if err != nil {
				t.Fatal(err)
			}
			if document.CheckCompleteInputWorld(input.World).WellFormed {
				t.Fatal("WF drift was classified well formed")
			}
		})
	}
}

func TestEvaluateAssumesEnforcesSemanticStepLimit(t *testing.T) {
	document, err := axiomir.ParseStructure(lockedB01(t), fixtureLimits())
	if err != nil {
		t.Fatal(err)
	}
	input, err := document.DecodeBenchmarkInput(validB01BenchmarkInput(), fixtureLimits())
	if err != nil {
		t.Fatal(err)
	}
	_, err = document.EvaluateAssumes(input.World, 1)
	assertCode(t, err, rejection.ResourceLimit)
}

func validB01BenchmarkInput() []byte {
	return []byte(`{
  "role" : "input",
  "format": "axiom-benchmark-data",
  "benchmark_id": "AX-B01",
  "tables": [
    {
      "rows": [
        {"state":"settled","subtotal_cents":"100","order_id":"O1","discount_cents":"0"},
        {"discount_cents":"25","order_id":"O2","state":"pending","subtotal_cents":"100"}
      ],
      "name": "orders"
    }
  ],
  "data_version": "0.1"
}
`)
}
