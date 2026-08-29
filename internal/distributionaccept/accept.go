package distributionaccept

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"radishaxiom.dev/independent-checker-go/internal/payloadarchive"
	"radishaxiom.dev/independent-checker-go/internal/sourceidentity"
)

const (
	AcceptanceName       = "checker-payload-distribution-acceptance-v0.1.jcs"
	CandidateName        = "checker-payload-candidate.tar"
	CheckerLicenseName   = "licenses/radishaxiom-checker/LICENSE"
	Format               = "radishaxiom-checker-payload-distribution-acceptance"
	FormatVersion        = "0.1"
	GoLicenseName        = "licenses/go/LICENSE"
	GoPatentsName        = "licenses/go/PATENTS"
	ToolchainArchiveName = "go1.26.7.darwin-arm64.tar.gz"
	ToolchainVersion     = "go1.26.7"

	maxCandidateBytes = int64(16 * 1024 * 1024)
	maxLegalBytes     = int64(64 * 1024)
	maxToolchainBytes = int64(128 * 1024 * 1024)
)

var acceptedScope = []string{
	"distribution-byte-inventory",
	"license-material-inclusion",
	"payload-identity-binding",
	"target-scoped-distribution",
}

var excludedScope = []string{
	"cross-platform-equivalence",
	"installation",
	"jurisdiction-wide-legal-compliance",
	"launcher-hard-isolation",
	"provider-publication",
	"release-signing",
	"runtime-activation",
}

type Config struct {
	CandidateArchive string
	OutputRoot       string
	SourceRoot       string
	ToolchainArchive string
	Version          string
}

type Result struct {
	AcceptanceBytes  int64
	AcceptancePath   string
	AcceptanceSHA256 string
	CandidateBytes   int64
	CandidateSHA256  string
	OutputRoot       string
	Source           string
	Version          string
}

type materialPolicy struct {
	goLicenseBytes      int64
	goLicenseSHA256     string
	goPatentsBytes      int64
	goPatentsSHA256     string
	projectLicenseBytes int64
	projectLicenseSHA   string
	toolchainBytes      int64
	toolchainSHA256     string
}

var productionPolicy = materialPolicy{
	goLicenseBytes:      1453,
	goLicenseSHA256:     "sha256:911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad",
	goPatentsBytes:      1303,
	goPatentsSHA256:     "sha256:96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc",
	projectLicenseBytes: 11357,
	projectLicenseSHA:   "sha256:c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4",
	toolchainBytes:      64772572,
	toolchainSHA256:     "sha256:020a1e8224811be75163e920bc77e0926a1390a6aeea19bdcf23f74b9d749f6d",
}

type acceptanceDocument struct {
	Acceptance       acceptanceDecision `json:"acceptance"`
	Candidate        candidateBinding   `json:"candidate"`
	Format           string             `json:"format"`
	FormatVersion    string             `json:"format_version"`
	Identity         identity           `json:"identity"`
	Licenses         []licenseRecord    `json:"licenses"`
	ToolchainPayload fileBinding        `json:"toolchain_payload"`
}

type acceptanceDecision struct {
	AcceptedScope []string `json:"accepted_scope"`
	Decision      string   `json:"decision"`
	ExcludedScope []string `json:"excluded_scope"`
}

type candidateBinding struct {
	Archive           fileBinding `json:"archive"`
	RetentionManifest fileBinding `json:"retention_manifest"`
}

type fileBinding struct {
	ByteLength string `json:"byte_length"`
	Filename   string `json:"filename"`
	RawSHA256  string `json:"raw_sha256"`
}

type identity struct {
	Source    string `json:"source"`
	Target    target `json:"target"`
	Toolchain string `json:"toolchain"`
	Version   string `json:"version"`
}

type target struct {
	GOARCH  string `json:"goarch"`
	GOARM64 string `json:"goarm64"`
	GOOS    string `json:"goos"`
	MachO   string `json:"macho"`
}

