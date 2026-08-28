package checkresult

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
)

func TestInvocationCompletesNormalBundleWithOneCumulativeLedger(t *testing.T) {
	root := filepath.Join("..", "bundle", "testdata", "upstream", "s", "ax-b01-correct", "bundle")
	boundary := canonicalTestBoundary(t)
	runtimeIdentity := runtimeIdentityForBoundary(
		boundary,
		mustDigest(t, "sha256:c9dde266f5791d6a1801440780cfac7208badd7ef30535603d1522f12e6b53bd"),
	)
	invocation, err := InvokeBundle(root, boundary, runtimeIdentity, nil)
	if err != nil {
		t.Fatal(err)
	}
	if invocation.Kind != InvocationResult || invocation.Evaluation.Result.Outcome.Kind != ResultAcceptedWithTrust {
		t.Fatalf("normal invocation = %s/%s, want result/accepted-with-trust", invocation.Kind, invocation.Evaluation.Result.Outcome.Kind)
	}
	if len(invocation.Evaluation.Result.Checks) != len(requiredCheckKinds) {
		t.Fatalf("normal invocation formed %d checks, want %d", len(invocation.Evaluation.Result.Checks), len(requiredCheckKinds))
	}
	if invocation.Resources.BundleBytes == 0 || invocation.Resources.CollectionItems == 0 ||
		invocation.Resources.DigestBlocks == 0 || invocation.Resources.SemanticSteps == 0 {
		t.Fatalf("normal invocation did not retain cumulative counters: %+v", invocation.Resources)
	}
	assertCompanionMatchesInvocation(t, invocation)
}

func TestInvocationWallClockExhaustionFormsIncompleteResult(t *testing.T) {
	root := filepath.Join("..", "bundle", "testdata", "upstream", "s", "ax-b01-correct", "bundle")
	boundary := canonicalTestBoundary(t)
	runtimeIdentity := runtimeIdentityForBoundary(
		boundary,
		mustDigest(t, "sha256:c9dde266f5791d6a1801440780cfac7208badd7ef30535603d1522f12e6b53bd"),
	)
	started := time.Unix(1_700_000_000, 0)
	calls := 0
	now := func() time.Time {
		calls++
		if calls >= 3 {
			return started.Add(5 * time.Second)
		}
		return started
	}
	invocation, err := InvokeBundle(root, boundary, runtimeIdentity, now)
	if err != nil {
		t.Fatal(err)
	}
	if invocation.Kind != InvocationResult || invocation.Evaluation.Result.Outcome.Kind != ResultIncomplete {
		t.Fatalf("wall-clock invocation = %s/%s, want result/incomplete", invocation.Kind, invocation.Evaluation.Result.Outcome.Kind)
	}
	assertIncompleteIsolation(t, invocation.Evaluation.Result)
	assertCompanionMatchesInvocation(t, invocation)
}

