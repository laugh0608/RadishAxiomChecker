package payloadarchive

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
	"sort"
	"strconv"
	"strings"
	"time"

	"radishaxiom.dev/independent-checker-go/internal/sourceidentity"
)

const (
	ArchiveFormat    = "ustar"
	Format           = "radishaxiom-checker-payload-retention-manifest"
	FormatVersion    = "0.1"
	ExecutableName   = "radishaxiom-independent-checker-go"
	ProvenanceName   = "checker-build-provenance-v0.1.jcs"
	AcceptanceName   = "checker-payload-acceptance-v0.1.jcs"
	ManifestName     = "checker-payload-retention-manifest-v0.1.jcs"
	ToolchainVersion = "go1.26.7"
	maxArchiveBytes  = int64(16 * 1024 * 1024)
)

var memberOrder = []string{
	ProvenanceName,
	AcceptanceName,
	ManifestName,
	ExecutableName,
}

type PackConfig struct {
	BuildRoot  string
	OutputFile string
	SourceRoot string
	Version    string
}

type Result struct {
	ArchiveBytes   int64
	ArchivePath    string
	ArchiveSHA256  string
	ManifestBytes  int64
	ManifestSHA256 string
	Source         string
	Version        string
}

type manifest struct {
	Contents      []contentRecord `json:"contents"`
	Format        string          `json:"format"`
	FormatVersion string          `json:"format_version"`
	Identity      identity        `json:"identity"`
	Packaging     packaging       `json:"packaging"`
}

type contentRecord struct {
	ByteLength string `json:"byte_length"`
	Mode       string `json:"mode"`
	Path       string `json:"path"`
	RawSHA256  string `json:"raw_sha256"`
	Role       string `json:"role"`
}

type identity struct {
	Implementation string `json:"implementation"`
	Source         string `json:"source"`
	Target         target `json:"target"`
	Toolchain      string `json:"toolchain"`
	Version        string `json:"version"`
}

