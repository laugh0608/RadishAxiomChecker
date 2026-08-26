package checkresult

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/bundle"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/sourceidentity"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

var companionTestLimits = strictjson.Limits{
	MaxBytes: 2 << 20,
	MaxDepth: 256,
	MaxItems: 100_000,
	MaxSteps: 1_000_000,
}

func TestEncodeCompanionMatchesLockedStrictRejectionFixture(t *testing.T) {
	result, runtimeIdentity := strictRejectionResult(t)
	document, err := EncodeCompanion(result, runtimeIdentity)
	if err != nil {
		t.Fatal(err)
	}
	raw := document.Bytes()
	if got, want := len(raw), 1685; got != want {
		t.Fatalf("canonical fixture byte length = %d, want %d", got, want)
	}
	rawHash := protocol.Digest(sha256.Sum256(raw))
	if got, want := rawHash.String(), "sha256:f61b746a3e85f8edf6e8e5d5bbbba2550081565b6a2d22bc73003dbe136b3089"; got != want {
		t.Fatalf("canonical fixture content digest = %s, want %s", got, want)
	}
	if got, want := document.DomainDigest.String(), "sha256:24e81c66e17150c70c1b2d2eac50b47f16fc20c6a094111be3410546c7b6e608"; got != want {
		t.Fatalf("canonical fixture domain digest = %s, want %s", got, want)
	}
	parsed, err := ParseCompanion(raw, companionTestLimits)
	if err != nil {
		t.Fatal(err)
	}
	if string(parsed.Bytes()) != string(raw) || parsed.DomainDigest != document.DomainDigest {
		t.Fatal("canonical companion did not round-trip exactly")
	}
}

func TestParseCompanionAcceptsLockedScenarioCompanions(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "bundle", "testdata", "upstream", "s", "*", "expected-result.jcs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 27 {
		t.Fatalf("found %d locked companions, want 27", len(paths))
	}
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			document, err := ParseCompanion(raw, companionTestLimits)
			if err != nil {
				t.Fatal(err)
			}
			if string(document.Bytes()) != string(raw) {
				t.Fatal("locked companion bytes changed during parse")
			}
		})
	}
}

func TestEncodeCompanionRoundTripsActualResultLayerScenarios(t *testing.T) {
	root := filepath.Join("..", "bundle", "testdata", "upstream", "s")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	boundary := canonicalTestBoundary(t)
	artifact := mustDigest(t, "sha256:c9dde266f5791d6a1801440780cfac7208badd7ef30535603d1522f12e6b53bd")
	runtimeIdentity := runtimeIdentityForBoundary(boundary, artifact)
	skipped := map[string]bool{
		"chk-digest-01": true, "chk-process-01": true, "chk-resource-01": true,
	}
	encoded := 0
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
			document, err := EncodeCompanion(evaluation.Result, runtimeIdentity)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := ParseCompanion(document.Bytes(), companionTestLimits)
			if err != nil {
				t.Fatal(err)
			}
			if parsed.Outcome.Kind != evaluation.Result.Outcome.Kind ||
				!equalDigestSlices(parsed.MissingArtifacts, evaluation.Result.MissingArtifacts) ||
				!equalDigestSlices(parsed.RemainingTrust, trustDigests(evaluation.Result.RemainingTrust)) {
				t.Fatal("canonical companion changed actual result findings")
			}
			encoded++
		})
	}
	if encoded != 25 {
		t.Fatalf("encoded %d result-layer scenarios, want 25", encoded)
	}
}