type licenseRecord struct {
	ByteLength     string `json:"byte_length"`
	Classification string `json:"classification"`
	Path           string `json:"path"`
	RawSHA256      string `json:"raw_sha256"`
	Role           string `json:"role"`
}

type legalMaterials struct {
	goLicense      []byte
	goPatents      []byte
	projectLicense []byte
}

// Accept independently binds the exact candidate and legal materials into a
// closed distribution root. It does not publish, sign, install, or activate it.
func Accept(config Config) (Result, error) {
	return acceptWithPolicy(config, productionPolicy)
}

func acceptWithPolicy(config Config, policy materialPolicy) (result Result, err error) {
	sourceRoot, err := canonicalDirectory(config.SourceRoot)
	if err != nil {
		return Result{}, fmt.Errorf("source root: %w", err)
	}
	candidatePath, err := canonicalRegularFile(config.CandidateArchive, 0o644)
	if err != nil {
		return Result{}, fmt.Errorf("candidate archive: %w", err)
	}
	toolchainPath, err := canonicalRegularFile(config.ToolchainArchive, 0o644)
	if err != nil {
		return Result{}, fmt.Errorf("toolchain archive: %w", err)
	}
	outputRoot, err := canonicalNewDirectory(config.OutputRoot)
	if err != nil {
		return Result{}, fmt.Errorf("output root: %w", err)
	}
	if err := validateVersion(config.Version); err != nil {
		return Result{}, err
	}
	snapshot, err := sourceidentity.Verify(sourceRoot)
	if err != nil {
		return Result{}, fmt.Errorf("verify checker source identity: %w", err)
	}

	candidateRaw, err := readStableFile(candidatePath, maxCandidateBytes)
	if err != nil {
		return Result{}, fmt.Errorf("read candidate archive: %w", err)
	}
	candidate, err := payloadarchive.VerifyBytes(candidateRaw)
	if err != nil {
		return Result{}, fmt.Errorf("verify candidate archive: %w", err)
	}
	if candidate.Source != snapshot.Digest || candidate.Version != config.Version {
		return Result{}, fmt.Errorf("candidate archive identity mismatch")
	}

	projectLicensePath, err := canonicalRegularFile(filepath.Join(sourceRoot, "LICENSE"), 0o644)
	if err != nil {
		return Result{}, fmt.Errorf("checker license: %w", err)
	}
	projectLicense, err := readStableFile(projectLicensePath, maxLegalBytes)
	if err != nil {
		return Result{}, fmt.Errorf("read checker license: %w", err)
	}
	if err := requireIdentity(projectLicense, policy.projectLicenseBytes, policy.projectLicenseSHA, "checker license"); err != nil {
		return Result{}, err
	}

	toolchainRaw, err := readStableFile(toolchainPath, maxToolchainBytes)
	if err != nil {
		return Result{}, fmt.Errorf("read toolchain archive: %w", err)
	}
	if err := requireIdentity(toolchainRaw, policy.toolchainBytes, policy.toolchainSHA256, "toolchain archive"); err != nil {
		return Result{}, err
	}
	goLicense, goPatents, err := extractGoLegalMaterials(toolchainRaw)
	if err != nil {
		return Result{}, err
	}
	if err := requireIdentity(goLicense, policy.goLicenseBytes, policy.goLicenseSHA256, "Go license"); err != nil {
		return Result{}, err
	}
	if err := requireIdentity(goPatents, policy.goPatentsBytes, policy.goPatentsSHA256, "Go patents"); err != nil {
		return Result{}, err
	}

	materials := legalMaterials{
		goLicense: goLicense, goPatents: goPatents, projectLicense: projectLicense,
	}
	acceptanceRaw, err := encodeAcceptance(snapshot.Digest, config.Version, candidate, materials, policy)
	if err != nil {
		return Result{}, err
	}
	if err := verifySourceSnapshot(sourceRoot, snapshot.Digest); err != nil {
		return Result{}, fmt.Errorf("source identity before distribution materialization: %w", err)
	}
	if err := requireUnchanged(candidatePath, candidateRaw, maxCandidateBytes); err != nil {
		return Result{}, fmt.Errorf("candidate archive changed before materialization: %w", err)
	}
	if err := requireUnchanged(toolchainPath, toolchainRaw, maxToolchainBytes); err != nil {
		return Result{}, fmt.Errorf("toolchain archive changed before materialization: %w", err)
	}

	if err := materialize(outputRoot, candidateRaw, acceptanceRaw, materials); err != nil {
		return Result{}, err
	}
	keepOutput := false
	defer func() {
		if !keepOutput {
			_ = os.RemoveAll(outputRoot)
		}
	}()
	if err := verifyMaterialized(outputRoot, candidateRaw, acceptanceRaw, materials); err != nil {
		return Result{}, err
	}
	if err := verifySourceSnapshot(sourceRoot, snapshot.Digest); err != nil {
		return Result{}, fmt.Errorf("source identity after distribution materialization: %w", err)
	}
	if err := requireUnchanged(candidatePath, candidateRaw, maxCandidateBytes); err != nil {
		return Result{}, fmt.Errorf("candidate archive changed after materialization: %w", err)
	}
	if err := requireUnchanged(toolchainPath, toolchainRaw, maxToolchainBytes); err != nil {
		return Result{}, fmt.Errorf("toolchain archive changed after materialization: %w", err)
	}
	keepOutput = true
	acceptanceDigest := sha256.Sum256(acceptanceRaw)
	return Result{
		AcceptanceBytes:  int64(len(acceptanceRaw)),
		AcceptancePath:   filepath.Join(outputRoot, AcceptanceName),
		AcceptanceSHA256: fmt.Sprintf("sha256:%x", acceptanceDigest),
		CandidateBytes:   candidate.ArchiveBytes, CandidateSHA256: candidate.ArchiveSHA256,
		OutputRoot: outputRoot, Source: snapshot.Digest, Version: config.Version,
	}, nil
}