type target struct {
	ExecutableFormat string `json:"executable_format"`
	GOARCH           string `json:"goarch"`
	GOARM64          string `json:"goarm64"`
	GOOS             string `json:"goos"`
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

type member struct {
	mode int64
	name string
	raw  []byte
}

// Pack creates one deterministic, uncompressed USTAR archive containing the
// accepted build output and a canonical retention manifest. It does not
// execute, accept, publish, or register the payload.
func Pack(config PackConfig) (Result, error) {
	sourceRoot, err := canonicalDirectory(config.SourceRoot, false)
	if err != nil {
		return Result{}, fmt.Errorf("source root: %w", err)
	}
	buildRoot, err := canonicalDirectory(config.BuildRoot, false)
	if err != nil {
		return Result{}, fmt.Errorf("build root: %w", err)
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
	inputs, err := readBuildRoot(buildRoot)
	if err != nil {
		return Result{}, err
	}
	manifestRaw, err := encodeManifest(inputs, snapshot.Digest, config.Version)
	if err != nil {
		return Result{}, err
	}
	archiveMembers := append([]member{}, inputs...)
	archiveMembers = append(archiveMembers, member{mode: 0o644, name: ManifestName, raw: manifestRaw})
	sort.Slice(archiveMembers, func(i, j int) bool { return archiveMembers[i].name < archiveMembers[j].name })
	archiveRaw, err := encodeArchive(archiveMembers)
	if err != nil {
		return Result{}, err
	}
	if int64(len(archiveRaw)) > maxArchiveBytes {
		return Result{}, fmt.Errorf("payload archive exceeds byte limit")
	}
	if err := verifySourceSnapshot(sourceRoot, snapshot.Digest); err != nil {
		return Result{}, fmt.Errorf("source identity before archive materialization: %w", err)
	}
	if err := writeExclusive(outputFile, archiveRaw, 0o644); err != nil {
		return Result{}, fmt.Errorf("write payload archive: %w", err)
	}
	verified, err := Verify(outputFile)
	if err != nil {
		_ = os.Remove(outputFile)
		return Result{}, fmt.Errorf("verify materialized payload archive: %w", err)
	}
	if verified.Source != snapshot.Digest || verified.Version != config.Version {
		_ = os.Remove(outputFile)
		return Result{}, fmt.Errorf("materialized payload archive identity mismatch")
	}
	if err := verifySourceSnapshot(sourceRoot, snapshot.Digest); err != nil {
		_ = os.Remove(outputFile)
		return Result{}, fmt.Errorf("source identity after archive materialization: %w", err)
	}
	return verified, nil
}

func verifySourceSnapshot(root, expected string) error {
	snapshot, err := sourceidentity.Verify(root)
	if err != nil {
		return err
	}
	if snapshot.Digest != expected {
		return fmt.Errorf("checker source identity changed during payload packaging")
	}
	return nil
}

// Verify strictly parses and byte-reconstructs one payload archive. Success
// confirms deterministic packaging and inner raw identities, not acceptance,
// publication, installation, or runtime eligibility.
func Verify(archivePath string) (Result, error) {
	canonical, err := canonicalRegularFile(archivePath, 0o644)
	if err != nil {
		return Result{}, fmt.Errorf("payload archive: %w", err)
	}
	raw, err := readStableFile(canonical, maxArchiveBytes)
	if err != nil {
		return Result{}, fmt.Errorf("read payload archive: %w", err)
	}
	result, err := VerifyBytes(raw)
	if err != nil {
		return Result{}, err
	}
	result.ArchivePath = canonical
	return result, nil
}

// VerifyBytes applies the same closed USTAR and manifest verification to
// already retained bytes. It does not materialize a temporary file.
func VerifyBytes(raw []byte) (Result, error) {
	if int64(len(raw)) > maxArchiveBytes {
		return Result{}, fmt.Errorf("payload archive exceeds byte limit")
	}
	members, err := decodeArchive(raw)
	if err != nil {
		return Result{}, err
	}
	manifestMember := members[ManifestName]
	parsed, err := decodeManifest(manifestMember.raw)
	if err != nil {
		return Result{}, err
	}
	if err := validateManifest(parsed, members); err != nil {
		return Result{}, err
	}
	rebuiltMembers := make([]member, 0, len(memberOrder))
	for _, name := range memberOrder {
		rebuiltMembers = append(rebuiltMembers, members[name])
	}
	rebuilt, err := encodeArchive(rebuiltMembers)
	if err != nil {
		return Result{}, err
	}
	if !bytes.Equal(raw, rebuilt) {
		return Result{}, fmt.Errorf("payload archive is not canonical USTAR")
	}
	archiveDigest := sha256.Sum256(raw)
	manifestDigest := sha256.Sum256(manifestMember.raw)
	return Result{
		ArchiveBytes:   int64(len(raw)),
		ArchiveSHA256:  fmt.Sprintf("sha256:%x", archiveDigest),
		ManifestBytes:  int64(len(manifestMember.raw)),
		ManifestSHA256: fmt.Sprintf("sha256:%x", manifestDigest),
		Source:         parsed.Identity.Source, Version: parsed.Identity.Version,
	}, nil
}

func readBuildRoot(root string) ([]member, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read build root: %w", err)
	}
	expected := map[string]int64{
		ExecutableName: 0o755,
		ProvenanceName: 0o644,
		AcceptanceName: 0o644,
	}
	if len(entries) != len(expected) {
		return nil, fmt.Errorf("accepted build root must contain exactly three files")
	}
	result := make([]member, 0, len(expected))
	for _, entry := range entries {
		mode, ok := expected[entry.Name()]
		if !ok {
			return nil, fmt.Errorf("unexpected accepted build member: %s", entry.Name())
		}
		path := filepath.Join(root, entry.Name())
		canonical, err := canonicalRegularFile(path, os.FileMode(mode))
		if err != nil {
			return nil, fmt.Errorf("accepted build member %s: %w", entry.Name(), err)
		}
		raw, err := readStableFile(canonical, maxArchiveBytes)
		if err != nil {
			return nil, fmt.Errorf("read accepted build member %s: %w", entry.Name(), err)
		}
		result = append(result, member{mode: mode, name: entry.Name(), raw: raw})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].name < result[j].name })
	return result, nil
}

