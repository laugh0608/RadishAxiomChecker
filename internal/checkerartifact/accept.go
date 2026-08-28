package checkerartifact

import (
	"bytes"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"debug/macho"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"radishaxiom.dev/independent-checker-go/internal/sourceidentity"
)

const (
	executableName  = "radishaxiom-independent-checker-go"
	provenanceName  = "checker-build-provenance-v0.1.jcs"
	acceptanceName  = "checker-payload-acceptance-v0.1.jcs"
	toolchainSHA256 = "020a1e8224811be75163e920bc77e0926a1390a6aeea19bdcf23f74b9d749f6d"
)

type Config struct {
	SourceRoot string
	BuildRoot  string
	Version    string
}

type Result struct {
	AcceptancePath   string
	AcceptanceSHA256 string
	ArtifactBytes    int64
	ArtifactPath     string
	ArtifactSHA256   string
	Source           string
	Version          string
}

type inspection struct {
	Bytes  int64
	SHA256 string
}

type scenarioResult struct {
	ID           string
	Outcome      string
	RawSHA256    string
	RawByteCount int
}

// Accept independently re-inspects one reproduced build output and exercises
// its public CLI. It does not invoke the compiler or trust builder output.
func Accept(config Config) (Result, error) {
	sourceRoot, err := canonicalDirectory(config.SourceRoot)
	if err != nil {
		return Result{}, fmt.Errorf("source root: %w", err)
	}
	buildRoot, err := canonicalDirectory(config.BuildRoot)
	if err != nil {
		return Result{}, fmt.Errorf("build root: %w", err)
	}
	if err := validateBuildRoot(buildRoot); err != nil {
		return Result{}, err
	}
	if err := validateVersion(config.Version); err != nil {
		return Result{}, err
	}
	snapshot, err := sourceidentity.Verify(sourceRoot)
	if err != nil {
		return Result{}, fmt.Errorf("verify checker source identity: %w", err)
	}
	artifactPath, err := canonicalRegularFile(filepath.Join(buildRoot, executableName), 0o755)
	if err != nil {
		return Result{}, fmt.Errorf("checker artifact: %w", err)
	}
	provenancePath, err := canonicalRegularFile(filepath.Join(buildRoot, provenanceName), 0o644)
	if err != nil {
		return Result{}, fmt.Errorf("build provenance: %w", err)
	}
	acceptancePath := filepath.Join(buildRoot, acceptanceName)
	if _, err := os.Lstat(acceptancePath); !os.IsNotExist(err) {
		return Result{}, fmt.Errorf("acceptance output already exists")
	}

	artifact, err := inspectArtifact(artifactPath)
	if err != nil {
		return Result{}, err
	}
	expectedProvenance := encodeExpectedProvenance(
		artifact.Bytes, artifact.SHA256, snapshot.Digest, config.Version,
	)
	actualProvenance, err := os.ReadFile(provenancePath)
	if err != nil {
		return Result{}, err
	}
	if !bytes.Equal(actualProvenance, expectedProvenance) {
		return Result{}, fmt.Errorf("build provenance does not match independent reconstruction")
	}
	provenanceDigest := sha256.Sum256(actualProvenance)

	scenarioRoot := filepath.Join(sourceRoot, "internal", "bundle", "testdata", "upstream", "s")
	scenarios := []struct {
		id      string
		outcome string
	}{
		{id: "ax-b01-correct", outcome: "accepted-with-trust"},
		{id: "chk-digest-01", outcome: "rejected"},
		{id: "chk-resource-01", outcome: "incomplete"},
	}
	results := make([]scenarioResult, 0, len(scenarios))
	for _, scenario := range scenarios {
		bundleRoot, err := canonicalDirectory(filepath.Join(scenarioRoot, scenario.id, "bundle"))
		if err != nil {
			return Result{}, fmt.Errorf("scenario %s: %w", scenario.id, err)
		}
		raw, err := runScenario(artifactPath, bundleRoot)
		if err != nil {
			return Result{}, fmt.Errorf("scenario %s: %w", scenario.id, err)
		}
		if err := verifyResultIdentity(
			raw, "sha256:"+artifact.SHA256, snapshot.Digest, config.Version, scenario.outcome,
		); err != nil {
			return Result{}, fmt.Errorf("scenario %s: %w", scenario.id, err)
		}
		rawDigest := sha256.Sum256(raw)
		results = append(results, scenarioResult{
			ID: scenario.id, Outcome: scenario.outcome,
			RawSHA256: fmt.Sprintf("sha256:%x", rawDigest), RawByteCount: len(raw),
		})
	}
	acceptance := encodeAcceptance(
		artifact, snapshot.Digest, config.Version,
		fmt.Sprintf("sha256:%x", provenanceDigest), results,
	)
	if err := writeExclusive(acceptancePath, acceptance, 0o644); err != nil {
		return Result{}, fmt.Errorf("write acceptance record: %w", err)
	}
	acceptanceDigest := sha256.Sum256(acceptance)
	return Result{
		AcceptancePath:   acceptancePath,
		AcceptanceSHA256: fmt.Sprintf("sha256:%x", acceptanceDigest),
		ArtifactBytes:    artifact.Bytes, ArtifactPath: artifactPath,
		ArtifactSHA256: "sha256:" + artifact.SHA256,
		Source:         snapshot.Digest, Version: config.Version,
	}, nil
}

