package distributionaccept

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"radishaxiom.dev/independent-checker-go/internal/payloadarchive"
)

func TestAcceptMaterializesClosedDistributionRoot(t *testing.T) {
	root := canonicalTempDir(t)
	sourceRoot := repositoryRoot(t)
	candidate := writeCandidate(t, root, sourceRoot, "0.1-test")
	toolchain, policy := writeSyntheticToolchain(t, root, sourceRoot, []byte("synthetic Go license\n"), []byte("synthetic Go patents\n"))
	output := filepath.Join(root, "accepted-distribution")

	result, err := acceptWithPolicy(Config{
		CandidateArchive: candidate, OutputRoot: output, SourceRoot: sourceRoot,
		ToolchainArchive: toolchain, Version: "0.1-test",
	}, policy)
	if err != nil {
		t.Fatal(err)
	}
	if result.OutputRoot != output || result.Source == result.CandidateSHA256 ||
		result.Version != "0.1-test" || result.AcceptanceBytes == 0 {
		t.Fatal("distribution acceptance result identity drifted")
	}
	acceptanceRaw, err := os.ReadFile(filepath.Join(output, AcceptanceName))
	if err != nil {
		t.Fatal(err)
	}
	var document acceptanceDocument
	if err := jsonDecodeCanonical(acceptanceRaw, &document); err != nil {
		t.Fatal(err)
	}
	if document.Acceptance.Decision != "accepted-for-controlled-durable-publication-candidate" ||
		len(document.Licenses) != 3 || document.Licenses[2].Classification != "Apache-2.0" {
		t.Fatal("distribution acceptance decision or legal inventory drifted")
	}
	for _, path := range []string{CandidateName, AcceptanceName, GoLicenseName, GoPatentsName, CheckerLicenseName} {
		info, err := os.Lstat(filepath.Join(output, filepath.FromSlash(path)))
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o644 {
			t.Fatalf("materialized file profile mismatch: %s", path)
		}
	}
}

func TestAcceptRejectsGoLegalIdentityDrift(t *testing.T) {
	root := canonicalTempDir(t)
	sourceRoot := repositoryRoot(t)
	candidate := writeCandidate(t, root, sourceRoot, "0.1-test")
	toolchain, policy := writeSyntheticToolchain(t, root, sourceRoot, []byte("synthetic Go license\n"), []byte("synthetic Go patents\n"))
	policy.goLicenseSHA256 = digest([]byte("different license"))
	output := filepath.Join(root, "accepted-distribution")
	_, err := acceptWithPolicy(Config{
		CandidateArchive: candidate, OutputRoot: output, SourceRoot: sourceRoot,
		ToolchainArchive: toolchain, Version: "0.1-test",
	}, policy)
	if err == nil {
		t.Fatal("Go license identity drift was accepted")
	}
	if _, statErr := os.Lstat(output); !os.IsNotExist(statErr) {
		t.Fatal("failed acceptance materialized an output root")
	}
}

func TestAcceptRejectsToolchainArchiveMutation(t *testing.T) {
	root := canonicalTempDir(t)
	sourceRoot := repositoryRoot(t)
	candidate := writeCandidate(t, root, sourceRoot, "0.1-test")
	toolchain, policy := writeSyntheticToolchain(t, root, sourceRoot, []byte("synthetic Go license\n"), []byte("synthetic Go patents\n"))
	file, err := os.OpenFile(toolchain, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("mutation")); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = acceptWithPolicy(Config{
		CandidateArchive: candidate, OutputRoot: filepath.Join(root, "accepted-distribution"),
		SourceRoot: sourceRoot, ToolchainArchive: toolchain, Version: "0.1-test",
	}, policy)
	if err == nil {
		t.Fatal("mutated toolchain archive was accepted")
	}
}