func encodeAcceptance(source, version string, candidate payloadarchive.Result, materials legalMaterials, policy materialPolicy) ([]byte, error) {
	value := acceptanceDocument{
		Acceptance: acceptanceDecision{
			AcceptedScope: append([]string(nil), acceptedScope...),
			Decision:      "accepted-for-controlled-durable-publication-candidate",
			ExcludedScope: append([]string(nil), excludedScope...),
		},
		Candidate: candidateBinding{
			Archive: fileBinding{
				ByteLength: strconv.FormatInt(candidate.ArchiveBytes, 10),
				Filename:   CandidateName, RawSHA256: candidate.ArchiveSHA256,
			},
			RetentionManifest: fileBinding{
				ByteLength: strconv.FormatInt(candidate.ManifestBytes, 10),
				Filename:   payloadarchive.ManifestName, RawSHA256: candidate.ManifestSHA256,
			},
		},
		Format: Format, FormatVersion: FormatVersion,
		Identity: identity{
			Source:    source,
			Target:    target{GOARCH: "arm64", GOARM64: "v8.0", GOOS: "darwin", MachO: "64-bit-arm64-executable"},
			Toolchain: ToolchainVersion, Version: version,
		},
		Licenses: []licenseRecord{
			licenseBinding(materials.goLicense, "BSD-3-Clause", GoLicenseName, "license"),
			licenseBinding(materials.goPatents, "PatentGrant-Go", GoPatentsName, "patent-grant"),
			licenseBinding(materials.projectLicense, "Apache-2.0", CheckerLicenseName, "license"),
		},
		ToolchainPayload: fileBinding{
			ByteLength: strconv.FormatInt(policy.toolchainBytes, 10),
			Filename:   ToolchainArchiveName, RawSHA256: policy.toolchainSHA256,
		},
	}
	return json.Marshal(value)
}

func licenseBinding(raw []byte, classification, path, role string) licenseRecord {
	digest := sha256.Sum256(raw)
	return licenseRecord{
		ByteLength: strconv.Itoa(len(raw)), Classification: classification,
		Path: path, RawSHA256: fmt.Sprintf("sha256:%x", digest), Role: role,
	}
}

