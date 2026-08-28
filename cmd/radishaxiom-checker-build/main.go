package main

import (
	"flag"
	"fmt"
	"os"

	"radishaxiom.dev/independent-checker-go/internal/checkerbuild"
)

func main() {
	if len(os.Args) == 0 || len(os.Args) < 2 || os.Args[1] != "build" {
		fail("usage: radishaxiom-checker-build build --source-root=<canonical> --toolchain-archive=<canonical> --output-root=<canonical-empty-dir> --version=<exact>")
	}
	options := flag.NewFlagSet("build", flag.ContinueOnError)
	options.SetOutput(os.Stderr)
	sourceRoot := options.String("source-root", "", "checker source root canonical realpath")
	toolchainArchive := options.String("toolchain-archive", "", "accepted Go archive canonical realpath")
	outputRoot := options.String("output-root", "", "empty output directory canonical realpath")
	version := options.String("version", "", "exact checker implementation version")
	if err := options.Parse(os.Args[2:]); err != nil || options.NArg() != 0 {
		fail("invalid controlled build arguments")
	}
	result, err := checkerbuild.Build(checkerbuild.Config{
		SourceRoot: *sourceRoot, ToolchainArchive: *toolchainArchive,
		OutputRoot: *outputRoot, Version: *version,
	})
	if err != nil {
		fail(err.Error())
	}
	fmt.Printf("artifact=%s\nartifact_sha256=%s\nprovenance=%s\nsource=%s\nversion=%s\n",
		result.ArtifactPath, result.ArtifactSHA256, result.ProvenancePath,
		result.Source, result.Version,
	)
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "radishaxiom-checker-build:", message)
	os.Exit(1)
}
