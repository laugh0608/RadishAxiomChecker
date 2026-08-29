package main

import (
	"flag"
	"fmt"
	"os"

	"radishaxiom.dev/independent-checker-go/internal/distributionaccept"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "accept" {
		fail("usage: radishaxiom-checker-distribution-accept accept --source-root=<canonical> --candidate-archive=<canonical> --toolchain-archive=<canonical> --output-root=<new-absolute-path> --version=<exact>")
	}
	options := flag.NewFlagSet("accept", flag.ContinueOnError)
	options.SetOutput(os.Stderr)
	sourceRoot := options.String("source-root", "", "checker source root canonical realpath")
	candidateArchive := options.String("candidate-archive", "", "accepted candidate archive canonical realpath")
	toolchainArchive := options.String("toolchain-archive", "", "accepted Go toolchain archive canonical realpath")
	outputRoot := options.String("output-root", "", "new accepted distribution root absolute path")
	version := options.String("version", "", "exact checker implementation version")
	if err := options.Parse(os.Args[2:]); err != nil || options.NArg() != 0 {
		fail("invalid distribution acceptance arguments")
	}
	result, err := distributionaccept.Accept(distributionaccept.Config{
		CandidateArchive: *candidateArchive, OutputRoot: *outputRoot,
		SourceRoot: *sourceRoot, ToolchainArchive: *toolchainArchive, Version: *version,
	})
	if err != nil {
		fail(err.Error())
	}
	fmt.Printf(
		"acceptance=%s\nacceptance_bytes=%d\nacceptance_sha256=%s\ncandidate_bytes=%d\ncandidate_sha256=%s\ndistribution_root=%s\nsource=%s\nversion=%s\n",
		result.AcceptancePath, result.AcceptanceBytes, result.AcceptanceSHA256,
		result.CandidateBytes, result.CandidateSHA256, result.OutputRoot,
		result.Source, result.Version,
	)
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "radishaxiom-checker-distribution-accept:", message)
	os.Exit(1)
}
