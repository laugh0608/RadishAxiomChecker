package checkerbuild

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncodeProvenanceIsPathAndTimeIndependent(t *testing.T) {
	raw := encodeProvenance(
		1234,
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"sha256:abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		"0.1-dev",
	)
	if bytes.Contains(raw, []byte("\n")) || bytes.Contains(raw, []byte("/private/")) || bytes.Contains(raw, []byte("timestamp")) {
		t.Fatalf("provenance contains ambient representation: %s", raw)
	}
	for _, required := range []string{
		`"build_count":"2"`, `"raw_bytes_identical":true`,
		`"toolchain_payload":{"byte_length":"64772572"`,
		`"version":"0.1-dev"`,
	} {
		if !bytes.Contains(raw, []byte(required)) {
			t.Fatalf("provenance lacks %s", required)
		}
	}
}

func TestValidateVersionRejectsAmbiguousValues(t *testing.T) {
	for _, valid := range []string{"0.1-dev", "26.8.1.2801-test", "v1+local"} {
		if err := validateVersion(valid); err != nil {
			t.Fatalf("valid version %q: %v", valid, err)
		}
	}
	for _, invalid := range []string{"", "latest", ".hidden", "+local", "with space", "路径", strings.Repeat("a", 65)} {
		if err := validateVersion(invalid); err == nil {
			t.Fatalf("invalid version %q was accepted", invalid)
		}
	}
}

func TestArchivePathRejectsTraversalAndAliases(t *testing.T) {
	for _, name := range []string{"", "../go/bin/go", "go/../outside", "/go/bin/go", `go\\bin\\go`, "other/bin/go", "go//bin/go"} {
		if err := validateArchivePath(name); err == nil {
			t.Fatalf("invalid archive path %q was accepted", name)
		}
	}
	for _, name := range []string{"go", "go/bin", "go/bin/go"} {
		if err := validateArchivePath(name); err != nil {
			t.Fatalf("valid archive path %q: %v", name, err)
		}
	}
}

func TestExtractToolchainRejectsLinkMember(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "toolchain.tar.gz")
	writeTestArchive(t, archive, []*tar.Header{
		{Name: "go", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "go/bin", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "go/bin/go", Typeflag: tar.TypeSymlink, Mode: 0o755, Linkname: "elsewhere"},
	})
	if err := extractToolchain(archive, filepath.Join(t.TempDir(), "out")); err == nil || !strings.Contains(err.Error(), "unsupported type") {
		t.Fatalf("link archive error = %v", err)
	}
}

func TestFilesEqualDetectsContentDrift(t *testing.T) {
	directory := t.TempDir()
	left := filepath.Join(directory, "left")
	right := filepath.Join(directory, "right")
	if err := os.WriteFile(left, []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}
	equal, err := filesEqual(left, right)
	if err != nil || !equal {
		t.Fatalf("equal files = %t, %v", equal, err)
	}
	if err := os.WriteFile(right, []byte("drift"), 0o644); err != nil {
		t.Fatal(err)
	}
	equal, err = filesEqual(left, right)
	if err != nil || equal {
		t.Fatalf("drift files = %t, %v", equal, err)
	}
}

func TestVerifySourceSnapshotRejectsWrongIdentity(t *testing.T) {
	root := canonicalTestPath(t, filepath.Join("..", ".."))
	if err := verifySourceSnapshot(root, "sha256:0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Fatal("wrong source identity was accepted")
	}
}

func canonicalTestPath(t *testing.T, value string) string {
	t.Helper()
	absolute, err := filepath.Abs(value)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func writeTestArchive(t *testing.T, target string, headers []*tar.Header) {
	t.Helper()
	file, err := os.Create(target)
	if err != nil {
		t.Fatal(err)
	}
	compressed := gzip.NewWriter(file)
	archive := tar.NewWriter(compressed)
	for _, header := range headers {
		if err := archive.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