func TestAcceptRejectsExistingOutputAndVersionDrift(t *testing.T) {
	root := canonicalTempDir(t)
	sourceRoot := repositoryRoot(t)
	candidate := writeCandidate(t, root, sourceRoot, "0.1-test")
	toolchain, policy := writeSyntheticToolchain(t, root, sourceRoot, []byte("synthetic Go license\n"), []byte("synthetic Go patents\n"))
	existing := filepath.Join(root, "existing")
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := acceptWithPolicy(Config{
		CandidateArchive: candidate, OutputRoot: existing, SourceRoot: sourceRoot,
		ToolchainArchive: toolchain, Version: "0.1-test",
	}, policy); err == nil {
		t.Fatal("existing output root was accepted")
	}
	if _, err := acceptWithPolicy(Config{
		CandidateArchive: candidate, OutputRoot: filepath.Join(root, "version-drift"),
		SourceRoot: sourceRoot, ToolchainArchive: toolchain, Version: "0.2-test",
	}, policy); err == nil {
		t.Fatal("candidate version drift was accepted")
	}
}

func writeCandidate(t *testing.T, root, sourceRoot, version string) string {
	t.Helper()
	buildRoot := filepath.Join(root, "accepted-build")
	if err := os.Mkdir(buildRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	files := []struct {
		mode os.FileMode
		name string
		raw  []byte
	}{
		{0o644, payloadarchive.ProvenanceName, []byte(`{"synthetic":"provenance"}`)},
		{0o644, payloadarchive.AcceptanceName, []byte(`{"synthetic":"acceptance"}`)},
		{0o755, payloadarchive.ExecutableName, []byte("synthetic checker artifact")},
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(buildRoot, file.name), file.raw, file.mode); err != nil {
			t.Fatal(err)
		}
	}
	candidate := filepath.Join(root, "candidate.tar")
	if _, err := payloadarchive.Pack(payloadarchive.PackConfig{
		BuildRoot: buildRoot, OutputFile: candidate, SourceRoot: sourceRoot, Version: version,
	}); err != nil {
		t.Fatal(err)
	}
	return candidate
}

func writeSyntheticToolchain(t *testing.T, root, sourceRoot string, license, patents []byte) (string, materialPolicy) {
	t.Helper()
	var archive bytes.Buffer
	gzipWriter := gzip.NewWriter(&archive)
	gzipWriter.Header.ModTime = time.Unix(0, 0).UTC()
	gzipWriter.Header.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)
	for _, file := range []struct {
		name string
		raw  []byte
	}{{"go/LICENSE", license}, {"go/PATENTS", patents}, {"go/VERSION", []byte("go1.26.7\n")}} {
		header := &tar.Header{
			Name: file.name, Mode: 0o644, Size: int64(len(file.raw)),
			ModTime: time.Unix(0, 0).UTC(), Typeflag: tar.TypeReg, Format: tar.FormatUSTAR,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write(file.raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "synthetic-go.tar.gz")
	if err := os.WriteFile(path, archive.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	projectLicense, err := os.ReadFile(filepath.Join(sourceRoot, "LICENSE"))
	if err != nil {
		t.Fatal(err)
	}
	return path, materialPolicy{
		goLicenseBytes: int64(len(license)), goLicenseSHA256: digest(license),
		goPatentsBytes: int64(len(patents)), goPatentsSHA256: digest(patents),
		projectLicenseBytes: int64(len(projectLicense)), projectLicenseSHA: digest(projectLicense),
		toolchainBytes: int64(archive.Len()), toolchainSHA256: digest(archive.Bytes()),
	}
}

func jsonDecodeCanonical(raw []byte, output any) error {
	if err := json.Unmarshal(raw, output); err != nil {
		return err
	}
	reencoded, err := json.Marshal(output)
	if err != nil || !bytes.Equal(raw, reencoded) {
		return fmt.Errorf("JSON is not canonical")
	}
	return nil
}

func digest(raw []byte) string {
	value := sha256.Sum256(raw)
	return fmt.Sprintf("sha256:%x", value)
}

func canonicalTempDir(t *testing.T) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test source path unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}