func extractGoLegalMaterials(raw []byte) ([]byte, []byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, nil, fmt.Errorf("open Go toolchain gzip: %w", err)
	}
	tarReader := tar.NewReader(gzipReader)
	wanted := map[string]*[]byte{"go/LICENSE": nil, "go/PATENTS": nil}
	var license, patents []byte
	wanted["go/LICENSE"] = &license
	wanted["go/PATENTS"] = &patents
	seen := make(map[string]bool, len(wanted))
	for {
		header, nextErr := tarReader.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			gzipReader.Close()
			return nil, nil, fmt.Errorf("read Go toolchain tar: %w", nextErr)
		}
		target, ok := wanted[header.Name]
		if !ok {
			continue
		}
		if seen[header.Name] || header.Typeflag != tar.TypeReg || header.Size < 0 || header.Size > maxLegalBytes {
			gzipReader.Close()
			return nil, nil, fmt.Errorf("Go toolchain legal member profile mismatch: %s", header.Name)
		}
		memberRaw, readErr := io.ReadAll(io.LimitReader(tarReader, header.Size+1))
		if readErr != nil || int64(len(memberRaw)) != header.Size {
			gzipReader.Close()
			return nil, nil, fmt.Errorf("Go toolchain legal member bytes mismatch: %s", header.Name)
		}
		*target = memberRaw
		seen[header.Name] = true
	}
	if err := gzipReader.Close(); err != nil {
		return nil, nil, fmt.Errorf("close Go toolchain gzip: %w", err)
	}
	if !seen["go/LICENSE"] || !seen["go/PATENTS"] {
		return nil, nil, fmt.Errorf("Go toolchain legal member set is incomplete")
	}
	return license, patents, nil
}

func materialize(root string, candidate, acceptance []byte, materials legalMaterials) (err error) {
	if err := os.Mkdir(root, 0o755); err != nil {
		return fmt.Errorf("create distribution root: %w", err)
	}
	keepRoot := false
	defer func() {
		if !keepRoot {
			_ = os.RemoveAll(root)
		}
	}()
	if err := os.Mkdir(filepath.Join(root, "licenses"), 0o755); err != nil {
		return fmt.Errorf("create licenses directory: %w", err)
	}
	if err := os.Mkdir(filepath.Join(root, "licenses", "go"), 0o755); err != nil {
		return fmt.Errorf("create Go licenses directory: %w", err)
	}
	if err := os.Mkdir(filepath.Join(root, "licenses", "radishaxiom-checker"), 0o755); err != nil {
		return fmt.Errorf("create checker licenses directory: %w", err)
	}
	files := []struct {
		name string
		raw  []byte
	}{
		{name: CandidateName, raw: candidate},
		{name: AcceptanceName, raw: acceptance},
		{name: GoLicenseName, raw: materials.goLicense},
		{name: GoPatentsName, raw: materials.goPatents},
		{name: CheckerLicenseName, raw: materials.projectLicense},
	}
	for _, file := range files {
		if err := writeExclusive(filepath.Join(root, filepath.FromSlash(file.name)), file.raw, 0o644); err != nil {
			return fmt.Errorf("write distribution member %s: %w", file.name, err)
		}
	}
	keepRoot = true
	return nil
}

func verifyMaterialized(root string, candidate, acceptance []byte, materials legalMaterials) error {
	expected := map[string][]byte{
		CandidateName: candidate, AcceptanceName: acceptance,
		GoLicenseName: materials.goLicense, GoPatentsName: materials.goPatents,
		CheckerLicenseName: materials.projectLicense,
	}
	seen := make(map[string]bool, len(expected))
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		relative = filepath.ToSlash(relative)
		if entry.IsDir() {
			if relative != "licenses" && relative != "licenses/go" && relative != "licenses/radishaxiom-checker" {
				return fmt.Errorf("unexpected distribution directory: %s", relative)
			}
			info, err := entry.Info()
			if err != nil || info.Mode().Perm() != 0o755 {
				return fmt.Errorf("distribution directory mode mismatch: %s", relative)
			}
			return nil
		}
		expectedRaw, ok := expected[relative]
		if !ok {
			return fmt.Errorf("unexpected distribution member: %s", relative)
		}
		actual, err := readStableFile(path, maxCandidateBytes)
		if err != nil || !bytes.Equal(actual, expectedRaw) {
			return fmt.Errorf("materialized distribution member mismatch: %s", relative)
		}
		seen[relative] = true
		return nil
	})
	if err != nil {
		return fmt.Errorf("verify distribution root: %w", err)
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("materialized distribution member set is incomplete")
	}
	return nil
}