func TestEncodeCompanionRequiresDistinctRuntimeArtifacts(t *testing.T) {
	result, runtimeIdentity := strictRejectionResult(t)
	tests := []struct {
		name   string
		mutate func(*Result, *RuntimeIdentity)
	}{
		{name: "missing checker artifact", mutate: func(_ *Result, runtime *RuntimeIdentity) {
			runtime.CheckerArtifact = protocol.Digest{}
		}},
		{name: "source substituted for binary", mutate: func(result *Result, runtime *RuntimeIdentity) {
			runtime.CheckerArtifact = result.Boundary.Checker.Source
		}},
		{name: "wrong toolchain", mutate: func(result *Result, _ *RuntimeIdentity) {
			result.Boundary.Checker.Toolchain = "go1.26.3"
		}},
		{name: "tcb source substituted for artifact", mutate: func(result *Result, runtime *RuntimeIdentity) {
			runtime.TCB[0].Artifact = result.Boundary.TCB[0].Source
		}},
		{name: "tcb version mismatch", mutate: func(_ *Result, runtime *RuntimeIdentity) {
			runtime.TCB[0].Version = "different"
		}},
		{name: "missing tcb component", mutate: func(_ *Result, runtime *RuntimeIdentity) {
			runtime.TCB = runtime.TCB[1:]
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inputResult := result
			inputResult.Boundary.TCB = append([]TCBComponent(nil), result.Boundary.TCB...)
			inputRuntime := runtimeIdentity
			inputRuntime.TCB = append([]RuntimeTCBComponent(nil), runtimeIdentity.TCB...)
			test.mutate(&inputResult, &inputRuntime)
			if _, err := EncodeCompanion(inputResult, inputRuntime); err == nil {
				t.Fatal("invalid runtime identity formed a canonical companion")
			}
		})
	}
}

func TestParseCompanionRejectsIdentityOrderAndAggregationDrift(t *testing.T) {
	result, runtimeIdentity := strictRejectionResult(t)
	valid, err := EncodeCompanion(result, runtimeIdentity)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		code   rejection.Code
		mutate func(CompanionDocument) []byte
	}{
		{
			name: "check ID mismatch", code: rejection.CheckIDMismatch,
			mutate: func(document CompanionDocument) []byte {
				raw := string(document.Bytes())
				from := `"id":"sha256:8ca5e5211be062a6d22592c39a0fe871fd3d32c2d8778ef938f337a50474fc6d"`
				to := `"id":"sha256:9ca5e5211be062a6d22592c39a0fe871fd3d32c2d8778ef938f337a50474fc6d"`
				return []byte(replaceOnce(t, raw, from, to))
			},
		},
		{
			name: "outcome drift", code: rejection.ResultAggregation,
			mutate: func(document CompanionDocument) []byte {
				raw := string(document.Bytes())
				return []byte(replaceOnce(t, raw, `"result":{"kind":"rejected"`, `"result":{"kind":"incomplete"`))
			},
		},
		{
			name: "unavailable accepted document", code: rejection.ResultAggregation,
			mutate: func(document CompanionDocument) []byte {
				document.Checks[0], _ = NewCheck(
					document.Checks[0].Definition.Kind, CheckPassed,
					document.Checks[0].Definition.Codes, document.Checks[0].Definition.Refs,
				)
				document.Outcome = Outcome{Kind: ResultAccepted}
				raw, err := encodeCompanionDocument(document)
				if err != nil {
					t.Fatal(err)
				}
				return raw
			},
		},
		{
			name: "checker source substitution", code: rejection.CheckerIdentity,
			mutate: func(document CompanionDocument) []byte {
				document.Checker.Artifact = document.Checker.Source
				raw, err := encodeCompanionDocument(document)
				if err != nil {
					t.Fatal(err)
				}
				return raw
			},
		},
		{
			name: "unsorted tcb", code: rejection.NoncanonicalOrder,
			mutate: func(document CompanionDocument) []byte {
				for left, right := 0, len(document.TCB)-1; left < right; left, right = left+1, right-1 {
					document.TCB[left], document.TCB[right] = document.TCB[right], document.TCB[left]
				}
				raw, err := encodeCompanionDocument(document)
				if err != nil {
					t.Fatal(err)
				}
				return raw
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseCompanion(test.mutate(valid), companionTestLimits)
			assertRejectionCode(t, err, test.code)
		})
	}
}