func inspectArtifact(path string) (inspection, error) {
	byteCount, digest, err := hashStableFile(path)
	if err != nil {
		return inspection{}, fmt.Errorf("hash checker artifact: %w", err)
	}
	machoFile, err := macho.Open(path)
	if err != nil {
		return inspection{}, fmt.Errorf("parse checker Mach-O: %w", err)
	}
	if machoFile.Magic != macho.Magic64 || machoFile.Cpu != macho.CpuArm64 || machoFile.Type != macho.TypeExec {
		machoFile.Close()
		return inspection{}, fmt.Errorf("checker artifact is not a darwin arm64 executable")
	}
	if err := machoFile.Close(); err != nil {
		return inspection{}, err
	}
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return inspection{}, fmt.Errorf("read checker Go build info: %w", err)
	}
	if info.GoVersion != "go1.26.7" ||
		info.Path != "radishaxiom.dev/independent-checker-go/cmd/radishaxiom-independent-checker-go" ||
		info.Main.Path != "radishaxiom.dev/independent-checker-go" ||
		info.Main.Version != "(devel)" || info.Main.Sum != "" || len(info.Deps) != 0 {
		return inspection{}, fmt.Errorf("checker Go module identity mismatch")
	}
	settings := make(map[string]string, len(info.Settings))
	for _, setting := range info.Settings {
		if _, duplicate := settings[setting.Key]; duplicate {
			return inspection{}, fmt.Errorf("checker build info contains duplicate setting %q", setting.Key)
		}
		settings[setting.Key] = setting.Value
	}
	expected := map[string]string{
		"-buildmode": "exe", "-compiler": "gc", "-trimpath": "true",
		"CGO_ENABLED": "0", "GOARCH": "arm64", "GOARM64": "v8.0", "GOOS": "darwin",
	}
	if len(settings) != len(expected) {
		return inspection{}, fmt.Errorf("checker build setting set is not closed")
	}
	for key, value := range expected {
		if settings[key] != value {
			return inspection{}, fmt.Errorf("checker build setting %s mismatch", key)
		}
	}
	return inspection{Bytes: byteCount, SHA256: digest}, nil
}

func runScenario(artifactPath, bundleRoot string) ([]byte, error) {
	workingDirectory, err := os.MkdirTemp("", "radishaxiom-checker-accept-run-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workingDirectory)
	workingDirectory, err = filepath.EvalSymlinks(workingDirectory)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, artifactPath, "check", "--bundle-root="+bundleRoot)
	command.Dir = workingDirectory
	command.Env = []string{}
	command.Stdin = strings.NewReader("")
	stdout := cappedBuffer{limit: 1 << 20}
	stderr := cappedBuffer{limit: 64 << 10}
	command.Stdout = &stdout
	command.Stderr = &stderr
	err = command.Run()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("checker exceeded acceptance wall-clock")
	}
	if err != nil {
		return nil, fmt.Errorf("checker exited unsuccessfully: %w; stderr=%q", err, stderr.String())
	}
	if stdout.overflow || stderr.overflow {
		return nil, fmt.Errorf("checker exceeded acceptance output limit")
	}
	if stderr.Len() != 0 || stdout.Len() == 0 {
		return nil, fmt.Errorf("checker stdout/stderr boundary mismatch")
	}
	return append([]byte(nil), stdout.Bytes()...), nil
}