func encodeManifest(inputs []member, source, version string) ([]byte, error) {
	roles := map[string]string{
		ExecutableName: "checker-artifact",
		ProvenanceName: "build-provenance",
		AcceptanceName: "payload-acceptance",
	}
	contents := make([]contentRecord, 0, len(inputs))
	for _, input := range inputs {
		digest := sha256.Sum256(input.raw)
		contents = append(contents, contentRecord{
			ByteLength: strconv.Itoa(len(input.raw)),
			Mode:       fmt.Sprintf("%04o", input.mode),
			Path:       input.name,
			RawSHA256:  fmt.Sprintf("sha256:%x", digest),
			Role:       roles[input.name],
		})
	}
	value := manifest{
		Contents: contents, Format: Format, FormatVersion: FormatVersion,
		Identity: identity{
			Implementation: "radishaxiom-independent-checker-go",
			Source:         source,
			Target: target{
				ExecutableFormat: "macho-64-arm64",
				GOARCH:           "arm64", GOARM64: "v8.0", GOOS: "darwin",
			},
			Toolchain: ToolchainVersion, Version: version,
		},
		Packaging: packaging{
			ArchiveFormat: ArchiveFormat,
			HeaderProfile: headerProfile{
				GID: "0", GName: "", MTime: "1970-01-01T00:00:00Z",
				Type: "regular", UID: "0", UName: "",
			},
			MemberOrder: append([]string(nil), memberOrder...),
		},
	}
	return json.Marshal(value)
}

func decodeManifest(raw []byte) (manifest, error) {
	var value manifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return manifest{}, fmt.Errorf("decode retention manifest: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return manifest{}, fmt.Errorf("retention manifest has trailing JSON")
	}
	reencoded, err := json.Marshal(value)
	if err != nil || !bytes.Equal(raw, reencoded) {
		return manifest{}, fmt.Errorf("retention manifest is not canonical JSON")
	}
	return value, nil
}

func validateManifest(value manifest, members map[string]member) error {
	if value.Format != Format || value.FormatVersion != FormatVersion {
		return fmt.Errorf("retention manifest format mismatch")
	}
	if value.Identity.Implementation != "radishaxiom-independent-checker-go" ||
		value.Identity.Toolchain != ToolchainVersion ||
		!validDigest(value.Identity.Source) || validateVersion(value.Identity.Version) != nil {
		return fmt.Errorf("retention manifest implementation identity mismatch")
	}
	if value.Identity.Target != (target{
		ExecutableFormat: "macho-64-arm64", GOARCH: "arm64", GOARM64: "v8.0", GOOS: "darwin",
	}) {
		return fmt.Errorf("retention manifest target mismatch")
	}
	expectedPackaging := packaging{
		ArchiveFormat: ArchiveFormat,
		HeaderProfile: headerProfile{
			GID: "0", GName: "", MTime: "1970-01-01T00:00:00Z",
			Type: "regular", UID: "0", UName: "",
		},
		MemberOrder: memberOrder,
	}
	if !equalPackaging(value.Packaging, expectedPackaging) {
		return fmt.Errorf("retention manifest packaging profile mismatch")
	}
	if len(value.Contents) != 3 {
		return fmt.Errorf("retention manifest must bind exactly three payload files")
	}
	expectedNames := []string{ProvenanceName, AcceptanceName, ExecutableName}
	roles := []string{"build-provenance", "payload-acceptance", "checker-artifact"}
	for index, record := range value.Contents {
		if record.Path != expectedNames[index] || record.Role != roles[index] {
			return fmt.Errorf("retention manifest content order or role mismatch")
		}
		bound := members[record.Path]
		digest := sha256.Sum256(bound.raw)
		if record.Mode != fmt.Sprintf("%04o", bound.mode) ||
			record.ByteLength != strconv.Itoa(len(bound.raw)) ||
			record.RawSHA256 != fmt.Sprintf("sha256:%x", digest) {
			return fmt.Errorf("retention manifest content identity mismatch: %s", record.Path)
		}
	}
	return nil
}