func requireIdentity(raw []byte, expectedBytes int64, expectedSHA256, label string) error {
	digest := sha256.Sum256(raw)
	if int64(len(raw)) != expectedBytes || fmt.Sprintf("sha256:%x", digest) != expectedSHA256 {
		return fmt.Errorf("%s identity mismatch", label)
	}
	return nil
}

func requireUnchanged(path string, expected []byte, limit int64) error {
	actual, err := readStableFile(path, limit)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, expected) {
		return fmt.Errorf("file bytes changed")
	}
	return nil
}

func verifySourceSnapshot(root, expected string) error {
	snapshot, err := sourceidentity.Verify(root)
	if err != nil {
		return err
	}
	if snapshot.Digest != expected {
		return fmt.Errorf("checker source identity changed during distribution acceptance")
	}
	return nil
}

func canonicalDirectory(path string) (string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", fmt.Errorf("path must be absolute and clean")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || resolved != path {
		return "", fmt.Errorf("path must be an existing canonical realpath")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("path must be a directory")
	}
	return path, nil
}

func canonicalNewDirectory(path string) (string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Base(path) == "." {
		return "", fmt.Errorf("path must be absolute and clean")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return "", fmt.Errorf("output directory must not exist")
	}
	parent, err := canonicalDirectory(filepath.Dir(path))
	if err != nil || parent != filepath.Dir(path) {
		return "", fmt.Errorf("output parent must be a canonical directory")
	}
	return path, nil
}

func canonicalRegularFile(path string, mode os.FileMode) (string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", fmt.Errorf("path must be absolute and clean")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || resolved != path {
		return "", fmt.Errorf("path must be an existing canonical realpath")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != mode ||
		info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return "", fmt.Errorf("path must be a plain %04o regular file", mode)
	}
	return path, nil
}

func readStableFile(path string, limit int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil || before.Size() < 0 || before.Size() > limit {
		return nil, fmt.Errorf("file size exceeds limit")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	written, copyErr := io.Copy(&output, io.LimitReader(file, limit+1))
	opened, statErr := file.Stat()
	closeErr := file.Close()
	after, afterErr := os.Lstat(path)
	if copyErr != nil || statErr != nil || closeErr != nil || afterErr != nil || written > limit ||
		!os.SameFile(before, opened) || !os.SameFile(before, after) || before.Mode() != after.Mode() ||
		before.Size() != after.Size() || written != before.Size() {
		return nil, fmt.Errorf("file changed while reading")
	}
	return output.Bytes(), nil
}

func writeExclusive(path string, raw []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if err := file.Chmod(mode); err != nil {
		file.Close()
		_ = os.Remove(path)
		return err
	}
	written, writeErr := file.Write(raw)
	closeErr := file.Close()
	if writeErr != nil || written != len(raw) || closeErr != nil {
		_ = os.Remove(path)
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
		return io.ErrShortWrite
	}
	return nil
}

func validateVersion(value string) error {
	if value == "" || value == "latest" || len(value) > 64 {
		return fmt.Errorf("version must be an exact non-latest ASCII identity")
	}
	for index, character := range []byte(value) {
		allowed := (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || strings.ContainsRune("._+-", rune(character))
		if !allowed || (index == 0 && !((character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9'))) {
			return fmt.Errorf("version must be an exact non-latest ASCII identity")
		}
	}
	return nil
}
