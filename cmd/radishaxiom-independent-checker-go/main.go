package main

import (
	"os"
	"runtime"

	"radishaxiom.dev/independent-checker-go/internal/checkercli"
)

// These values are mandatory build inputs. Keeping the zero values unusable
// prevents an ordinary local build from masquerading as a registered checker.
var (
	checkerSource  string
	checkerVersion string
)

func main() {
	os.Exit(checkercli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, resolveRuntimeIdentity))
}

func resolveRuntimeIdentity() (checkercli.InvocationIdentity, error) {
	executable, err := os.Executable()
	if err != nil {
		return checkercli.InvocationIdentity{}, err
	}
	return checkercli.ResolveRuntimeIdentity(checkercli.RuntimeInputs{
		Source:     checkerSource,
		Toolchain:  runtime.Version(),
		Version:    checkerVersion,
		Executable: executable,
	})
}
