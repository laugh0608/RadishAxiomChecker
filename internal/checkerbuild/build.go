package checkerbuild

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"radishaxiom.dev/independent-checker-go/internal/sourceidentity"
)

const (
	ToolchainArchiveBytes  = int64(64_772_572)
	ToolchainArchiveSHA256 = "020a1e8224811be75163e920bc77e0926a1390a6aeea19bdcf23f74b9d749f6d"
	ToolchainArchiveName   = "go1.26.7.darwin-arm64.tar.gz"
	ToolchainVersion       = "go1.26.7"
	ExecutableName         = "radishaxiom-independent-checker-go"
	ProvenanceName         = "checker-build-provenance-v0.1.jcs"

	archiveMemberCount  = 16_701
	archiveRegularBytes = int64(228_748_173)
	goVersionFile       = "go1.26.7\ntime 2026-08-18T21:44:21Z\n"
)

type Config struct {
	SourceRoot       string
	ToolchainArchive string
	OutputRoot       string
	Version          string
}

type Result struct {
	ArtifactPath   string
	ArtifactBytes  int64
	ArtifactSHA256 string
	ProvenancePath string
	Source         string
	Version        string
}

// Build performs two isolated builds from one verified source snapshot and
// one exact accepted Go payload. Nothing is emitted unless both binaries are
// byte-identical.
func Build(config Config) (Result, error) {
	sourceRoot, err := canonicalDirectory(config.SourceRoot, false)
	if err != nil {
		return Result{}, fmt.Errorf("source root: %w", err)
	}
	outputRoot, err := canonicalDirectory(config.OutputRoot, true)
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
	archive, err := canonicalRegularFile(config.ToolchainArchive)
	if err != nil {
		return Result{}, fmt.Errorf("toolchain archive: %w", err)
	}
	archiveBytes, archiveSHA256, err := hashStableFile(archive)
	if err != nil {
		return Result{}, fmt.Errorf("hash toolchain archive: %w", err)
	}
	if archiveBytes != ToolchainArchiveBytes || archiveSHA256 != ToolchainArchiveSHA256 {
		return Result{}, fmt.Errorf("toolchain archive identity mismatch")
	}

	workspace, err := os.MkdirTemp("", "radishaxiom-checker-build-")
	if err != nil {
		return Result{}, fmt.Errorf("create isolated build workspace: %w", err)
	}
	defer os.RemoveAll(workspace)
	workspace, err = filepath.EvalSymlinks(workspace)
	if err != nil {
		return Result{}, fmt.Errorf("resolve isolated build workspace: %w", err)
	}
	toolchainRoot := filepath.Join(workspace, "toolchain", "go")
	if err := extractToolchain(archive, filepath.Join(workspace, "toolchain")); err != nil {
		return Result{}, err
	}
	if err := verifyToolchain(toolchainRoot, workspace); err != nil {
		return Result{}, err
	}

	artifacts := make([]string, 0, 2)
	for _, label := range []string{"a", "b"} {
		artifact, err := buildOnce(
			filepath.Join(workspace, "build-"+label), toolchainRoot,
			sourceRoot, snapshot.Digest, config.Version,
		)
		if err != nil {
			return Result{}, fmt.Errorf("isolated build %s: %w", label, err)
		}
		if err := verifySourceSnapshot(sourceRoot, snapshot.Digest); err != nil {
			return Result{}, fmt.Errorf("source identity after isolated build %s: %w", label, err)
		}
		artifacts = append(artifacts, artifact)
	}
	identical, err := filesEqual(artifacts[0], artifacts[1])
	if err != nil {
		return Result{}, fmt.Errorf("compare isolated build outputs: %w", err)
	}
	if !identical {
		return Result{}, fmt.Errorf("isolated build outputs are not byte-identical")
	}
	artifactBytes, artifactSHA256, err := hashStableFile(artifacts[0])
	if err != nil {
		return Result{}, fmt.Errorf("hash checker artifact: %w", err)
	}
	provenance := encodeProvenance(
		artifactBytes, artifactSHA256, snapshot.Digest, config.Version,
	)
	if err := verifySourceSnapshot(sourceRoot, snapshot.Digest); err != nil {
		return Result{}, fmt.Errorf("source identity before materialization: %w", err)
	}

	artifactPath := filepath.Join(outputRoot, ExecutableName)
	provenancePath := filepath.Join(outputRoot, ProvenanceName)
	if err := copyExclusive(artifacts[0], artifactPath, 0o755); err != nil {
		return Result{}, fmt.Errorf("materialize checker artifact: %w", err)
	}
	if err := writeExclusive(provenancePath, provenance, 0o644); err != nil {
		_ = os.Remove(artifactPath)
		return Result{}, fmt.Errorf("materialize build provenance: %w", err)
	}
	return Result{
		ArtifactPath: artifactPath, ArtifactBytes: artifactBytes,
		ArtifactSHA256: "sha256:" + artifactSHA256,
		ProvenancePath: provenancePath, Source: snapshot.Digest,
		Version: config.Version,
	}, nil
}

