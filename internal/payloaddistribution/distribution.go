package payloaddistribution

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"radishaxiom.dev/independent-checker-go/internal/payloadarchive"
	"radishaxiom.dev/independent-checker-go/internal/sourceidentity"
)

const (
	AcceptanceFormat     = "radishaxiom-checker-payload-distribution-acceptance"
	AcceptanceName       = "checker-payload-distribution-acceptance-v0.1.jcs"
	ArchiveFormat        = "ustar"
	CandidateName        = "checker-payload-candidate.tar"
	CheckerLicenseName   = "licenses/radishaxiom-checker/LICENSE"
	Format               = "radishaxiom-checker-runtime-distribution-manifest"
	FormatVersion        = "0.1"
	GoLicenseName        = "licenses/go/LICENSE"
	GoPatentsName        = "licenses/go/PATENTS"
	ManifestName         = "checker-payload-distribution-manifest-v0.1.jcs"
	ToolchainArchiveName = "go1.26.7.darwin-arm64.tar.gz"
	ToolchainVersion     = "go1.26.7"
	maxArchiveBytes      = int64(32 * 1024 * 1024)
	maxMemberBytes       = int64(16 * 1024 * 1024)
)

var memberOrder = []string{
	CandidateName,
	AcceptanceName,
	ManifestName,
	GoLicenseName,
	GoPatentsName,
	CheckerLicenseName,
}

var contentOrder = []string{
	CandidateName,
	AcceptanceName,
	GoLicenseName,
	GoPatentsName,
	CheckerLicenseName,
}

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

type PackConfig struct {
	DistributionRoot string
	OutputFile       string
	SourceRoot       string
	Version          string
}

