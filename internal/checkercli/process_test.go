package checkercli

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/checkresult"
	"radishaxiom.dev/independent-checker-go/internal/sourceidentity"
)

const processHelperEnvironment = "RADISHAXIOM_CHECKERCLI_PROCESS_HELPER"

func TestRunProcessBoundary(t *testing.T) {
	root := fixtureBundle(t, "ax-b01-correct")
	tests := []struct {
		name         string
		args         []string
		stdin        string
		identityMode string
		wantExit     int
		wantOutcome  checkresult.ResultKind
	}{
		{
			name: "result", args: []string{"check", "--bundle-root=" + root},
			wantExit: exitSuccess, wantOutcome: checkresult.ResultAcceptedWithTrust,
		},
		{name: "usage", args: []string{"check", "--bundle-root", root}, wantExit: exitUsage},
		{name: "stdin", args: []string{"check", "--bundle-root=" + root}, stdin: "x", wantExit: exitUsage},
		{
			name: "identity", args: []string{"check", "--bundle-root=" + root},
			identityMode: "fail", wantExit: exitFailure,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			arguments := []string{"-test.run=^TestCLIProcessHelper$", "--"}
			arguments = append(arguments, test.args...)
			command := exec.Command(executable, arguments...)
			command.Env = append(
				os.Environ(),
				processHelperEnvironment+"=1",
				processHelperEnvironment+"_IDENTITY="+test.identityMode,
			)
			command.Stdin = strings.NewReader(test.stdin)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr
			err = command.Run()
			gotExit := 0
			if err != nil {
				var exitError *exec.ExitError
				if !errors.As(err, &exitError) {
					t.Fatal(err)
				}
				gotExit = exitError.ExitCode()
			}
			if gotExit != test.wantExit {
				t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q", gotExit, test.wantExit, stdout.String(), stderr.String())
			}
			if test.wantExit != exitSuccess {
				assertNoResultDiagnostic(t, stdout.Bytes(), stderr.Bytes())
				return
			}
			if stderr.Len() != 0 {
				t.Fatalf("success stderr = %q", stderr.String())
			}
			document, err := checkresult.ParseCompanion(stdout.Bytes(), resultTestLimits)
			if err != nil {
				t.Fatal(err)
			}
			if document.Outcome.Kind != test.wantOutcome {
				t.Fatalf("outcome = %s, want %s", document.Outcome.Kind, test.wantOutcome)
			}
		})
	}
}

func TestCLIProcessHelper(t *testing.T) {
	if os.Getenv(processHelperEnvironment) != "1" {
		return
	}
	arguments := []string{}
	for index, argument := range os.Args {
		if argument == "--" {
			arguments = os.Args[index+1:]
			break
		}
	}
	resolve := processIdentityResolver(t)
	if os.Getenv(processHelperEnvironment+"_IDENTITY") == "fail" {
		resolve = func() (InvocationIdentity, error) {
			return InvocationIdentity{}, errors.New("synthetic process identity failure")
		}
	}
	os.Exit(Run(arguments, os.Stdin, os.Stdout, os.Stderr, resolve))
}

func processIdentityResolver(t *testing.T) IdentityResolver {
	t.Helper()
	snapshot, err := sourceidentity.Generate(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable = canonicalPath(t, executable)
	input := RuntimeInputs{
		Source:     snapshot.Digest,
		Toolchain:  "go1.26.7",
		Version:    "0.1-test",
		Executable: executable,
	}
	return func() (InvocationIdentity, error) {
		return ResolveRuntimeIdentity(input)
	}
}
