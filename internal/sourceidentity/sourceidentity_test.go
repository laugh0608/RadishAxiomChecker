package sourceidentity

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const updateEnvironment = "RADISHAXIOM_UPDATE_SOURCE_IDENTITY"

func TestRepositoryManifestMatches(t *testing.T) {
	root := repositoryRoot(t)
	snapshot, err := Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv(updateEnvironment) == "1" {
		writeRepositoryManifest(t, root, snapshot.Manifest)
	}
	verified, err := Verify(root)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Digest != snapshot.Digest || !bytes.Equal(verified.Manifest, snapshot.Manifest) {
		t.Fatal("verified source identity differs from generated identity")
	}
	t.Logf("checker.source=%s files=%d manifest_bytes=%d", verified.Digest, verified.FileCount, verified.ManifestSize)
}

func TestGenerateCanonicalRawByteManifest(t *testing.T) {
	root := fixtureRoot(t)
	raw := []byte{0x00, '\r', '\n', 0xff}
	writeMode(t, filepath.Join(root, "A.bin"), raw, 0o644)
	writeMode(t, filepath.Join(root, "z.sh"), []byte("#!/bin/sh\n"), 0o755)
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeMode(t, filepath.Join(root, ".git", "config"), []byte("not an identity input"), 0o600)

	snapshot, err := Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"content_encoding":"raw-bytes","digest_algorithm":"sha256","exclusions":[{"path":".git","scope":"entry-and-descendants"},{"path":"source-identity/checker-source-v0.1.jcs","scope":"exact-file"}],"files":[` +
		`{"byte_length":"4","content_digest":"` + testDigest(raw) + `","mode":"0644","path":"A.bin","type":"regular"},` +
		`{"byte_length":"` + strconv.Itoa(len(expectedGoMod)) + `","content_digest":"` + testDigest([]byte(expectedGoMod)) + `","mode":"0644","path":"go.mod","type":"regular"},` +
		`{"byte_length":"10","content_digest":"` + testDigest([]byte("#!/bin/sh\n")) + `","mode":"0755","path":"z.sh","type":"regular"}` +
		`],"identity":"checker.source","manifest_encoding":"RFC8785-JCS-UTF-8","module":{"go_language":"1.26.0","go_toolchain":"go1.26.7","module_path":"radishaxiom.dev/independent-checker-go"},"path_encoding":"UTF-8","path_order":"UTF-8-byte-lexicographic","source_version":"0.1"}`
	if got := string(snapshot.Manifest); got != expected {
		t.Fatalf("canonical manifest mismatch\nwant: %s\n got: %s", expected, got)
	}
	if snapshot.Digest != testDigest([]byte(expected)) {
		t.Fatalf("unexpected source digest: %s", snapshot.Digest)
	}
	if bytes.Contains(snapshot.Manifest, []byte("not an identity input")) || bytes.HasSuffix(snapshot.Manifest, []byte("\n")) {
		t.Fatal("manifest included excluded Git bytes or a trailing newline")
	}
}

func TestGeneratedSidecarDoesNotHashItself(t *testing.T) {
	root := fixtureRoot(t)
	before, err := Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, filepath.FromSlash(ManifestPath))
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	writeMode(t, manifestPath, []byte("sidecar bytes are excluded"), 0o644)
	after, err := Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	if before.Digest != after.Digest || !bytes.Equal(before.Manifest, after.Manifest) {
		t.Fatal("generated sidecar became a recursive source input")
	}
}

func TestGitWorktreeMetadataFileIsExcluded(t *testing.T) {
	root := fixtureRoot(t)
	before, err := Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	writeMode(t, filepath.Join(root, ".git"), []byte("gitdir: elsewhere\n"), 0o600)
	after, err := Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	if before.Digest != after.Digest || !bytes.Equal(before.Manifest, after.Manifest) {
		t.Fatal("worktree .git metadata file became a source input")
	}
}

func TestVerifyFailsClosedOnDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, string)
		want   error
	}{
		{
			name: "untracked source file",
			mutate: func(t *testing.T, root string) {
				writeMode(t, filepath.Join(root, "untracked.go"), []byte("package untracked\n"), 0o644)
			},
			want: ErrManifestDrift,
		},
		{
			name: "source byte change",
			mutate: func(t *testing.T, root string) {
				writeMode(t, filepath.Join(root, "sample.txt"), []byte("changed\r\n"), 0o644)
			},
			want: ErrManifestDrift,
		},
		{
			name: "source mode change",
			mutate: func(t *testing.T, root string) {
				if err := os.Chmod(filepath.Join(root, "sample.txt"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: ErrInvalidMode,
		},
		{
			name: "generated manifest drift",
			mutate: func(t *testing.T, root string) {
				path := filepath.Join(root, filepath.FromSlash(ManifestPath))
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				writeMode(t, path, append(data, '\n'), 0o644)
			},
			want: ErrManifestDrift,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := fixtureRoot(t)
			writeMode(t, filepath.Join(root, "sample.txt"), []byte("original\n"), 0o644)
			writeCurrentManifest(t, root)
			test.mutate(t, root)
			_, err := Verify(root)
			if !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}

func TestGenerateRejectsModuleIdentityDrift(t *testing.T) {
	tests := []struct {
		name string
		old  string
		new  string
	}{
		{"module path", "radishaxiom.dev/independent-checker-go", "example.invalid/checker"},
		{"language baseline", "go 1.26.0", "go 1.27.0"},
		{"toolchain", "toolchain go1.26.7", "toolchain go1.26.3"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := fixtureRoot(t)
			changed := strings.Replace(expectedGoMod, test.old, test.new, 1)
			writeMode(t, filepath.Join(root, "go.mod"), []byte(changed), 0o644)
			_, err := Generate(root)
			if !errors.Is(err, ErrModuleIdentity) {
				t.Fatalf("expected module identity rejection, got %v", err)
			}
		})
	}
}

func TestGenerateRejectsAliasesAndSpecialEntries(t *testing.T) {
	t.Run("nonportable path", func(t *testing.T) {
		root := fixtureRoot(t)
		writeMode(t, filepath.Join(root, "bad name.go"), []byte("package bad\n"), 0o644)
		_, err := Generate(root)
		if !errors.Is(err, ErrInvalidPath) {
			t.Fatalf("expected path rejection, got %v", err)
		}
	})

	t.Run("symlink", func(t *testing.T) {
		root := fixtureRoot(t)
		target := filepath.Join(root, "target.txt")
		writeMode(t, target, []byte("target"), 0o644)
		if err := os.Symlink(target, filepath.Join(root, "alias.txt")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		_, err := Generate(root)
		if !errors.Is(err, ErrUnsupportedType) {
			t.Fatalf("expected symlink rejection, got %v", err)
		}
	})

	t.Run("sidecar symlink", func(t *testing.T) {
		root := fixtureRoot(t)
		dir := filepath.Join(root, "source-identity")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "go.mod"), filepath.Join(root, filepath.FromSlash(ManifestPath))); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		_, err := Generate(root)
		if !errors.Is(err, ErrUnsupportedType) {
			t.Fatalf("expected sidecar symlink rejection, got %v", err)
		}
	})
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate sourceidentity test source")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	return root
}

func fixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeMode(t, filepath.Join(root, "go.mod"), []byte(expectedGoMod), 0o644)
	return root
}

func writeCurrentManifest(t *testing.T, root string) {
	t.Helper()
	snapshot, err := Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(ManifestPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeMode(t, path, snapshot.Manifest, 0o644)
}

func writeRepositoryManifest(t *testing.T, root string, manifest []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(ManifestPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			t.Fatal("refusing to replace a non-regular source manifest")
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeMode(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func testDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