type Result struct {
	AcceptanceBytes  int64
	AcceptanceSHA256 string
	ArchiveBytes     int64
	ArchivePath      string
	ArchiveSHA256    string
	CandidateBytes   int64
	CandidateSHA256  string
	ManifestBytes    int64
	ManifestSHA256   string
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

type manifest struct {
	Contents      []contentRecord  `json:"contents"`
	Format        string           `json:"format"`
	FormatVersion string           `json:"format_version"`
	Identity      manifestIdentity `json:"identity"`
	Packaging     packaging        `json:"packaging"`
}

type contentRecord struct {
	ByteLength string `json:"byte_length"`
	Mode       string `json:"mode"`
	Path       string `json:"path"`
	RawSHA256  string `json:"raw_sha256"`
	Role       string `json:"role"`
}

type manifestIdentity struct {
	Implementation string `json:"implementation"`
	Source         string `json:"source"`
	Target         target `json:"target"`
	Toolchain      string `json:"toolchain"`
	Version        string `json:"version"`
}

type target struct {
	GOARCH  string `json:"goarch"`
	GOARM64 string `json:"goarm64"`
	GOOS    string `json:"goos"`
	MachO   string `json:"macho"`
}

type packaging struct {
	ArchiveFormat string        `json:"archive_format"`
	HeaderProfile headerProfile `json:"header_profile"`
	MemberOrder   []string      `json:"member_order"`
}

type headerProfile struct {
	GID   string `json:"gid"`
	GName string `json:"gname"`
	MTime string `json:"mtime"`
	Type  string `json:"type"`
	UID   string `json:"uid"`
	UName string `json:"uname"`
}

type acceptanceDocument struct {
	Acceptance       acceptanceDecision `json:"acceptance"`
	Candidate        candidateBinding   `json:"candidate"`
	Format           string             `json:"format"`
	FormatVersion    string             `json:"format_version"`
	Identity         acceptanceIdentity `json:"identity"`
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

type acceptanceIdentity struct {
	Source    string `json:"source"`
	Target    target `json:"target"`
	Toolchain string `json:"toolchain"`
	Version   string `json:"version"`
}

type licenseRecord struct {
	ByteLength     string `json:"byte_length"`
	Classification string `json:"classification"`
	Path           string `json:"path"`
	RawSHA256      string `json:"raw_sha256"`
	Role           string `json:"role"`
}

type member struct {
	mode int64
	name string
	raw  []byte
}

// Pack creates one deterministic outer USTAR from an independently accepted
// distribution root. It does not publish, register, install, or activate it.
func Pack(config PackConfig) (Result, error) {
	return packWithPolicy(config, productionPolicy)
}

func packWithPolicy(config PackConfig, policy materialPolicy) (Result, error) {
	sourceRoot, err := canonicalDirectory(config.SourceRoot)
	if err != nil {
		return Result{}, fmt.Errorf("source root: %w", err)
	}
	distributionRoot, err := canonicalDirectory(config.DistributionRoot)
	if err != nil {
		return Result{}, fmt.Errorf("distribution root: %w", err)
	}
	outputFile, err := canonicalNewFile(config.OutputFile)
	if err != nil {
		return Result{}, fmt.Errorf("output file: %w", err)
	}
	if err := validateVersion(config.Version); err != nil {
		return Result{}, err
	}
	snapshot, err := sourceidentity.Verify(sourceRoot)
	if err != nil {
		return Result{}, fmt.Errorf("verify checker source identity: %w", err)
	}
	if filepath.Base(outputFile) != ArchiveFilename(snapshot.Digest, config.Version) {
		return Result{}, fmt.Errorf("distribution archive filename does not match source and version")
	}
	inputs, err := readDistributionRoot(distributionRoot)
	if err != nil {
		return Result{}, err
	}
	candidate, acceptance, err := validateAcceptedInputs(inputs, policy)
	if err != nil {
		return Result{}, err
	}
	if candidate.Source != snapshot.Digest || candidate.Version != config.Version ||
		acceptance.Identity.Source != snapshot.Digest || acceptance.Identity.Version != config.Version {
		return Result{}, fmt.Errorf("accepted distribution identity mismatch")
	}
	manifestRaw, err := encodeManifest(inputs, snapshot.Digest, config.Version)
	if err != nil {
		return Result{}, err
	}
	archiveMembers := make([]member, 0, len(memberOrder))
	for _, name := range memberOrder {
		if name == ManifestName {
			archiveMembers = append(archiveMembers, member{mode: 0o644, name: name, raw: manifestRaw})
			continue
		}
		archiveMembers = append(archiveMembers, inputs[name])
	}
	archiveRaw, err := encodeArchive(archiveMembers)
	if err != nil {
		return Result{}, err
	}
	if int64(len(archiveRaw)) > maxArchiveBytes {
		return Result{}, fmt.Errorf("distribution archive exceeds byte limit")
	}
	if err := verifySourceSnapshot(sourceRoot, snapshot.Digest); err != nil {
		return Result{}, fmt.Errorf("source identity before distribution materialization: %w", err)
	}
	if err := writeExclusive(outputFile, archiveRaw, 0o644); err != nil {
		return Result{}, fmt.Errorf("write distribution archive: %w", err)
	}
	materializedRaw, err := readStableFile(outputFile, maxArchiveBytes)
	if err != nil || !bytes.Equal(materializedRaw, archiveRaw) {
		_ = os.Remove(outputFile)
		return Result{}, fmt.Errorf("materialized distribution archive bytes mismatch")
	}
	verified, err := verifyBytesWithPolicy(materializedRaw, policy)
	if err != nil {
		_ = os.Remove(outputFile)
		return Result{}, fmt.Errorf("verify materialized distribution archive: %w", err)
	}
	verified.ArchivePath = outputFile
	if verified.Source != snapshot.Digest || verified.Version != config.Version {
		_ = os.Remove(outputFile)
		return Result{}, fmt.Errorf("materialized distribution identity mismatch")
	}
	if err := verifySourceSnapshot(sourceRoot, snapshot.Digest); err != nil {
		_ = os.Remove(outputFile)
		return Result{}, fmt.Errorf("source identity after distribution materialization: %w", err)
	}
	return verified, nil
}

func Verify(path string) (Result, error) {
	canonical, err := canonicalRegularFile(path, 0o644)
	if err != nil {
		return Result{}, fmt.Errorf("distribution archive: %w", err)
	}
	raw, err := readStableFile(canonical, maxArchiveBytes)
	if err != nil {
		return Result{}, fmt.Errorf("read distribution archive: %w", err)
	}
	result, err := verifyBytesWithPolicy(raw, productionPolicy)
	if err != nil {
		return Result{}, err
	}
	if filepath.Base(canonical) != ArchiveFilename(result.Source, result.Version) {
		return Result{}, fmt.Errorf("distribution archive filename does not match manifest identity")
	}
	result.ArchivePath = canonical
	return result, nil
}

func VerifyBytes(raw []byte) (Result, error) {
	return verifyBytesWithPolicy(raw, productionPolicy)
}

func verifyBytesWithPolicy(raw []byte, policy materialPolicy) (Result, error) {
	if int64(len(raw)) > maxArchiveBytes {
		return Result{}, fmt.Errorf("distribution archive exceeds byte limit")
	}
	members, err := decodeArchive(raw)
	if err != nil {
		return Result{}, err
	}
	parsedManifest, err := decodeManifest(members[ManifestName].raw)
	if err != nil {
		return Result{}, err
	}
	if err := validateManifest(parsedManifest, members); err != nil {
		return Result{}, err
	}
	candidate, acceptance, err := validateAcceptedInputs(members, policy)
	if err != nil {
		return Result{}, err
	}
	if candidate.Source != parsedManifest.Identity.Source || candidate.Version != parsedManifest.Identity.Version ||
		acceptance.Identity.Source != parsedManifest.Identity.Source || acceptance.Identity.Version != parsedManifest.Identity.Version {
		return Result{}, fmt.Errorf("distribution manifest and acceptance identity mismatch")
	}
	rebuilt := make([]member, 0, len(memberOrder))
	for _, name := range memberOrder {
		rebuilt = append(rebuilt, members[name])
	}
	rebuiltRaw, err := encodeArchive(rebuilt)
	if err != nil {
		return Result{}, err
	}
	if !bytes.Equal(raw, rebuiltRaw) {
		return Result{}, fmt.Errorf("distribution archive is not canonical USTAR")
	}
	archiveDigest := sha256.Sum256(raw)
	manifestRaw := members[ManifestName].raw
	manifestDigest := sha256.Sum256(manifestRaw)
	acceptanceRaw := members[AcceptanceName].raw
	acceptanceDigest := sha256.Sum256(acceptanceRaw)
	return Result{
		AcceptanceBytes: int64(len(acceptanceRaw)), AcceptanceSHA256: fmt.Sprintf("sha256:%x", acceptanceDigest),
		ArchiveBytes: int64(len(raw)), ArchiveSHA256: fmt.Sprintf("sha256:%x", archiveDigest),
		CandidateBytes: candidate.ArchiveBytes, CandidateSHA256: candidate.ArchiveSHA256,
		ManifestBytes: int64(len(manifestRaw)), ManifestSHA256: fmt.Sprintf("sha256:%x", manifestDigest),
		Source: parsedManifest.Identity.Source, Version: parsedManifest.Identity.Version,
	}, nil
}

func ArchiveFilename(source, version string) string {
	hexDigest := strings.TrimPrefix(source, "sha256:")
	return "radishaxiom-checker-go" + version + "-darwin-arm64-v8.0-sha256-" + hexDigest + ".distribution.tar"
}

func readDistributionRoot(root string) (map[string]member, error) {
	rootInfo, err := os.Lstat(root)
	if err != nil || rootInfo.Mode().Perm() != 0o755 {
		return nil, fmt.Errorf("distribution root mode mismatch")
	}
	expectedFiles := make(map[string]bool, len(contentOrder))
	for _, name := range contentOrder {
		expectedFiles[name] = true
	}
	expectedDirs := map[string]bool{
		"licenses": true, "licenses/go": true, "licenses/radishaxiom-checker": true,
	}
	result := make(map[string]member, len(contentOrder))
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
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
			if !expectedDirs[relative] {
				return fmt.Errorf("unexpected distribution directory: %s", relative)
			}
			info, err := entry.Info()
			if err != nil || info.Mode().Perm() != 0o755 {
				return fmt.Errorf("distribution directory mode mismatch: %s", relative)
			}
			return nil
		}
		if !expectedFiles[relative] {
			return fmt.Errorf("unexpected distribution member: %s", relative)
		}
		canonical, err := canonicalRegularFile(path, 0o644)
		if err != nil {
			return fmt.Errorf("distribution member %s: %w", relative, err)
		}
		raw, err := readStableFile(canonical, maxMemberBytes)
		if err != nil {
			return fmt.Errorf("read distribution member %s: %w", relative, err)
		}
		result[relative] = member{mode: 0o644, name: relative, raw: raw}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(result) != len(contentOrder) {
		return nil, fmt.Errorf("distribution root member set mismatch")
	}
	return result, nil
}

func validateAcceptedInputs(members map[string]member, policy materialPolicy) (payloadarchive.Result, acceptanceDocument, error) {
	candidateRaw, ok := members[CandidateName]
	if !ok {
		return payloadarchive.Result{}, acceptanceDocument{}, fmt.Errorf("distribution candidate is missing")
	}
	candidate, err := payloadarchive.VerifyBytes(candidateRaw.raw)
	if err != nil {
		return payloadarchive.Result{}, acceptanceDocument{}, fmt.Errorf("verify inner candidate: %w", err)
	}
	acceptanceRaw, ok := members[AcceptanceName]
	if !ok {
		return payloadarchive.Result{}, acceptanceDocument{}, fmt.Errorf("distribution acceptance is missing")
	}
	acceptance, err := decodeAcceptance(acceptanceRaw.raw)
	if err != nil {
		return payloadarchive.Result{}, acceptanceDocument{}, err
	}
	if err := validateAcceptance(acceptance, members, candidate, policy); err != nil {
		return payloadarchive.Result{}, acceptanceDocument{}, err
	}
	return candidate, acceptance, nil
}

func decodeAcceptance(raw []byte) (acceptanceDocument, error) {
	var value acceptanceDocument
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return acceptanceDocument{}, fmt.Errorf("decode distribution acceptance: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return acceptanceDocument{}, fmt.Errorf("distribution acceptance has trailing JSON")
	}
	reencoded, err := json.Marshal(value)
	if err != nil || !bytes.Equal(raw, reencoded) {
		return acceptanceDocument{}, fmt.Errorf("distribution acceptance is not canonical JSON")
	}
	return value, nil
}

func validateAcceptance(value acceptanceDocument, members map[string]member, candidate payloadarchive.Result, policy materialPolicy) error {
	if value.Format != AcceptanceFormat || value.FormatVersion != FormatVersion {
		return fmt.Errorf("distribution acceptance format mismatch")
	}
	if value.Acceptance.Decision != "accepted-for-controlled-durable-publication-candidate" ||
		!equalStrings(value.Acceptance.AcceptedScope, acceptedScope) ||
		!equalStrings(value.Acceptance.ExcludedScope, excludedScope) {
		return fmt.Errorf("distribution acceptance scope mismatch")
	}
	expectedTarget := target{GOARCH: "arm64", GOARM64: "v8.0", GOOS: "darwin", MachO: "64-bit-arm64-executable"}
	if !validDigest(value.Identity.Source) || validateVersion(value.Identity.Version) != nil ||
		value.Identity.Toolchain != ToolchainVersion || value.Identity.Target != expectedTarget {
		return fmt.Errorf("distribution acceptance identity mismatch")
	}
	if value.Candidate.Archive != (fileBinding{
		ByteLength: strconv.FormatInt(candidate.ArchiveBytes, 10), Filename: CandidateName,
		RawSHA256: candidate.ArchiveSHA256,
	}) || value.Candidate.RetentionManifest != (fileBinding{
		ByteLength: strconv.FormatInt(candidate.ManifestBytes, 10), Filename: payloadarchive.ManifestName,
		RawSHA256: candidate.ManifestSHA256,
	}) {
		return fmt.Errorf("distribution acceptance candidate binding mismatch")
	}
	if value.ToolchainPayload != (fileBinding{
		ByteLength: strconv.FormatInt(policy.toolchainBytes, 10), Filename: ToolchainArchiveName,
		RawSHA256: policy.toolchainSHA256,
	}) {
		return fmt.Errorf("distribution acceptance toolchain binding mismatch")
	}
	expectedLicenses := []struct {
		bytes          int64
		classification string
		path           string
		role           string
		sha256         string
	}{
		{policy.goLicenseBytes, "BSD-3-Clause", GoLicenseName, "license", policy.goLicenseSHA256},
		{policy.goPatentsBytes, "PatentGrant-Go", GoPatentsName, "patent-grant", policy.goPatentsSHA256},
		{policy.projectLicenseBytes, "Apache-2.0", CheckerLicenseName, "license", policy.projectLicenseSHA},
	}
	if len(value.Licenses) != len(expectedLicenses) {
		return fmt.Errorf("distribution acceptance license inventory count mismatch")
	}
	for index, expected := range expectedLicenses {
		record := value.Licenses[index]
		bound, ok := members[expected.path]
		if !ok {
			return fmt.Errorf("distribution legal member is missing: %s", expected.path)
		}
		digest := sha256.Sum256(bound.raw)
		if record != (licenseRecord{
			ByteLength: strconv.FormatInt(expected.bytes, 10), Classification: expected.classification,
			Path: expected.path, RawSHA256: expected.sha256, Role: expected.role,
		}) || int64(len(bound.raw)) != expected.bytes || fmt.Sprintf("sha256:%x", digest) != expected.sha256 {
			return fmt.Errorf("distribution legal member identity mismatch: %s", expected.path)
		}
	}
	return nil
}

func encodeManifest(inputs map[string]member, source, version string) ([]byte, error) {
	roles := map[string]string{
		CandidateName: "candidate-archive", AcceptanceName: "distribution-acceptance",
		GoLicenseName: "go-license", GoPatentsName: "go-patent-grant",
		CheckerLicenseName: "checker-license",
	}
	contents := make([]contentRecord, 0, len(contentOrder))
	for _, name := range contentOrder {
		input := inputs[name]
		digest := sha256.Sum256(input.raw)
		contents = append(contents, contentRecord{
			ByteLength: strconv.Itoa(len(input.raw)), Mode: "0644", Path: name,
			RawSHA256: fmt.Sprintf("sha256:%x", digest), Role: roles[name],
		})
	}
	value := manifest{
		Contents: contents, Format: Format, FormatVersion: FormatVersion,
		Identity: manifestIdentity{
			Implementation: "radishaxiom-independent-checker-go", Source: source,
			Target:    target{GOARCH: "arm64", GOARM64: "v8.0", GOOS: "darwin", MachO: "64-bit-arm64-executable"},
			Toolchain: ToolchainVersion, Version: version,
		},
		Packaging: packaging{
			ArchiveFormat: ArchiveFormat,
			HeaderProfile: headerProfile{GID: "0", GName: "", MTime: "1970-01-01T00:00:00Z", Type: "regular", UID: "0", UName: ""},
			MemberOrder:   append([]string(nil), memberOrder...),
		},
	}
	return json.Marshal(value)
}

func decodeManifest(raw []byte) (manifest, error) {
	var value manifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return manifest{}, fmt.Errorf("decode distribution manifest: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return manifest{}, fmt.Errorf("distribution manifest has trailing JSON")
	}
	reencoded, err := json.Marshal(value)
	if err != nil || !bytes.Equal(raw, reencoded) {
		return manifest{}, fmt.Errorf("distribution manifest is not canonical JSON")
	}
	return value, nil
}

func validateManifest(value manifest, members map[string]member) error {
	if value.Format != Format || value.FormatVersion != FormatVersion {
		return fmt.Errorf("distribution manifest format mismatch")
	}
	expectedTarget := target{GOARCH: "arm64", GOARM64: "v8.0", GOOS: "darwin", MachO: "64-bit-arm64-executable"}
	if value.Identity.Implementation != "radishaxiom-independent-checker-go" ||
		!validDigest(value.Identity.Source) || value.Identity.Toolchain != ToolchainVersion ||
		validateVersion(value.Identity.Version) != nil || value.Identity.Target != expectedTarget {
		return fmt.Errorf("distribution manifest identity mismatch")
	}
	expectedPackaging := packaging{
		ArchiveFormat: ArchiveFormat,
		HeaderProfile: headerProfile{GID: "0", GName: "", MTime: "1970-01-01T00:00:00Z", Type: "regular", UID: "0", UName: ""},
		MemberOrder:   memberOrder,
	}
	if !equalPackaging(value.Packaging, expectedPackaging) {
		return fmt.Errorf("distribution manifest packaging mismatch")
	}
	roles := []string{"candidate-archive", "distribution-acceptance", "go-license", "go-patent-grant", "checker-license"}
	if len(value.Contents) != len(contentOrder) {
		return fmt.Errorf("distribution manifest content count mismatch")
	}
	for index, name := range contentOrder {
		record := value.Contents[index]
		bound, ok := members[name]
		if !ok {
			return fmt.Errorf("distribution manifest member is missing: %s", name)
		}
		digest := sha256.Sum256(bound.raw)
		if record != (contentRecord{
			ByteLength: strconv.Itoa(len(bound.raw)), Mode: "0644", Path: name,
			RawSHA256: fmt.Sprintf("sha256:%x", digest), Role: roles[index],
		}) {
			return fmt.Errorf("distribution manifest content identity mismatch: %s", name)
		}
	}
	return nil
}

func equalPackaging(left, right packaging) bool {
	return left.ArchiveFormat == right.ArchiveFormat && left.HeaderProfile == right.HeaderProfile &&
		equalStrings(left.MemberOrder, right.MemberOrder)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func encodeArchive(members []member) ([]byte, error) {
	var output bytes.Buffer
	writer := tar.NewWriter(&output)
	for _, item := range members {
		header := &tar.Header{
			Name: item.name, Mode: item.mode, Size: int64(len(item.raw)),
			ModTime: time.Unix(0, 0).UTC(), Typeflag: tar.TypeReg,
			Uid: 0, Gid: 0, Uname: "", Gname: "", Format: tar.FormatUSTAR,
		}
		if err := writer.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("encode distribution archive header: %w", err)
		}
		if _, err := writer.Write(item.raw); err != nil {
			return nil, fmt.Errorf("encode distribution archive member: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close distribution archive: %w", err)
	}
	return output.Bytes(), nil
}

func decodeArchive(raw []byte) (map[string]member, error) {
	reader := tar.NewReader(bytes.NewReader(raw))
	result := make(map[string]member, len(memberOrder))
	index := 0
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode distribution archive: %w", err)
		}
		if index >= len(memberOrder) || header.Name != memberOrder[index] {
			return nil, fmt.Errorf("distribution archive member order mismatch")
		}
		if header.Format != tar.FormatUSTAR || header.Typeflag != tar.TypeReg || header.Mode != 0o644 ||
			header.Uid != 0 || header.Gid != 0 || header.Uname != "" || header.Gname != "" ||
			!header.ModTime.Equal(time.Unix(0, 0).UTC()) || !header.AccessTime.IsZero() ||
			!header.ChangeTime.IsZero() || header.Linkname != "" || header.Devmajor != 0 || header.Devminor != 0 ||
			len(header.PAXRecords) != 0 || len(header.Xattrs) != 0 {
			return nil, fmt.Errorf("distribution archive header profile mismatch: %s", header.Name)
		}
		if header.Size < 0 || header.Size > maxMemberBytes {
			return nil, fmt.Errorf("distribution archive member size is invalid")
		}
		memberRaw, err := io.ReadAll(io.LimitReader(reader, header.Size+1))
		if err != nil || int64(len(memberRaw)) != header.Size {
			return nil, fmt.Errorf("distribution archive member bytes mismatch: %s", header.Name)
		}
		result[header.Name] = member{mode: header.Mode, name: header.Name, raw: memberRaw}
		index++
	}
	if index != len(memberOrder) {
		return nil, fmt.Errorf("distribution archive member set mismatch")
	}
	return result, nil
}

func verifySourceSnapshot(root, expected string) error {
	snapshot, err := sourceidentity.Verify(root)
	if err != nil {
		return err
	}
	if snapshot.Digest != expected {
		return fmt.Errorf("checker source identity changed during distribution packaging")
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

func canonicalNewFile(path string) (string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Base(path) == "." {
		return "", fmt.Errorf("path must be absolute and clean")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return "", fmt.Errorf("output must not exist")
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

func validDigest(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+64 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil && value == strings.ToLower(value)
}
