package main

import (
	"flag"
	"fmt"
	"os"

	"radishaxiom.dev/independent-checker-go/internal/checkerartifact"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "accept" {
		fail("usage: radishaxiom-checker-accept accept --source-root=<canonical> --build-root=<canonical> --version=<exact>")
	}
	options := flag.NewFlagSet("accept", flag.ContinueOnError)
	options.SetOutput(os.Stderr)
	sourceRoot := options.String("source-root", "", "checker source root canonical realpath")
	buildRoot := options.String("build-root", "", "controlled build output canonical realpath")
	version := options.String("version", "", "exact checker implementation version")
	if err := options.Parse(os.Args[2:]); err != nil || options.NArg() != 0 {
		fail("invalid payload acceptance arguments")
	}
	result, err := checkerartifact.Accept(checkerartifact.Config{
		SourceRoot: *sourceRoot, BuildRoot: *buildRoot, Version: *version,
	})
	if err != nil {
		fail(err.Error())
	}
	fmt.Printf("acceptance=%s\nacceptance_sha256=%s\nartifact=%s\nartifact_sha256=%s\nsource=%s\nversion=%s\n",
		result.AcceptancePath, result.AcceptanceSHA256, result.ArtifactPath,
		result.ArtifactSHA256, result.Source, result.Version,
	)
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "radishaxiom-checker-accept:", message)
	os.Exit(1)
}