func verifyResultIdentity(raw []byte, artifact, source, version, outcome string) error {
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return fmt.Errorf("result is not JSON: %w", err)
	}
	if err := rejectNumberOrNull(decoded); err != nil {
		return err
	}
	reencoded, err := json.Marshal(decoded)
	if err != nil || !bytes.Equal(reencoded, raw) {
		return fmt.Errorf("result is not canonical in the independent acceptance profile")
	}
	root, err := exactObject(decoded, []string{
		"checker", "checks", "evidence", "missing_artifacts", "remaining_trust",
		"request", "result", "result_version", "tcb",
	})
	if err != nil {
		return err
	}
	if root["result_version"] != "0.1" {
		return fmt.Errorf("result version mismatch")
	}
	checker, err := exactObject(root["checker"], []string{"artifact", "name", "source", "toolchain", "version"})
	if err != nil {
		return err
	}
	if checker["artifact"] != artifact || checker["name"] != executableName ||
		checker["source"] != source || checker["toolchain"] != "go1.26.7" || checker["version"] != version {
		return fmt.Errorf("result checker identity mismatch")
	}
	result, err := exactObject(root["result"], []string{"kind", "refs"})
	if err != nil {
		return err
	}
	if result["kind"] != outcome {
		return fmt.Errorf("result outcome mismatch")
	}
	tcb, ok := root["tcb"].([]any)
	if !ok || len(tcb) != 4 {
		return fmt.Errorf("result TCB cardinality mismatch")
	}
	categories := []string{"canonicalization", "checker-core", "cryptographic-primitive", "rule-interpreter"}
	for index, rawComponent := range tcb {
		component, err := exactObject(rawComponent, []string{"artifact", "category", "version"})
		if err != nil {
			return err
		}
		if component["artifact"] != artifact || component["category"] != categories[index] || component["version"] != version {
			return fmt.Errorf("result TCB identity mismatch")
		}
	}
	return nil
}