func equalPackaging(left, right packaging) bool {
	if left.ArchiveFormat != right.ArchiveFormat || left.HeaderProfile != right.HeaderProfile ||
		len(left.MemberOrder) != len(right.MemberOrder) {
		return false
	}
	for index := range left.MemberOrder {
		if left.MemberOrder[index] != right.MemberOrder[index] {
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
			return nil, fmt.Errorf("encode payload archive header: %w", err)
		}
		if _, err := writer.Write(item.raw); err != nil {
			return nil, fmt.Errorf("encode payload archive member: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close payload archive: %w", err)
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
			return nil, fmt.Errorf("decode payload archive: %w", err)
		}
		if index >= len(memberOrder) || header.Name != memberOrder[index] {
			return nil, fmt.Errorf("payload archive member order mismatch")
		}
		expectedMode := int64(0o644)
		if header.Name == ExecutableName {
			expectedMode = 0o755
		}
		if header.Format != tar.FormatUSTAR || header.Typeflag != tar.TypeReg ||
			header.Mode != expectedMode || header.Uid != 0 || header.Gid != 0 ||
			header.Uname != "" || header.Gname != "" || !header.ModTime.Equal(time.Unix(0, 0).UTC()) ||
			!header.AccessTime.IsZero() || !header.ChangeTime.IsZero() || header.Linkname != "" ||
			header.Devmajor != 0 || header.Devminor != 0 || len(header.PAXRecords) != 0 || len(header.Xattrs) != 0 {
			return nil, fmt.Errorf("payload archive header profile mismatch: %s", header.Name)
		}
		if header.Size < 0 || header.Size > maxArchiveBytes {
			return nil, fmt.Errorf("payload archive member size is invalid")
		}
		memberRaw, err := io.ReadAll(io.LimitReader(reader, header.Size+1))
		if err != nil || int64(len(memberRaw)) != header.Size {
			return nil, fmt.Errorf("payload archive member bytes mismatch: %s", header.Name)
		}
		result[header.Name] = member{mode: header.Mode, name: header.Name, raw: memberRaw}
		index++
	}
	if index != len(memberOrder) {
		return nil, fmt.Errorf("payload archive member set mismatch")
	}
	return result, nil
}

func canonicalDirectory(path string, requireEmpty bool) (string, error) {
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
	if requireEmpty {
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) != 0 {
			return "", fmt.Errorf("directory must be empty")
		}
	}
	return path, nil
}

func canonicalNewFile(path string) (string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", fmt.Errorf("path must be absolute and clean")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return "", fmt.Errorf("output must not exist")
	}
	parent, err := canonicalDirectory(filepath.Dir(path), false)
	if err != nil || parent != filepath.Dir(path) || filepath.Base(path) == "." {
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
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != mode {
		return "", fmt.Errorf("path must be a %04o regular file", mode)
	}
	return path, nil
}

func readStableFile(path string, limit int64) ([]byte, error) {
	before, err := os.Stat(path)
	if err != nil || before.Size() < 0 || before.Size() > limit {
		return nil, fmt.Errorf("file size exceeds limit")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) || before.Size() != after.Size() ||
		before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) || int64(len(raw)) != before.Size() {
		return nil, fmt.Errorf("file changed while reading")
	}
	return raw, nil
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
	if writeErr != nil || written != len(raw) {
		_ = os.Remove(path)
		if writeErr != nil {
			return writeErr
		}
		return io.ErrShortWrite
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return closeErr
	}
	return nil
}

func validateVersion(value string) error {
	if value == "" || value == "latest" || len(value) > 64 {
		return fmt.Errorf("version must be an exact non-latest ASCII identity")
	}
	for _, character := range []byte(value) {
		if character < 0x21 || character > 0x7e {
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