func verifySourceSnapshot(root, expected string) error {
	snapshot, err := sourceidentity.Verify(root)
	if err != nil {
		return err
	}
	if snapshot.Digest != expected {
		return fmt.Errorf("checker source identity changed during controlled build")
	}
	return nil
}

func buildOnce(root, toolchainRoot, sourceRoot, source, version string) (string, error) {
	for _, directory := range []string{
		root,
		filepath.Join(root, "cache"),
		filepath.Join(root, "home"),
		filepath.Join(root, "module-cache"),
		filepath.Join(root, "gopath"),
		filepath.Join(root, "tmp"),
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return "", err
		}
	}
	artifact := filepath.Join(root, ExecutableName)
	goExecutable := filepath.Join(toolchainRoot, "bin", "go")
	environment := buildEnvironment(root, toolchainRoot)
	ldflags := "-buildid= -X main.checkerSource=" + source + " -X main.checkerVersion=" + version
	if err := runBounded(
		goExecutable, sourceRoot, environment,
		"build", "-mod=readonly", "-trimpath", "-buildvcs=false", "-p=1",
		"-ldflags="+ldflags, "-o", artifact,
		"./cmd/radishaxiom-independent-checker-go",
	); err != nil {
		return "", err
	}
	info, err := os.Lstat(artifact)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o755 {
		return "", fmt.Errorf("build output is not a 0755 regular file")
	}
	return artifact, nil
}

func buildEnvironment(root, toolchainRoot string) []string {
	return []string{
		"CGO_ENABLED=0",
		"GOARCH=arm64",
		"GOARM64=v8.0",
		"GOCACHE=" + filepath.Join(root, "cache"),
		"GOENV=off",
		"GOFLAGS=",
		"GOMODCACHE=" + filepath.Join(root, "module-cache"),
		"GOOS=darwin",
		"GOPATH=" + filepath.Join(root, "gopath"),
		"GOPROXY=off",
		"GOROOT=" + toolchainRoot,
		"GOSUMDB=off",
		"GOTELEMETRY=off",
		"GOTOOLCHAIN=local",
		"GOWORK=off",
		"HOME=" + filepath.Join(root, "home"),
		"LANG=C",
		"LC_ALL=C",
		"PATH=",
		"TMPDIR=" + filepath.Join(root, "tmp"),
		"TZ=UTC",
	}
}

func verifyToolchain(root, workspace string) error {
	versionRaw, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		return fmt.Errorf("read toolchain VERSION: %w", err)
	}
	if string(versionRaw) != goVersionFile {
		return fmt.Errorf("toolchain VERSION mismatch")
	}
	probeRoot := filepath.Join(workspace, "toolchain-probe")
	for _, directory := range []string{
		probeRoot,
		filepath.Join(probeRoot, "cache"), filepath.Join(probeRoot, "home"),
		filepath.Join(probeRoot, "module-cache"), filepath.Join(probeRoot, "gopath"),
		filepath.Join(probeRoot, "tmp"),
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return err
		}
	}
	goExecutable := filepath.Join(root, "bin", "go")
	output, err := runBoundedOutput(goExecutable, workspace, buildEnvironment(probeRoot, root), "version")
	if err != nil {
		return fmt.Errorf("execute exact toolchain version probe: %w", err)
	}
	if output != "go version go1.26.7 darwin/arm64\n" {
		return fmt.Errorf("toolchain executable version mismatch")
	}
	return nil
}

