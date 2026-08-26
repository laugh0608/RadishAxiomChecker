package checkresult

import (
	"os"
	"path/filepath"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

func TestInvocationFailureMatchesLockedProcessScenario(t *testing.T) {
	root := filepath.Join("..", "bundle", "testdata", "upstream", "s", "chk-process-01")
	requestRaw, err := os.ReadFile(filepath.Join(root, "bundle", "request.jcs"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(root, "expected-process-failure.jcs"))
	if err != nil {
		t.Fatal(err)
	}
	failure, err := NewInvocationFailure(requestRaw, companionTestLimits)
	if err != nil {
		t.Fatal(err)
	}
	if string(failure.Bytes()) != string(want) {
		t.Fatalf("invocation failure bytes differ from locked process scenario:\n got %s\nwant %s", failure.Bytes(), want)
	}
	if err := failure.VerifyRequest(requestRaw, companionTestLimits); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseCompanion(failure.Bytes(), companionTestLimits); err == nil {
		t.Fatal("invocation failure was accepted as a four-state checker result")
	}
}

func TestInvocationFailureRejectsUnboundOrResultProducingRecords(t *testing.T) {
	root := filepath.Join("..", "bundle", "testdata", "upstream", "s", "chk-process-01")
	requestRaw, err := os.ReadFile(filepath.Join(root, "bundle", "request.jcs"))
	if err != nil {
		t.Fatal(err)
	}
	failure, err := NewInvocationFailure(requestRaw, companionTestLimits)
	if err != nil {
		t.Fatal(err)
	}

	resultProducing := replaceOnce(t, string(failure.Bytes()), `"result":"not-produced"`, `"result":"incomplete"`)
	_, err = ParseInvocationFailure([]byte(resultProducing), companionTestLimits)
	assertRejectionCode(t, err, rejection.ResultAggregation)

	tampered := replaceOnce(
		t, string(failure.Bytes()),
		`"content_digest":"sha256:880b1913edc7df0d8b6c4e3f50103f6b483c4b91f3d449fb5e206442571703d5"`,
		`"content_digest":"sha256:980b1913edc7df0d8b6c4e3f50103f6b483c4b91f3d449fb5e206442571703d5"`,
	)
	parsed, err := ParseInvocationFailure([]byte(tampered), companionTestLimits)
	if err != nil {
		t.Fatal(err)
	}
	assertRejectionCode(t, parsed.VerifyRequest(requestRaw, companionTestLimits), rejection.RequestBindingMismatch)

	if _, err := NewInvocationFailure([]byte(`{}`), companionTestLimits); err == nil {
		t.Fatal("noncanonical or incomplete request formed a falsely bound invocation failure")
	}
}
