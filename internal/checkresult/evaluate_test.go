package checkresult

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/axiomevidence"
	"radishaxiom.dev/independent-checker-go/internal/bundle"
	"radishaxiom.dev/independent-checker-go/internal/sourceidentity"
)

func TestEvaluateLockedResultLayerScenarios(t *testing.T) {
	root := filepath.Join("..", "bundle", "testdata", "upstream", "s")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	boundary := currentTestBoundary(t)
	skipped := map[string]bool{
		"chk-digest-01": true, "chk-process-01": true, "chk-resource-01": true,
	}
	distribution := make(map[ResultKind]int)
	conclusionResults := make(map[string]map[ResultKind]int)
	evaluated := 0
	for _, entry := range entries {
		name := entry.Name()
		if skipped[name] {
			continue
		}
		t.Run(name, func(t *testing.T) {
			verified, err := bundle.Verify(filepath.Join(root, name, "bundle"))
			if err != nil {
				t.Fatal(err)
			}
			evaluation, err := EvaluateVerifiedBundle(verified, boundary)
			if err != nil {
				t.Fatal(err)
			}
			expected := readExpectedResult(t, filepath.Join(root, name, "expected-result.jcs"))
			if evaluation.Result.Outcome.Kind != ResultKind(expected.Result.Kind) {
				t.Fatalf("actual result %s differs from locked result %s", evaluation.Result.Outcome.Kind, expected.Result.Kind)
			}
			if got := trustIDs(evaluation.Result.RemainingTrust); !equalStrings(got, expected.RemainingTrust) {
				t.Fatalf("remaining trust was not independently preserved:\n got %v\nwant %v", got, expected.RemainingTrust)
			}
			if got := digestStrings(evaluation.Result.MissingArtifacts); !equalStrings(got, expected.MissingArtifacts) {
				t.Fatalf("missing artifact inventory differs:\n got %v\nwant %v", got, expected.MissingArtifacts)
			}
			if len(evaluation.Result.Checks) != 10 {
				t.Fatalf("expected ten materialized checks, got %d", len(evaluation.Result.Checks))
			}
			if evaluation.Result.Boundary.Request.DomainDigest != verified.Request.DomainDigest ||
				evaluation.Result.Boundary.Request.ContentDigest != verified.RequestDigest {
				t.Fatal("request raw/domain identity was not retained")
			}
			distribution[evaluation.Result.Outcome.Kind]++
			if conclusionResults[evaluation.Conclusion.Kind] == nil {
				conclusionResults[evaluation.Conclusion.Kind] = make(map[ResultKind]int)
			}
			conclusionResults[evaluation.Conclusion.Kind][evaluation.Result.Outcome.Kind]++
			evaluated++
		})
	}
	if evaluated != 25 || distribution[ResultAcceptedWithTrust] != 22 ||
		distribution[ResultIncomplete] != 2 || distribution[ResultRejected] != 1 {
		t.Fatalf("unexpected result-layer distribution after %d scenarios: %+v", evaluated, distribution)
	}
	for _, conclusion := range []string{
		axiomevidence.ConclusionViolated,
		axiomevidence.ConclusionInputRejected,
		axiomevidence.ConclusionImplementationInconsistent,
	} {
		if conclusionResults[conclusion][ResultAcceptedWithTrust] == 0 {
			t.Fatalf("faithful producer conclusion %s was not independently accepted: %+v", conclusion, conclusionResults[conclusion])
		}
	}
}

func TestEvaluateProofPolicyDoesNotUpgradeAttestation(t *testing.T) {
	root := filepath.Join("..", "bundle", "testdata", "upstream", "s")
	boundary := currentTestBoundary(t)
	tests := []struct {
		scenario string
		want     ResultKind
		missing  int
	}{
		{"chk-proof-01", ResultIncomplete, 12},
		{"chk-proof-02", ResultAcceptedWithTrust, 0},
	}
	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			verified, err := bundle.Verify(filepath.Join(root, test.scenario, "bundle"))
			if err != nil {
				t.Fatal(err)
			}
			evaluation, err := EvaluateVerifiedBundle(verified, boundary)
			if err != nil {
				t.Fatal(err)
			}
			if evaluation.Result.Outcome.Kind != test.want || evaluation.ProofSupport.MissingProofMaterial != test.missing {
				t.Fatalf("proof policy boundary drifted: result=%s proof=%+v", evaluation.Result.Outcome.Kind, evaluation.ProofSupport)
			}
			if evaluation.ProofSupport.IndependentlyVerified != 0 {
				t.Fatal("backend attestation was upgraded to independent proof")
			}
		})
	}
}

func TestEvaluateIsDeterministicWithoutExpectedResultInput(t *testing.T) {
	root := filepath.Join("..", "bundle", "testdata", "upstream", "s", "ax-b01-correct", "bundle")
	verified, err := bundle.Verify(root)
	if err != nil {
		t.Fatal(err)
	}
	boundary := currentTestBoundary(t)
	want, err := EvaluateVerifiedBundle(verified, boundary)
	if err != nil {
		t.Fatal(err)
	}
	for iteration := 0; iteration < 100; iteration++ {
		got, err := EvaluateVerifiedBundle(verified, boundary)
		if err != nil {
			t.Fatal(err)
		}
		if got.Result.Outcome.Kind != want.Result.Outcome.Kind ||
			!equalStrings(trustIDs(got.Result.RemainingTrust), trustIDs(want.Result.RemainingTrust)) ||
			!equalStrings(digestStrings(got.Result.Outcome.Refs), digestStrings(want.Result.Outcome.Refs)) {
			t.Fatal("in-memory result changed across identical bundle evaluations")
		}
	}
}

type expectedResult struct {
	MissingArtifacts []string `json:"missing_artifacts"`
	RemainingTrust   []string `json:"remaining_trust"`
	Result           struct {
		Kind string `json:"kind"`
	} `json:"result"`
}

func readExpectedResult(t *testing.T, path string) expectedResult {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var result expectedResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func currentTestBoundary(t *testing.T) IdentityBoundary {
	t.Helper()
	snapshot, err := sourceidentity.Generate(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	source := mustDigest(t, snapshot.Digest)
	return NewSourceBoundary(source, runtime.Version(), "development")
}

func trustIDs(input []Trust) []string {
	result := make([]string, 0, len(input))
	for _, item := range input {
		result = append(result, item.ID.String())
	}
	return result
}

func digestStrings[T interface{ String() string }](input []T) []string {
	result := make([]string, 0, len(input))
	for _, item := range input {
		result = append(result, item.String())
	}
	return result
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
