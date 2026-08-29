package payloaddistribution

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/payloadarchive"
)

func TestPackIsDeterministicAndVerifiable(t *testing.T) {
	root := distributionTempDir(t)
	sourceRoot := distributionRepositoryRoot(t)
	preparedA, policy, source := writeAcceptedDistributionRoot(t, filepath.Join(root, "a"), sourceRoot, "0.1-test")
	preparedB, _, _ := writeAcceptedDistributionRoot(t, filepath.Join(root, "b"), sourceRoot, "0.1-test")
	archiveA := filepath.Join(root, ArchiveFilename(source, "0.1-test"))
	otherRoot := filepath.Join(root, "other")
	if err := os.Mkdir(otherRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	archiveB := filepath.Join(otherRoot, ArchiveFilename(source, "0.1-test"))

	resultA, err := packWithPolicy(PackConfig{
		DistributionRoot: preparedA, OutputFile: archiveA,
		SourceRoot: sourceRoot, Version: "0.1-test",
	}, policy)
	if err != nil {
		t.Fatal(err)
	}
	resultB, err := packWithPolicy(PackConfig{
		DistributionRoot: preparedB, OutputFile: archiveB,
		SourceRoot: sourceRoot, Version: "0.1-test",
	}, policy)
	if err != nil {
		t.Fatal(err)
	}
	left, err := os.ReadFile(archiveA)
	if err != nil {
		t.Fatal(err)
	}
	right, err := os.ReadFile(archiveB)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(left, right) || resultA.ArchiveSHA256 != resultB.ArchiveSHA256 {
		t.Fatal("distribution archive depends on staging path")
	}
	verified, err := verifyBytesWithPolicy(left, policy)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Source != source || verified.Version != "0.1-test" ||
		verified.CandidateSHA256 != resultA.CandidateSHA256 || verified.AcceptanceSHA256 != resultA.AcceptanceSHA256 {
		t.Fatal("distribution verification result drifted")
	}
}

func TestPackRejectsOpenRootAndWrongFilename(t *testing.T) {
	root := distributionTempDir(t)
	sourceRoot := distributionRepositoryRoot(t)
	prepared, policy, source := writeAcceptedDistributionRoot(t, filepath.Join(root, "prepared"), sourceRoot, "0.1-test")
	if err := os.WriteFile(filepath.Join(prepared, "unexpected"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := packWithPolicy(PackConfig{
		DistributionRoot: prepared, OutputFile: filepath.Join(root, ArchiveFilename(source, "0.1-test")),
		SourceRoot: sourceRoot, Version: "0.1-test",
	}, policy); err == nil {
		t.Fatal("open distribution root was packaged")
	}
	if err := os.Remove(filepath.Join(prepared, "unexpected")); err != nil {
		t.Fatal(err)
	}
	if _, err := packWithPolicy(PackConfig{
		DistributionRoot: prepared, OutputFile: filepath.Join(root, "wrong-name.tar"),
		SourceRoot: sourceRoot, Version: "0.1-test",
	}, policy); err == nil {
		t.Fatal("wrong distribution filename was accepted")
	}
}

func TestVerifyRejectsArchiveAndLicenseMutation(t *testing.T) {
	root := distributionTempDir(t)
	sourceRoot := distributionRepositoryRoot(t)
	prepared, policy, source := writeAcceptedDistributionRoot(t, filepath.Join(root, "prepared"), sourceRoot, "0.1-test")
	archive := filepath.Join(root, ArchiveFilename(source, "0.1-test"))
	if _, err := packWithPolicy(PackConfig{
		DistributionRoot: prepared, OutputFile: archive,
		SourceRoot: sourceRoot, Version: "0.1-test",
	}, policy); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	mutated := append([]byte(nil), raw...)
	mutated[len(mutated)/2] ^= 0xff
	if _, err := verifyBytesWithPolicy(mutated, policy); err == nil {
		t.Fatal("mutated distribution archive was accepted")
	}
	trailing := append(append([]byte(nil), raw...), []byte("unexpected")...)
	if _, err := verifyBytesWithPolicy(trailing, policy); err == nil {
		t.Fatal("distribution archive with trailing bytes was accepted")
	}
	if err := os.WriteFile(filepath.Join(prepared, filepath.FromSlash(GoLicenseName)), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(root, "second", ArchiveFilename(source, "0.1-test"))
	if err := os.Mkdir(filepath.Dir(second), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := packWithPolicy(PackConfig{
		DistributionRoot: prepared, OutputFile: second,
		SourceRoot: sourceRoot, Version: "0.1-test",
	}, policy); err == nil {
		t.Fatal("distribution root with changed Go license was accepted")
	}
}

func TestPackRejectsNonCanonicalOrUnknownAcceptance(t *testing.T) {
	root := distributionTempDir(t)
	sourceRoot := distributionRepositoryRoot(t)
	prepared, policy, source := writeAcceptedDistributionRoot(t, filepath.Join(root, "prepared"), sourceRoot, "0.1-test")
	acceptancePath := filepath.Join(prepared, AcceptanceName)
	raw, err := os.ReadFile(acceptancePath)
	if err != nil {
		t.Fatal(err)
	}
	unknown := append(append([]byte(nil), raw[:len(raw)-1]...), []byte(`,"unexpected":"member"}`)...)
	if err := os.WriteFile(acceptancePath, unknown, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := packWithPolicy(PackConfig{
		DistributionRoot: prepared, OutputFile: filepath.Join(root, ArchiveFilename(source, "0.1-test")),
		SourceRoot: sourceRoot, Version: "0.1-test",
	}, policy); err == nil {
		t.Fatal("distribution acceptance with unknown member was accepted")
	}
	if err := os.WriteFile(acceptancePath, append([]byte(" "), raw...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := packWithPolicy(PackConfig{
		DistributionRoot: prepared, OutputFile: filepath.Join(root, ArchiveFilename(source, "0.1-test")),
		SourceRoot: sourceRoot, Version: "0.1-test",
	}, policy); err == nil {
		t.Fatal("non-canonical distribution acceptance was accepted")
	}
}

func writeAcceptedDistributionRoot(t *testing.T, root, sourceRoot, version string) (string, materialPolicy, string) {
	t.Helper()
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	buildRoot := filepath.Join(root, "build")
	if err := os.Mkdir(buildRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, file := range []struct {
		mode os.FileMode
		name string
		raw  []byte
	}{
		{0o644, payloadarchive.ProvenanceName, []byte("synthetic provenance")},
		{0o644, payloadarchive.AcceptanceName, []byte("synthetic acceptance")},
		{0o755, payloadarchive.ExecutableName, []byte("synthetic checker artifact")},
	} {
		if err := os.WriteFile(filepath.Join(buildRoot, file.name), file.raw, file.mode); err != nil {
			t.Fatal(err)
		}
	}
	candidatePath := filepath.Join(root, "candidate-input.tar")
	candidate, err := payloadarchive.Pack(payloadarchive.PackConfig{
		BuildRoot: buildRoot, OutputFile: candidatePath, SourceRoot: sourceRoot, Version: version,
	})
	if err != nil {
		t.Fatal(err)
	}
	candidateRaw, err := os.ReadFile(candidatePath)
	if err != nil {
		t.Fatal(err)
	}
	projectLicense, err := os.ReadFile(filepath.Join(sourceRoot, "LICENSE"))
	if err != nil {
		t.Fatal(err)
	}
	goLicense := []byte("synthetic Go license\n")
	goPatents := []byte("synthetic Go patents\n")
	policy := materialPolicy{
		goLicenseBytes: int64(len(goLicense)), goLicenseSHA256: distributionDigest(goLicense),
		goPatentsBytes: int64(len(goPatents)), goPatentsSHA256: distributionDigest(goPatents),
		projectLicenseBytes: int64(len(projectLicense)), projectLicenseSHA: distributionDigest(projectLicense),
		toolchainBytes: 1234, toolchainSHA256: distributionDigest([]byte("synthetic toolchain")),
	}
	acceptedRoot := filepath.Join(root, "accepted")
	for _, directory := range []string{acceptedRoot, filepath.Join(acceptedRoot, "licenses"), filepath.Join(acceptedRoot, "licenses", "go"), filepath.Join(acceptedRoot, "licenses", "radishaxiom-checker")} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string][]byte{
		CandidateName: candidateRaw, GoLicenseName: goLicense,
		GoPatentsName: goPatents, CheckerLicenseName: projectLicense,
	}
	for name, raw := range files {
		if err := os.WriteFile(filepath.Join(acceptedRoot, filepath.FromSlash(name)), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	acceptance := acceptanceDocument{
		Acceptance: acceptanceDecision{
			AcceptedScope: append([]string(nil), acceptedScope...),
			Decision:      "accepted-for-controlled-durable-publication-candidate",
			ExcludedScope: append([]string(nil), excludedScope...),
		},
		Candidate: candidateBinding{
			Archive:           fileBinding{ByteLength: fmt.Sprint(candidate.ArchiveBytes), Filename: CandidateName, RawSHA256: candidate.ArchiveSHA256},
			RetentionManifest: fileBinding{ByteLength: fmt.Sprint(candidate.ManifestBytes), Filename: payloadarchive.ManifestName, RawSHA256: candidate.ManifestSHA256},
		},
		Format: AcceptanceFormat, FormatVersion: FormatVersion,
		Identity: acceptanceIdentity{
			Source:    candidate.Source,
			Target:    target{GOARCH: "arm64", GOARM64: "v8.0", GOOS: "darwin", MachO: "64-bit-arm64-executable"},
			Toolchain: ToolchainVersion, Version: version,
		},
		Licenses: []licenseRecord{
			{ByteLength: fmt.Sprint(len(goLicense)), Classification: "BSD-3-Clause", Path: GoLicenseName, RawSHA256: distributionDigest(goLicense), Role: "license"},
			{ByteLength: fmt.Sprint(len(goPatents)), Classification: "PatentGrant-Go", Path: GoPatentsName, RawSHA256: distributionDigest(goPatents), Role: "patent-grant"},
			{ByteLength: fmt.Sprint(len(projectLicense)), Classification: "Apache-2.0", Path: CheckerLicenseName, RawSHA256: distributionDigest(projectLicense), Role: "license"},
		},
		ToolchainPayload: fileBinding{ByteLength: fmt.Sprint(policy.toolchainBytes), Filename: ToolchainArchiveName, RawSHA256: policy.toolchainSHA256},
	}
	acceptanceRaw, err := json.Marshal(acceptance)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(acceptedRoot, AcceptanceName), acceptanceRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	return acceptedRoot, policy, candidate.Source
}

func distributionDigest(raw []byte) string {
	digest := sha256.Sum256(raw)
	return fmt.Sprintf("sha256:%x", digest)
}

func distributionTempDir(t *testing.T) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func distributionRepositoryRoot(t *testing.T) string {
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
