package main

import (
	"flag"
	"fmt"
	"os"

	"radishaxiom.dev/independent-checker-go/internal/payloadarchive"
)

func main() {
	if len(os.Args) < 2 {
		fail("usage: radishaxiom-checker-payload-archive <pack|verify> [exact options]")
	}
	switch os.Args[1] {
	case "pack":
		pack(os.Args[2:])
	case "verify":
		verify(os.Args[2:])
	default:
		fail("usage: radishaxiom-checker-payload-archive <pack|verify> [exact options]")
	}
}

func pack(arguments []string) {
	options := flag.NewFlagSet("pack", flag.ContinueOnError)
	options.SetOutput(os.Stderr)
	buildRoot := options.String("build-root", "", "accepted build root canonical realpath")
	outputFile := options.String("output-file", "", "new payload archive absolute path")
	sourceRoot := options.String("source-root", "", "checker source root canonical realpath")
	version := options.String("version", "", "exact checker implementation version")
	if err := options.Parse(arguments); err != nil || options.NArg() != 0 {
		fail("invalid payload archive pack arguments")
	}
	result, err := payloadarchive.Pack(payloadarchive.PackConfig{
		BuildRoot: *buildRoot, OutputFile: *outputFile,
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
	archive := options.String("archive", "", "payload archive canonical realpath")
	if err := options.Parse(arguments); err != nil || options.NArg() != 0 {
		fail("invalid payload archive verify arguments")
	}
	result, err := payloadarchive.Verify(*archive)
	if err != nil {
		fail(err.Error())
	}
	printResult(result)
}

func printResult(result payloadarchive.Result) {
	fmt.Printf(
		"archive=%s\narchive_bytes=%d\narchive_sha256=%s\nmanifest_bytes=%d\nmanifest_sha256=%s\nsource=%s\nversion=%s\n",
		result.ArchivePath, result.ArchiveBytes, result.ArchiveSHA256,
		result.ManifestBytes, result.ManifestSHA256, result.Source, result.Version,
	)
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "radishaxiom-checker-payload-archive:", message)
	os.Exit(1)
}
