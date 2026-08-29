package main

import (
	"flag"
	"fmt"
	"os"

	"radishaxiom.dev/independent-checker-go/internal/payloaddistribution"
)

func main() {
	if len(os.Args) < 2 {
		fail("usage: radishaxiom-checker-payload-distribution <pack|verify> [exact options]")
	}
	switch os.Args[1] {
	case "pack":
		pack(os.Args[2:])
	case "verify":
		verify(os.Args[2:])
	default:
		fail("usage: radishaxiom-checker-payload-distribution <pack|verify> [exact options]")
	}
}

func pack(arguments []string) {
	options := flag.NewFlagSet("pack", flag.ContinueOnError)
	options.SetOutput(os.Stderr)
	distributionRoot := options.String("distribution-root", "", "accepted distribution root canonical realpath")
	outputFile := options.String("output-file", "", "new distribution archive absolute path")
	sourceRoot := options.String("source-root", "", "checker source root canonical realpath")
	version := options.String("version", "", "exact checker implementation version")
	if err := options.Parse(arguments); err != nil || options.NArg() != 0 {
		fail("invalid payload distribution pack arguments")
	}
	result, err := payloaddistribution.Pack(payloaddistribution.PackConfig{
		DistributionRoot: *distributionRoot, OutputFile: *outputFile,
		SourceRoot: *sourceRoot, Version: *version,
	})
	if err != nil {
		fail(err.Error())
	}
	printResult(result)
}

func verify(arguments []string) {
	options := flag.NewFlagSet("verify", flag.ContinueOnError)
	options.SetOutput(os.Stderr)
	archive := options.String("archive", "", "distribution archive canonical realpath")
	if err := options.Parse(arguments); err != nil || options.NArg() != 0 {
		fail("invalid payload distribution verify arguments")
	}
	result, err := payloaddistribution.Verify(*archive)
	if err != nil {
		fail(err.Error())
	}
	printResult(result)
}

func printResult(result payloaddistribution.Result) {
	fmt.Printf(
		"acceptance_bytes=%d\nacceptance_sha256=%s\narchive=%s\narchive_bytes=%d\narchive_sha256=%s\ncandidate_bytes=%d\ncandidate_sha256=%s\nmanifest_bytes=%d\nmanifest_sha256=%s\nsource=%s\nversion=%s\n",
		result.AcceptanceBytes, result.AcceptanceSHA256, result.ArchivePath,
		result.ArchiveBytes, result.ArchiveSHA256, result.CandidateBytes,
		result.CandidateSHA256, result.ManifestBytes, result.ManifestSHA256,
		result.Source, result.Version,
	)
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "radishaxiom-checker-payload-distribution:", message)
	os.Exit(1)
}