func TestCompanionDomainIdentityAndEncodingAreDeterministic(t *testing.T) {
	result, runtimeIdentity := strictRejectionResult(t)
	want, err := EncodeCompanion(result, runtimeIdentity)
	if err != nil {
		t.Fatal(err)
	}
	for iteration := 0; iteration < 100; iteration++ {
		got, err := EncodeCompanion(result, runtimeIdentity)
		if err != nil {
			t.Fatal(err)
		}
		if string(got.Bytes()) != string(want.Bytes()) || got.DomainDigest != want.DomainDigest {
			t.Fatal("canonical companion changed across identical inputs")
		}
	}
	tampered := want.DomainDigest
	tampered[0] ^= 0xff
	if err := want.VerifyDomainDigest(tampered); err == nil {
		t.Fatal("tampered result domain digest was accepted")
	}
	if err := want.VerifyDomainDigest(want.DomainDigest); err != nil {
		t.Fatal(err)
	}
}

func TestParseCompanionRejectsArrayAndReferenceDrift(t *testing.T) {
	document, boundary, runtimeIdentity := scenarioCompanion(t, "ax-b01-correct")
	tests := []struct {
		name   string
		code   rejection.Code
		mutate func(*CompanionDocument)
	}{
		{name: "check order", code: rejection.NoncanonicalOrder, mutate: func(document *CompanionDocument) {
			reverseChecks(document.Checks)
		}},
		{name: "trust order", code: rejection.NoncanonicalOrder, mutate: func(document *CompanionDocument) {
			reverseDigests(document.RemainingTrust)
		}},
		{name: "definition ref order", code: rejection.NoncanonicalOrder, mutate: func(document *CompanionDocument) {
			for index := range document.Checks {
				if len(document.Checks[index].Definition.Refs) > 1 {
					reverseRefs(document.Checks[index].Definition.Refs)
					return
				}
			}
			t.Fatal("scenario has no multi-reference check")
		}},
		{name: "unknown result ref", code: rejection.ResultAggregation, mutate: func(document *CompanionDocument) {
			document.Outcome.Refs = []protocol.Digest{mustDigest(t, "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := cloneCompanion(document)
			test.mutate(&mutated)
			raw, err := encodeCompanionDocument(mutated)
			if err != nil {
				t.Fatal(err)
			}
			_, err = ParseCompanion(raw, companionTestLimits)
			assertRejectionCode(t, err, test.code)
		})
	}

	tampered := cloneCompanion(document)
	tampered.TCB[0].Artifact = mustDigest(t, "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
	raw, err := encodeCompanionDocument(tampered)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseCompanion(raw, companionTestLimits)
	if err != nil {
		t.Fatal(err)
	}
	if err := parsed.VerifyIdentity(boundary, runtimeIdentity); err == nil {
		t.Fatal("runtime TCB artifact drift matched the invocation identity")
	}
	if err := document.VerifyIdentity(boundary, runtimeIdentity); err != nil {
		t.Fatal(err)
	}
}

func strictRejectionResult(t *testing.T) (Result, RuntimeIdentity) {
	t.Helper()
	source := mustDigest(t, "sha256:6abf9d3ab1c3fbc461e19d8b9a13a04444a1ffeacc9717d0f41ae3c4adc3c7aa")
	artifact := mustDigest(t, "sha256:c9dde266f5791d6a1801440780cfac7208badd7ef30535603d1522f12e6b53bd")
	evidence := mustDigest(t, "sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a")
	request := mustDigest(t, "sha256:f11516fd4bfe9c835b8899813daa1cd6b2791e85214eb72e6c9868dcd042182b")
	requestDomain := mustDigest(t, "sha256:2763823df8835c17dd3c3bdc19bbc70c3045cddc0edca9689b0af8d06e85f1ca")
	boundary := NewSourceBoundary(source, checkerToolchain, "0.0-test")
	boundary.Evidence = DocumentIdentity{ContentDigest: evidence}
	boundary.Request = DocumentIdentity{ContentDigest: request, DomainDigest: requestDomain, DomainAvailable: true}
	check, err := NewCheck(
		CheckStrictParse, CheckRejected, []string{"evidence-missing-required-members"},
		[]Ref{{Kind: RefArtifact, ID: evidence}},
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Aggregate(Input{Boundary: boundary, Checks: []Check{check}})
	if err != nil {
		t.Fatal(err)
	}
	return result, runtimeIdentityForBoundary(boundary, artifact)
}

func canonicalTestBoundary(t *testing.T) IdentityBoundary {
	t.Helper()
	snapshot, err := sourceidentity.Generate(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return NewSourceBoundary(mustDigest(t, snapshot.Digest), checkerToolchain, "0.1-test")
}

func runtimeIdentityForBoundary(boundary IdentityBoundary, artifact protocol.Digest) RuntimeIdentity {
	components := make([]RuntimeTCBComponent, 0, len(boundary.TCB))
	for _, component := range boundary.TCB {
		components = append(components, RuntimeTCBComponent{
			Artifact: artifact,
			Category: component.Category,
			Version:  component.Version,
		})
	}
	return RuntimeIdentity{CheckerArtifact: artifact, TCB: components}
}

func scenarioCompanion(t *testing.T, scenario string) (CompanionDocument, IdentityBoundary, RuntimeIdentity) {
	t.Helper()
	boundary := canonicalTestBoundary(t)
	artifact := mustDigest(t, "sha256:c9dde266f5791d6a1801440780cfac7208badd7ef30535603d1522f12e6b53bd")
	runtimeIdentity := runtimeIdentityForBoundary(boundary, artifact)
	root := filepath.Join("..", "bundle", "testdata", "upstream", "s", scenario, "bundle")
	verified, err := bundle.Verify(root)
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := EvaluateVerifiedBundle(verified, boundary)
	if err != nil {
		t.Fatal(err)
	}
	document, err := EncodeCompanion(evaluation.Result, runtimeIdentity)
	if err != nil {
		t.Fatal(err)
	}
	return document, evaluation.Result.Boundary, runtimeIdentity
}

func cloneCompanion(input CompanionDocument) CompanionDocument {
	result := input
	result.Checks = append([]Check(nil), input.Checks...)
	for index := range result.Checks {
		result.Checks[index].Definition.Codes = append([]string(nil), input.Checks[index].Definition.Codes...)
		result.Checks[index].Definition.Refs = append([]Ref(nil), input.Checks[index].Definition.Refs...)
	}
	result.MissingArtifacts = append([]protocol.Digest(nil), input.MissingArtifacts...)
	result.RemainingTrust = append([]protocol.Digest(nil), input.RemainingTrust...)
	result.Outcome.Refs = append([]protocol.Digest(nil), input.Outcome.Refs...)
	result.TCB = append([]RuntimeTCBComponent(nil), input.TCB...)
	result.raw = append([]byte(nil), input.raw...)
	return result
}

func reverseChecks(values []Check) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func reverseDigests(values []protocol.Digest) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func reverseRefs(values []Ref) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func trustDigests(values []Trust) []protocol.Digest {
	result := make([]protocol.Digest, 0, len(values))
	for _, value := range values {
		result = append(result, value.ID)
	}
	return result
}

func replaceOnce(t *testing.T, input, from, to string) string {
	t.Helper()
	index := -1
	for offset := 0; offset+len(from) <= len(input); offset++ {
		if input[offset:offset+len(from)] == from {
			if index != -1 {
				t.Fatalf("replacement target %q occurs more than once", from)
			}
			index = offset
		}
	}
	if index == -1 {
		t.Fatalf("replacement target %q was not found", from)
	}
	return input[:index] + to + input[index+len(from):]
}

func assertRejectionCode(t *testing.T, err error, want rejection.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected rejection %s", want)
	}
	got, ok := rejection.CodeOf(err)
	if !ok || got != want {
		t.Fatalf("rejection code = %q (%v), want %q", got, err, want)
	}
}