func TestEncodingCheckpointCannotLeaveAcceptedResult(t *testing.T) {
	input := validAggregateInput(t)
	result, err := Aggregate(input)
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := materializeEncodingResource(Evaluation{Result: result}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.Result.Outcome.Kind != ResultIncomplete {
		t.Fatalf("encoding resource outcome = %s, want incomplete", evaluation.Result.Outcome.Kind)
	}
	assertIncompleteIsolation(t, evaluation.Result)
}

func TestInvocationUnionClosesDigestResourceAndProcessBoundaries(t *testing.T) {
	root := filepath.Join("..", "bundle", "testdata", "upstream", "s")
	boundary := canonicalTestBoundary(t)
	artifact := mustDigest(t, "sha256:c9dde266f5791d6a1801440780cfac7208badd7ef30535603d1522f12e6b53bd")
	runtimeIdentity := runtimeIdentityForBoundary(boundary, artifact)

	digest, err := InvokeBundle(
		filepath.Join(root, "chk-digest-01", "bundle"), boundary, runtimeIdentity, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if digest.Kind != InvocationResult || digest.Evaluation.Result.Outcome.Kind != ResultRejected {
		t.Fatalf("digest invocation = %s/%s, want result/rejected", digest.Kind, digest.Evaluation.Result.Outcome.Kind)
	}
	assertCompanionMatchesInvocation(t, digest)

	resource, err := InvokeBundle(
		filepath.Join(root, "chk-resource-01", "bundle"), boundary, runtimeIdentity, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if resource.Kind != InvocationResult || resource.Evaluation.Result.Outcome.Kind != ResultIncomplete {
		t.Fatalf("resource invocation = %s/%s, want result/incomplete", resource.Kind, resource.Evaluation.Result.Outcome.Kind)
	}
	if resource.Resources.SemanticSteps <= 1 {
		t.Fatalf("resource invocation did not retain cumulative steps: %+v", resource.Resources)
	}
	assertCompanionMatchesInvocation(t, resource)

	requestRaw, err := os.ReadFile(filepath.Join(root, "chk-process-01", "bundle", "request.jcs"))
	if err != nil {
		t.Fatal(err)
	}
	process, err := RecordInvocationFailure(requestRaw, companionTestLimits)
	if err != nil {
		t.Fatal(err)
	}
	if process.Kind != InvocationFailed || len(process.Failure.Bytes()) == 0 || len(process.Companion.Bytes()) != 0 {
		t.Fatal("outer process observation coexisted with or failed to form the not-produced record")
	}
}

func TestInvocationRejectionPrecedesAccumulatedResourceExhaustion(t *testing.T) {
	source := filepath.Join("..", "bundle", "testdata", "upstream", "s", "chk-digest-01", "bundle")
	root := filepath.Join(t.TempDir(), "bundle")
	if err := os.CopyFS(root, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(root, "request.jcs")
	raw, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatal(err)
	}
	mutated := bytes.Replace(
		raw, []byte(`"name":"semantic-steps","unit":"step","value":"1000000"`),
		[]byte(`"name":"semantic-steps","unit":"step","value":"1"`), 1,
	)
	if bytes.Equal(mutated, raw) {
		t.Fatal("semantic step limit mutation did not apply")
	}
	if err := os.WriteFile(requestPath, mutated, 0o644); err != nil {
		t.Fatal(err)
	}
	boundary := canonicalTestBoundary(t)
	runtimeIdentity := runtimeIdentityForBoundary(
		boundary,
		mustDigest(t, "sha256:c9dde266f5791d6a1801440780cfac7208badd7ef30535603d1522f12e6b53bd"),
	)
	invocation, err := InvokeBundle(root, boundary, runtimeIdentity, nil)
	if err != nil {
		t.Fatal(err)
	}
	if invocation.Evaluation.Result.Outcome.Kind != ResultRejected || invocation.Resources.SemanticSteps <= 1 {
		t.Fatalf("rejection did not dominate accumulated resource exhaustion: %+v", invocation)
	}
}

func TestInvocationDoesNotFormResultWithoutBindableInputsOrChecker(t *testing.T) {
	root := filepath.Join("..", "bundle", "testdata", "upstream", "s", "chk-process-01", "bundle")
	boundary := canonicalTestBoundary(t)
	runtimeIdentity := runtimeIdentityForBoundary(
		boundary,
		mustDigest(t, "sha256:c9dde266f5791d6a1801440780cfac7208badd7ef30535603d1522f12e6b53bd"),
	)

	if invocation, err := InvokeBundle(filepath.Join(t.TempDir(), "missing"), boundary, runtimeIdentity, nil); err == nil || invocation.Kind != "" {
		t.Fatal("unreadable request formed a checker result")
	}

	noncanonical := filepath.Join(t.TempDir(), "bundle")
	if err := os.CopyFS(noncanonical, os.DirFS(root)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(noncanonical, "request.jcs"), []byte(`{ }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if invocation, err := InvokeBundle(noncanonical, boundary, runtimeIdentity, nil); err == nil || invocation.Kind != "" {
		t.Fatal("noncanonical request formed a checker result")
	}

	missingChecker := boundary
	missingChecker.Checker.Source = protocol.Digest{}
	if invocation, err := InvokeBundle(root, missingChecker, runtimeIdentity, nil); err == nil || invocation.Kind != "" {
		t.Fatal("missing checker source identity formed a checker result")
	}
}

func TestTruncatedResultRemainsAnOuterFailureObservation(t *testing.T) {
	root := filepath.Join("..", "bundle", "testdata", "upstream", "s", "chk-resource-01", "bundle")
	boundary := canonicalTestBoundary(t)
	runtimeIdentity := runtimeIdentityForBoundary(
		boundary,
		mustDigest(t, "sha256:c9dde266f5791d6a1801440780cfac7208badd7ef30535603d1522f12e6b53bd"),
	)
	invocation, err := InvokeBundle(root, boundary, runtimeIdentity, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw := invocation.Companion.Bytes()
	if _, err := ParseCompanion(raw[:len(raw)-1], companionTestLimits); err == nil {
		t.Fatal("truncated result was accepted as a four-state companion")
	}
	requestRaw, err := os.ReadFile(filepath.Join(root, "request.jcs"))
	if err != nil {
		t.Fatal(err)
	}
	failure, err := RecordInvocationFailure(requestRaw, companionTestLimits)
	if err != nil {
		t.Fatal(err)
	}
	if failure.Kind != InvocationFailed || len(failure.Companion.Bytes()) != 0 {
		t.Fatal("launcher truncation was converted into a four-state result")
	}
}

func assertCompanionMatchesInvocation(t *testing.T, invocation Invocation) {
	t.Helper()
	parsed, err := ParseCompanion(invocation.Companion.Bytes(), companionTestLimits)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Outcome.Kind != invocation.Evaluation.Result.Outcome.Kind {
		t.Fatalf("companion outcome = %s, memory outcome = %s", parsed.Outcome.Kind, invocation.Evaluation.Result.Outcome.Kind)
	}
}

func assertIncompleteIsolation(t *testing.T, result Result) {
	t.Helper()
	for _, check := range result.Checks {
		if check.Definition.Kind == CheckIsolation {
			if check.Definition.Outcome != CheckIncomplete || len(check.Definition.Codes) != 1 ||
				check.Definition.Codes[0] != "tcb-incomplete" {
				t.Fatalf("isolation check = %+v, want incomplete/tcb-incomplete", check.Definition)
			}
			return
		}
	}
	t.Fatal("incomplete result has no isolation report")
}