func extractToolchain(archive, destination string) error {
	if err := os.Mkdir(destination, 0o755); err != nil {
		return fmt.Errorf("create toolchain extraction root: %w", err)
	}
	file, err := os.Open(archive)
	if err != nil {
		return fmt.Errorf("open toolchain archive: %w", err)
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open toolchain gzip stream: %w", err)
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	seen := make(map[string]struct{}, archiveMemberCount)
	members := 0
	var regularBytes int64
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read toolchain tar member: %w", err)
		}
		memberName := header.Name
		if header.Typeflag == tar.TypeDir && strings.HasSuffix(memberName, "/") {
			memberName = strings.TrimSuffix(memberName, "/")
		}
		if err := validateArchivePath(memberName); err != nil {
			return err
		}
		if _, duplicate := seen[memberName]; duplicate {
			return fmt.Errorf("toolchain archive contains duplicate path %q", memberName)
		}
		seen[memberName] = struct{}{}
		members++
		target := filepath.Join(destination, filepath.FromSlash(memberName))
		switch header.Typeflag {
		case tar.TypeDir:
			if header.Mode != 0o755 {
				return fmt.Errorf("toolchain directory %q has invalid mode", header.Name)
			}
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Mode != 0o644 && header.Mode != 0o755 {
				return fmt.Errorf("toolchain file %q has invalid mode", header.Name)
			}
			if header.Size < 0 {
				return fmt.Errorf("toolchain file %q has invalid length", header.Name)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			written, copyErr := io.CopyN(output, reader, header.Size)
			closeErr := output.Close()
			if copyErr != nil || closeErr != nil || written != header.Size {
				return fmt.Errorf("extract toolchain file %q", header.Name)
			}
			regularBytes += written
		default:
			return fmt.Errorf("toolchain archive contains unsupported type at %q", header.Name)
		}
	}
	if members != archiveMemberCount || regularBytes != archiveRegularBytes {
		return fmt.Errorf("toolchain archive inventory mismatch")
	}
	return nil
}

func validateArchivePath(name string) error {
	if name == "" || strings.Contains(name, "\\") || path.IsAbs(name) || path.Clean(name) != name {
		return fmt.Errorf("toolchain archive contains invalid path %q", name)
	}
	if name != "go" && !strings.HasPrefix(name, "go/") {
		return fmt.Errorf("toolchain archive path is outside go root")
	}
	for _, component := range strings.Split(name, "/") {
		if component == "" || component == "." || component == ".." {
			return fmt.Errorf("toolchain archive contains invalid path component")
		}
	}
	return nil
}

func encodeProvenance(artifactBytes int64, artifactSHA256, source, version string) []byte {
	var output strings.Builder
	output.WriteString(`{"artifact":{"byte_length":"`)
	output.WriteString(strconv.FormatInt(artifactBytes, 10))
	output.WriteString(`","filename":"` + ExecutableName + `","raw_sha256":"sha256:`)
	output.WriteString(artifactSHA256)
	output.WriteString(`"},"build":{"command":["build","-mod=readonly","-trimpath","-buildvcs=false","-p=1","-ldflags=-buildid= -X main.checkerSource=<checker.source> -X main.checkerVersion=<implementation.version>","-o","<isolated-output>","./cmd/radishaxiom-independent-checker-go"],"environment":["CGO_ENABLED=0","GOARCH=arm64","GOARM64=v8.0","GOCACHE=<isolated>","GOENV=off","GOFLAGS=","GOMODCACHE=<isolated>","GOOS=darwin","GOPATH=<isolated>","GOPROXY=off","GOROOT=<verified-toolchain>","GOSUMDB=off","GOTELEMETRY=off","GOTOOLCHAIN=local","GOWORK=off","HOME=<isolated>","LANG=C","LC_ALL=C","PATH=","TMPDIR=<isolated>","TZ=UTC"],"profile":"radishaxiom-independent-checker-go-macos-arm64-build-v0.1","target":{"goarch":"arm64","goarm64":"v8.0","goos":"darwin"}},"format":"radishaxiom-checker-build-provenance","format_version":"0.1","implementation":{"source":"`)
	output.WriteString(source)
	output.WriteString(`","version":"`)
	output.WriteString(version)
	output.WriteString(`"},"reproduction":{"build_count":"2","isolated_state":"cache-home-module-cache-tmp","raw_bytes_identical":true},"toolchain_payload":{"byte_length":"64772572","filename":"go1.26.7.darwin-arm64.tar.gz","raw_sha256":"sha256:`)
	output.WriteString(ToolchainArchiveSHA256)
	output.WriteString(`"}}`)
	return []byte(output.String())
}

