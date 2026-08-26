package checkresult

import (
	"reflect"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
)

func TestNewCheckMatchesLockedContractDomainID(t *testing.T) {
	evidence := mustDigest(t, "sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a")
	check, err := NewCheck(
		CheckStrictParse,
		CheckRejected,
		[]string{"evidence-missing-required-members"},
		[]Ref{{Kind: RefArtifact, ID: evidence}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := check.ID.String(), "sha256:8ca5e5211be062a6d22592c39a0fe871fd3d32c2d8778ef938f337a50474fc6d"; got != want {
		t.Fatalf("check domain ID mismatch: got %s, want %s", got, want)
	}
}

func TestAggregateAppliesUniqueFourStatePriority(t *testing.T) {
	trustID := mustDigest(t, "sha256:901ef8a29e1610b1f093c74ebc8d3716b77011d87b1b8941e4a16035ee88a840")
	missing := mustDigest(t, "sha256:ae68157c603623ea9f08b68a836c16ca9d082d224800b91160b8859929eba564")
	tests := []struct {
		name      string
		mutate    func(*Input)
		want      ResultKind
		wantTrust int
		wantMiss  int
	}{
		{name: "accepted", want: ResultAccepted},
		{
			name: "accepted with allowed trust",
			mutate: func(input *Input) {
				input.AllowedTrustCategories = []string{"proof-backend"}
				input.RemainingTrust = []Trust{{ID: trustID, Category: "proof-backend"}}
				input.Checks = replaceOutcome(t, input.Checks, CheckStateSupport, CheckTrusted, "invalid-state-support")
			},
			want: ResultAcceptedWithTrust, wantTrust: 1,
		},
		{
			name: "disallowed trust is incomplete",
			mutate: func(input *Input) {
				input.RemainingTrust = []Trust{{ID: trustID, Category: "proof-backend"}}
			},
			want: ResultIncomplete, wantTrust: 1,
		},
		{
			name: "missing artifact is incomplete",
			mutate: func(input *Input) {
				input.MissingArtifacts = []protocol.Digest{missing}
			},
			want: ResultIncomplete, wantMiss: 1,
		},
		{
			name: "rejection wins over incomplete trust and missing artifact",
			mutate: func(input *Input) {
				input.RemainingTrust = []Trust{{ID: trustID, Category: "proof-backend"}}
				input.MissingArtifacts = []protocol.Digest{missing}
				input.Checks = replaceOutcome(t, input.Checks, CheckStrictParse, CheckRejected, "noncanonical-json")
				input.Checks = replaceOutcome(t, input.Checks, CheckIdentity, CheckIncomplete, "artifact-missing")
			},
			want: ResultRejected, wantTrust: 1, wantMiss: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validAggregateInput(t)
			if test.mutate != nil {
				test.mutate(&input)
			}
			result, err := Aggregate(input)
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome.Kind != test.want || len(result.RemainingTrust) != test.wantTrust || len(result.MissingArtifacts) != test.wantMiss {
				t.Fatalf("unexpected aggregate: %+v", result)
			}
			if test.want != ResultAccepted && len(result.Outcome.Refs) == 0 {
				t.Fatal("non-accepted result omitted causal check refs")
			}
		})
	}
}

func TestAggregateRejectsIdentityAndCoverageDrift(t *testing.T) {
	input := validAggregateInput(t)
	input.Boundary.TCB = input.Boundary.TCB[1:]
	if _, err := Aggregate(input); err == nil {
		t.Fatal("missing TCB component was accepted")
	}

	input = validAggregateInput(t)
	input.Checks = input.Checks[:len(input.Checks)-1]
	if _, err := Aggregate(input); err == nil {
		t.Fatal("accepted result with a missing check kind was accepted")
	}

	input = validAggregateInput(t)
	input.Boundary.Evidence.DomainAvailable = false
	input.Boundary.Evidence.DomainDigest = protocol.Digest{}
	if _, err := Aggregate(input); err == nil {
		t.Fatal("non-rejected result with unavailable Evidence domain identity was accepted")
	}

	input = validAggregateInput(t)
	input.Checks = replaceOutcome(t, input.Checks, CheckStrictParse, CheckRejected, "noncanonical-json")
	input.Boundary.Evidence.DomainAvailable = false
	input.Boundary.Evidence.DomainDigest = protocol.Digest{}
	result, err := Aggregate(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome.Kind != ResultRejected {
		t.Fatalf("unavailable rejected Evidence identity changed result: %+v", result.Outcome)
	}
}

func TestAggregateIsDeterministic(t *testing.T) {
	input := validAggregateInput(t)
	want, err := Aggregate(input)
	if err != nil {
		t.Fatal(err)
	}
	for iteration := 0; iteration < 100; iteration++ {
		shuffled := input
		shuffled.Checks = append([]Check(nil), input.Checks...)
		for left, right := 0, len(shuffled.Checks)-1; left < right; left, right = left+1, right-1 {
			shuffled.Checks[left], shuffled.Checks[right] = shuffled.Checks[right], shuffled.Checks[left]
		}
		got, err := Aggregate(shuffled)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatal("aggregation changed across equivalent input order")
		}
	}
}

func validAggregateInput(t *testing.T) Input {
	t.Helper()
	source := mustDigest(t, "sha256:6f52f4756b163868f54e6858f777be6894a7179e374ce9cbaf73592b247dcbf8")
	evidence := mustDigest(t, "sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a")
	evidenceDomain := mustDigest(t, "sha256:24e81c66e17150c70c1b2d2eac50b47f16fc20c6a094111be3410546c7b6e608")
	request := mustDigest(t, "sha256:f11516fd4bfe9c835b8899813daa1cd6b2791e85214eb72e6c9868dcd042182b")
	requestDomain := mustDigest(t, "sha256:2763823df8835c17dd3c3bdc19bbc70c3045cddc0edca9689b0af8d06e85f1ca")
	boundary := NewSourceBoundary(source, "go1.26.3", "development")
	boundary.Evidence = DocumentIdentity{ContentDigest: evidence, DomainDigest: evidenceDomain, DomainAvailable: true}
	boundary.Request = DocumentIdentity{ContentDigest: request, DomainDigest: requestDomain, DomainAvailable: true}

	checks := make([]Check, 0, len(requiredCheckKinds))
	for _, kind := range requiredCheckKinds {
		check, err := NewCheck(kind, CheckPassed, []string{defaultCode(kind)}, []Ref{{Kind: RefArtifact, ID: evidence}})
		if err != nil {
			t.Fatal(err)
		}
		checks = append(checks, check)
	}
	return Input{Boundary: boundary, Checks: checks}
}

func replaceOutcome(t *testing.T, checks []Check, kind CheckKind, outcome CheckOutcome, code string) []Check {
	t.Helper()
	result := append([]Check(nil), checks...)
	for index, check := range result {
		if check.Definition.Kind != kind {
			continue
		}
		replacement, err := NewCheck(kind, outcome, []string{code}, check.Definition.Refs)
		if err != nil {
			t.Fatal(err)
		}
		result[index] = replacement
		return result
	}
	t.Fatalf("check kind %s not found", kind)
	return nil
}

func defaultCode(kind CheckKind) string {
	switch kind {
	case CheckConclusion:
		return "conclusion-mismatch"
	case CheckConcreteReplay:
		return "concrete-check-mismatch"
	case CheckCounterexampleReplay:
		return "counterexample-invalid"
	case CheckIdentity:
		return "manifest-coverage"
	case CheckIsolation:
		return "isolation-boundary-violation"
	case CheckObligation:
		return "obligation-mismatch"
	case CheckProofSupport:
		return "proof-support-mismatch"
	case CheckStateSupport:
		return "invalid-state-support"
	case CheckStrictParse:
		return "noncanonical-json"
	case CheckSubject:
		return "subject-mismatch"
	default:
		return ""
	}
}

func mustDigest(t *testing.T, value string) protocol.Digest {
	t.Helper()
	digest, err := protocol.ParseDigest(value)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
