package checkercli

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"radishaxiom.dev/independent-checker-go/internal/checkresult"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/sourceidentity"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

var resultTestLimits = strictjson.Limits{
	MaxBytes: 2 << 20,
	MaxDepth: 256,
	MaxItems: 100_000,
	MaxSteps: 1_000_000,
}

func TestRunEmitsOneCanonicalResultForFrozenScenarios(t *testing.T) {
	tests := []struct {
		name string
		root string
		want checkresult.ResultKind
	}{
		{name: "normal", root: "ax-b01-correct", want: checkresult.ResultAcceptedWithTrust},
		{name: "digest", root: "chk-digest-01", want: checkresult.ResultRejected},
		{name: "resource", root: "chk-resource-01", want: checkresult.ResultIncomplete},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(
				[]string{"check", "--bundle-root=" + fixtureBundle(t, test.root)},
				strings.NewReader(""), &stdout, &stderr, testIdentityResolver(t),
			)
			if code != exitSuccess {
				t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
			}
			if stderr.Len() != 0 || stdout.Len() == 0 || bytes.HasSuffix(stdout.Bytes(), []byte("\n")) {
				t.Fatalf("stdout/stderr boundary = %d/%q", stdout.Len(), stderr.String())
			}
			document, err := checkresult.ParseCompanion(stdout.Bytes(), resultTestLimits)
			if err != nil {
				t.Fatal(err)
			}
			if document.Outcome.Kind != test.want {
				t.Fatalf("outcome = %s, want %s", document.Outcome.Kind, test.want)
			}
			if document.Checker.Artifact == document.Checker.Source {
				t.Fatal("runtime artifact collapsed into source identity")
			}
		})
	}
}

func TestRunIsByteDeterministic(t *testing.T) {
	root := fixtureBundle(t, "ax-b01-correct")
	resolve := testIdentityResolver(t)
	var expected []byte
	for iteration := 0; iteration < 8; iteration++ {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if code := Run(
			[]string{"check", "--bundle-root=" + root}, strings.NewReader(""),
			&stdout, &stderr, resolve,
		); code != exitSuccess {
			t.Fatalf("iteration %d exit = %d, stderr = %q", iteration, code, stderr.String())
		}
		if iteration == 0 {
			expected = append([]byte(nil), stdout.Bytes()...)
			continue
		}
		if !bytes.Equal(stdout.Bytes(), expected) {
			t.Fatalf("iteration %d changed canonical bytes", iteration)
		}
	}
}

func TestRunRejectsNonFrozenInvocationFormsBeforeIdentity(t *testing.T) {
	root := fixtureBundle(t, "ax-b01-correct")
	tests := [][]string{
		nil,
		{"check"},
		{"CHECK", "--bundle-root=" + root},
		{"check", "--bundle-root", root},
		{"check", "--bundle-root=" + root, "extra"},
		{"check", "--bundle-root=relative"},
		{"check", "--bundle-root=" + root + string(os.PathSeparator)},
		{"check", "--bundle-root=" + filepath.Join(root, "missing")},
	}
	for _, args := range tests {
		calls := 0
		resolve := func() (InvocationIdentity, error) {
			calls++
			return InvocationIdentity{}, errors.New("must not be called")
		}
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if code := Run(args, strings.NewReader(""), &stdout, &stderr, resolve); code != exitUsage {
			t.Fatalf("args %q exit = %d, want %d", args, code, exitUsage)
		}
		assertNoResultDiagnostic(t, stdout.Bytes(), stderr.Bytes())
		if calls != 0 {
			t.Fatalf("args %q resolved identity %d times", args, calls)
		}
	}
}

func TestRunRejectsSymlinkRootAndNonemptyStdin(t *testing.T) {
	realRoot := fixtureBundle(t, "ax-b01-correct")
	temp := canonicalPath(t, t.TempDir())
	link := filepath.Join(temp, "bundle-link")
	if err := os.Symlink(realRoot, link); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		root  string
		stdin string
	}{
		{name: "symlink", root: link},
		{name: "stdin", root: realRoot, stdin: "x"},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			resolve := func() (InvocationIdentity, error) {
				calls++
				return InvocationIdentity{}, errors.New("must not be called")
			}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if code := Run(
				[]string{"check", "--bundle-root=" + test.root}, strings.NewReader(test.stdin),
				&stdout, &stderr, resolve,
			); code != exitUsage {
				t.Fatalf("exit = %d, want %d", code, exitUsage)
			}
			assertNoResultDiagnostic(t, stdout.Bytes(), stderr.Bytes())
			if calls != 0 {
				t.Fatalf("resolved identity %d times", calls)
			}
		})
	}
}

func TestRunKeepsIdentityAndBundleFailuresOutsideResult(t *testing.T) {
	emptyRoot := canonicalPath(t, t.TempDir())
	tests := []struct {
		name    string
		root    string
		resolve IdentityResolver
	}{
		{
			name: "identity", root: fixtureBundle(t, "ax-b01-correct"),
			resolve: func() (InvocationIdentity, error) {
				return InvocationIdentity{}, errors.New("synthetic identity failure")
			},
		},
		{name: "bundle", root: emptyRoot, resolve: testIdentityResolver(t)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if code := Run(
				[]string{"check", "--bundle-root=" + test.root}, strings.NewReader(""),
				&stdout, &stderr, test.resolve,
			); code != exitFailure {
				t.Fatalf("exit = %d, want %d", code, exitFailure)
			}
			assertNoResultDiagnostic(t, stdout.Bytes(), stderr.Bytes())
		})
	}
}