func validateVersion(version string) error {
	if version == "" || version == "latest" || len(version) > 64 {
		return fmt.Errorf("implementation version is not an exact bounded value")
	}
	if first := version[0]; !((first >= 'a' && first <= 'z') ||
		(first >= 'A' && first <= 'Z') || (first >= '0' && first <= '9')) {
		return fmt.Errorf("implementation version must start with an ASCII alphanumeric character")
	}
	for _, value := range []byte(version) {
		if (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') ||
			(value >= '0' && value <= '9') || strings.ContainsRune("._+-", rune(value)) {
			continue
		}
		return fmt.Errorf("implementation version contains an unsupported character")
	}
	return nil
}

func canonicalDirectory(value string, requireEmpty bool) (string, error) {
	if value == "" || !filepath.IsAbs(value) || filepath.Clean(value) != value {
		return "", fmt.Errorf("path is not absolute and canonical")
	}
	resolved, err := filepath.EvalSymlinks(value)
	if err != nil || resolved != value {
		return "", fmt.Errorf("path is not a canonical realpath")
	}
	info, err := os.Lstat(value)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("path is not a directory")
	}
	if requireEmpty {
		entries, err := os.ReadDir(value)
		if err != nil {
			return "", err
		}
		if len(entries) != 0 {
			return "", fmt.Errorf("directory is not empty")
		}
	}
	return value, nil
}

func canonicalRegularFile(value string) (string, error) {
	if value == "" || !filepath.IsAbs(value) || filepath.Clean(value) != value {
		return "", fmt.Errorf("path is not absolute and canonical")
	}
	resolved, err := filepath.EvalSymlinks(value)
	if err != nil || resolved != value {
		return "", fmt.Errorf("path is not a canonical realpath")
	}
	info, err := os.Lstat(value)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("path is not a regular file")
	}
	return value, nil
}

func hashStableFile(name string) (int64, string, error) {
	before, err := os.Lstat(name)
	if err != nil {
		return 0, "", err
	}
	file, err := os.Open(name)
	if err != nil {
		return 0, "", err
	}
	hash := sha256.New()
	written, copyErr := io.Copy(hash, file)
	opened, statErr := file.Stat()
	closeErr := file.Close()
	after, afterErr := os.Lstat(name)
	if copyErr != nil || statErr != nil || closeErr != nil || afterErr != nil {
		return 0, "", fmt.Errorf("file changed or failed while hashing")
	}
	if !os.SameFile(before, opened) || !os.SameFile(before, after) ||
		before.Mode() != after.Mode() || before.Size() != after.Size() || written != before.Size() {
		return 0, "", fmt.Errorf("file changed while hashing")
	}
	return written, fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func filesEqual(left, right string) (bool, error) {
	leftFile, err := os.Open(left)
	if err != nil {
		return false, err
	}
	defer leftFile.Close()
	rightFile, err := os.Open(right)
	if err != nil {
		return false, err
	}
	defer rightFile.Close()
	leftBuffer := make([]byte, 64<<10)
	rightBuffer := make([]byte, 64<<10)
	for {
		leftCount, leftErr := leftFile.Read(leftBuffer)
		rightCount, rightErr := rightFile.Read(rightBuffer)
		if leftCount != rightCount || !bytes.Equal(leftBuffer[:leftCount], rightBuffer[:rightCount]) {
			return false, nil
		}
		if leftErr == io.EOF && rightErr == io.EOF {
			return true, nil
		}
		if leftErr != nil || rightErr != nil {
			return false, fmt.Errorf("read build outputs: %v / %v", leftErr, rightErr)
		}
	}
}

func copyExclusive(source, target string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func writeExclusive(target string, data []byte, mode os.FileMode) error {
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func runBounded(executable, directory string, environment []string, arguments ...string) error {
	_, err := runBoundedOutput(executable, directory, environment, arguments...)
	return err
}

func runBoundedOutput(executable, directory string, environment []string, arguments ...string) (string, error) {
	command := exec.Command(executable, arguments...)
	command.Dir = directory
	command.Env = environment
	var stdout limitedBuffer
	var stderr limitedBuffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("%w: stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
	if stdout.overflow || stderr.overflow {
		return "", fmt.Errorf("tool output exceeded diagnostic limit")
	}
	if stderr.Len() != 0 {
		return "", fmt.Errorf("tool wrote unexpected stderr: %q", stderr.String())
	}
	return stdout.String(), nil
}

type limitedBuffer struct {
	bytes.Buffer
	overflow bool
}

func (buffer *limitedBuffer) Write(data []byte) (int, error) {
	const limit = 64 << 10
	remaining := limit - buffer.Len()
	if remaining <= 0 {
		buffer.overflow = true
		return len(data), nil
	}
	if len(data) > remaining {
		buffer.overflow = true
		_, _ = buffer.Buffer.Write(data[:remaining])
		return len(data), nil
	}
	return buffer.Buffer.Write(data)
}