func rejectNumberOrNull(value any) error {
	switch typed := value.(type) {
	case nil, float64:
		return fmt.Errorf("result contains JSON number or null")
	case []any:
		for _, item := range typed {
			if err := rejectNumberOrNull(item); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, item := range typed {
			if err := rejectNumberOrNull(item); err != nil {
				return err
			}
		}
	case string, bool:
	default:
		return fmt.Errorf("result contains unsupported JSON value")
	}
	return nil
}

func exactObject(value any, keys []string) (map[string]any, error) {
	object, ok := value.(map[string]any)
	if !ok || len(object) != len(keys) {
		return nil, fmt.Errorf("result object shape mismatch")
	}
	for _, key := range keys {
		if _, present := object[key]; !present {
			return nil, fmt.Errorf("result object lacks %s", key)
		}
	}
	return object, nil
}

func encodeAcceptance(artifact inspection, source, version, provenance string, scenarios []scenarioResult) []byte {
	var output strings.Builder
	output.WriteString(`{"acceptance":{"accepted_scope":["artifact-byte-reproducibility","build-metadata","runtime-self-identity","scenario-behavior"],"decision":"accepted-for-controlled-runtime-registration","excluded_scope":["cross-platform-equivalence","installation","launcher-hard-isolation","legal-compliance-for-distribution","publication","release-signing"]},"artifact":{"byte_length":"`)
	output.WriteString(strconv.FormatInt(artifact.Bytes, 10))
	output.WriteString(`","filename":"` + executableName + `","raw_sha256":"sha256:`)
	output.WriteString(artifact.SHA256)
	output.WriteString(`"},"build":{"profile":"radishaxiom-independent-checker-go-macos-arm64-build-v0.1","provenance_raw_sha256":"`)
	output.WriteString(provenance)
	output.WriteString(`","reproduction":"two-isolated-byte-identical-builds"},"format":"radishaxiom-checker-payload-acceptance","format_version":"0.1","identity":{"source":"`)
	output.WriteString(source)
	output.WriteString(`","toolchain":"go1.26.7","version":"`)
	output.WriteString(version)
	output.WriteString(`"},"scenarios":[`)
	for index, scenario := range scenarios {
		if index != 0 {
			output.WriteByte(',')
		}
		output.WriteString(`{"id":"`)
		output.WriteString(scenario.ID)
		output.WriteString(`","outcome":"`)
		output.WriteString(scenario.Outcome)
		output.WriteString(`","result_byte_length":"`)
		output.WriteString(strconv.Itoa(scenario.RawByteCount))
		output.WriteString(`","result_raw_sha256":"`)
		output.WriteString(scenario.RawSHA256)
		output.WriteString(`"}`)
	}
	output.WriteString(`],"target":{"goarch":"arm64","goarm64":"v8.0","goos":"darwin","macho":"64-bit-arm64-executable"}}`)
	return []byte(output.String())
}

func encodeExpectedProvenance(artifactBytes int64, artifactSHA256, source, version string) []byte {
	var output strings.Builder
	output.WriteString(`{"artifact":{"byte_length":"`)
	output.WriteString(strconv.FormatInt(artifactBytes, 10))
	output.WriteString(`","filename":"` + executableName + `","raw_sha256":"sha256:`)
	output.WriteString(artifactSHA256)
	output.WriteString(`"},"build":{"command":["build","-mod=readonly","-trimpath","-buildvcs=false","-p=1","-ldflags=-buildid= -X main.checkerSource=<checker.source> -X main.checkerVersion=<implementation.version>","-o","<isolated-output>","./cmd/radishaxiom-independent-checker-go"],"environment":["CGO_ENABLED=0","GOARCH=arm64","GOARM64=v8.0","GOCACHE=<isolated>","GOENV=off","GOFLAGS=","GOMODCACHE=<isolated>","GOOS=darwin","GOPATH=<isolated>","GOPROXY=off","GOROOT=<verified-toolchain>","GOSUMDB=off","GOTELEMETRY=off","GOTOOLCHAIN=local","GOWORK=off","HOME=<isolated>","LANG=C","LC_ALL=C","PATH=","TMPDIR=<isolated>","TZ=UTC"],"profile":"radishaxiom-independent-checker-go-macos-arm64-build-v0.1","target":{"goarch":"arm64","goarm64":"v8.0","goos":"darwin"}},"format":"radishaxiom-checker-build-provenance","format_version":"0.1","implementation":{"source":"`)
	output.WriteString(source)
	output.WriteString(`","version":"`)
	output.WriteString(version)
	output.WriteString(`"},"reproduction":{"build_count":"2","isolated_state":"cache-home-module-cache-tmp","raw_bytes_identical":true},"toolchain_payload":{"byte_length":"64772572","filename":"go1.26.7.darwin-arm64.tar.gz","raw_sha256":"sha256:`)
	output.WriteString(toolchainSHA256)
	output.WriteString(`"}}`)
	return []byte(output.String())
}

func validateVersion(version string) error {
	if version == "" || version == "latest" || len(version) > 64 {
		return fmt.Errorf("implementation version is not an exact bounded value")
	}
	for index, value := range []byte(version) {
		allowed := (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') ||
			(value >= '0' && value <= '9') || strings.ContainsRune("._+-", rune(value))
		if !allowed || (index == 0 && !((value >= 'a' && value <= 'z') ||
			(value >= 'A' && value <= 'Z') || (value >= '0' && value <= '9'))) {
			return fmt.Errorf("implementation version contains an unsupported character")
		}
	}
	return nil
}

func canonicalDirectory(value string) (string, error) {
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
	return value, nil
}

func validateBuildRoot(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	if len(entries) != 2 || entries[0].Name() != provenanceName || entries[1].Name() != executableName {
		return fmt.Errorf("build root does not contain exactly the artifact and provenance")
	}
	return nil
}

func canonicalRegularFile(value string, mode os.FileMode) (string, error) {
	if value == "" || !filepath.IsAbs(value) || filepath.Clean(value) != value {
		return "", fmt.Errorf("path is not absolute and canonical")
	}
	resolved, err := filepath.EvalSymlinks(value)
	if err != nil || resolved != value {
		return "", fmt.Errorf("path is not a canonical realpath")
	}
	info, err := os.Lstat(value)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != mode ||
		info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return "", fmt.Errorf("path is not a plain %04o regular file", mode)
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
	if copyErr != nil || statErr != nil || closeErr != nil || afterErr != nil ||
		!os.SameFile(before, opened) || !os.SameFile(before, after) ||
		before.Mode() != after.Mode() || before.Size() != after.Size() || written != before.Size() {
		return 0, "", fmt.Errorf("file changed while hashing")
	}
	return written, fmt.Sprintf("%x", hash.Sum(nil)), nil
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

type cappedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (buffer *cappedBuffer) Write(data []byte) (int, error) {
	remaining := buffer.limit - buffer.Len()
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