func TestRunMakesPartialStdoutUnparseable(t *testing.T) {
	written := &failingWriter{remaining: 37}
	var stderr bytes.Buffer
	code := Run(
		[]string{"check", "--bundle-root=" + fixtureBundle(t, "ax-b01-correct")},
		strings.NewReader(""), written, &stderr, testIdentityResolver(t),
	)
	if code != exitFailure {
		t.Fatalf("exit = %d, want %d", code, exitFailure)
	}
	if len(written.data) != 37 || stderr.Len() == 0 {
		t.Fatalf("partial stdout/stderr = %d/%d", len(written.data), stderr.Len())
	}
	if _, err := checkresult.ParseCompanion(written.data, resultTestLimits); err == nil {
		t.Fatal("partial stdout parsed as a canonical result")
	}
}

func TestResolveRuntimeIdentityBindsExactExecutable(t *testing.T) {
	input := testRuntimeInputs(t)
	identity, err := ResolveRuntimeIdentity(input)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(input.Executable)
	if err != nil {
		t.Fatal(err)
	}
	wantArtifact := protocol.Digest(sha256.Sum256(raw))
	if identity.Runtime.CheckerArtifact != wantArtifact ||
		identity.Boundary.Checker.Source.String() != input.Source ||
		identity.Boundary.Checker.Toolchain != input.Toolchain ||
		identity.Boundary.Checker.Version != input.Version {
		t.Fatalf("resolved identity mismatch: %+v", identity)
	}
	if len(identity.Runtime.TCB) != len(identity.Boundary.TCB) || len(identity.Runtime.TCB) == 0 {
		t.Fatalf("runtime/source TCB cardinality = %d/%d", len(identity.Runtime.TCB), len(identity.Boundary.TCB))
	}
	for index, component := range identity.Runtime.TCB {
		if component.Artifact != wantArtifact ||
			component.Category != identity.Boundary.TCB[index].Category ||
			component.Version != identity.Boundary.TCB[index].Version {
			t.Fatalf("runtime TCB[%d] mismatch: %+v", index, component)
		}
	}
}

func TestResolveRuntimeIdentityFailsClosed(t *testing.T) {
	valid := testRuntimeInputs(t)
	raw, err := os.ReadFile(valid.Executable)
	if err != nil {
		t.Fatal(err)
	}
	artifact := protocol.Digest(sha256.Sum256(raw)).String()
	tests := []struct {
		name   string
		mutate func(*RuntimeInputs)
	}{
		{name: "source", mutate: func(input *RuntimeInputs) { input.Source = "invalid" }},
		{name: "toolchain", mutate: func(input *RuntimeInputs) { input.Toolchain = "go1.26.3" }},
		{name: "version", mutate: func(input *RuntimeInputs) { input.Version = "latest" }},
		{name: "collapsed", mutate: func(input *RuntimeInputs) { input.Source = artifact }},
		{name: "relative-executable", mutate: func(input *RuntimeInputs) { input.Executable = "checker" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := valid
			test.mutate(&input)
			if _, err := ResolveRuntimeIdentity(input); err == nil {
				t.Fatal("invalid runtime identity was accepted")
			}
		})
	}
}

func TestDiagnosticIsBoundedSingleLineUTF8(t *testing.T) {
	var stderr bytes.Buffer
	writeDiagnostic(&stderr, errors.New(strings.Repeat("界", 30_000)+"\n\xff"))
	if stderr.Len() > diagnosticLimit || !utf8.Valid(stderr.Bytes()) {
		t.Fatalf("diagnostic boundary = %d bytes, utf8=%t", stderr.Len(), utf8.Valid(stderr.Bytes()))
	}
	if bytes.Count(stderr.Bytes(), []byte("\n")) != 1 || stderr.Bytes()[stderr.Len()-1] != '\n' {
		t.Fatal("diagnostic is not one terminated line")
	}
}

type failingWriter struct {
	remaining int
	data      []byte
}

func (writer *failingWriter) Write(data []byte) (int, error) {
	if writer.remaining == 0 {
		return 0, errors.New("synthetic output failure")
	}
	count := len(data)
	if count > writer.remaining {
		count = writer.remaining
	}
	writer.data = append(writer.data, data[:count]...)
	writer.remaining -= count
	if count != len(data) {
		return count, errors.New("synthetic output failure")
	}
	return count, nil
}

func testIdentityResolver(t *testing.T) IdentityResolver {
	t.Helper()
	input := testRuntimeInputs(t)
	return func() (InvocationIdentity, error) {
		return ResolveRuntimeIdentity(input)
	}
}

func testRuntimeInputs(t *testing.T) RuntimeInputs {
	t.Helper()
	snapshot, err := sourceidentity.Generate(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	directory := canonicalPath(t, t.TempDir())
	executable := filepath.Join(directory, "synthetic-checker")
	if err := os.WriteFile(executable, []byte("synthetic checker executable\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return RuntimeInputs{
		Source:     snapshot.Digest,
		Toolchain:  "go1.26.7",
		Version:    "0.1-test",
		Executable: executable,
	}
}

func fixtureBundle(t *testing.T, name string) string {
	t.Helper()
	return canonicalPath(t, filepath.Join("..", "bundle", "testdata", "upstream", "s", name, "bundle"))
}

func canonicalPath(t *testing.T, path string) string {
	t.Helper()
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func assertNoResultDiagnostic(t *testing.T, stdout []byte, stderr []byte) {
	t.Helper()
	if len(stdout) != 0 {
		t.Fatalf("failure emitted %d stdout bytes", len(stdout))
	}
	if len(stderr) == 0 || len(stderr) > diagnosticLimit || !utf8.Valid(stderr) {
		t.Fatalf("diagnostic boundary = %d bytes, utf8=%t", len(stderr), utf8.Valid(stderr))
	}
}
