package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

const blobName = "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a"

func TestVerifyImportedBundle(t *testing.T) {
	root := validBundle(t)
	verified, err := Verify(root)
	if err != nil {
		t.Fatal(err)
	}
	if verified.ManifestDigest != verified.Request.BundleManifest {
		t.Fatal("manifest digest was not bound to request")
	}
	if len(verified.Manifest.Artifacts) != 1 {
		t.Fatalf("unexpected artifact count: %d", len(verified.Manifest.Artifacts))
	}
}

func TestVerifyImportedTwentyEightBundleBoundary(t *testing.T) {
	root := filepath.Join("testdata", "upstream", "s")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 28 {
		t.Fatalf("expected 28 imported scenarios, got %d", len(entries))
	}
	rejections := map[string]rejection.Code{
		"chk-bundle-01":   rejection.ArtifactMissing,
		"chk-digest-01":   rejection.DigestMismatch,
		"chk-resource-01": rejection.ResourceLimit,
	}
	for _, entry := range entries {
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			_, err := Verify(filepath.Join(root, name, "bundle"))
			if want, rejected := rejections[name]; rejected {
				assertBundleCode(t, err, want)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestImportedBundleCorpusRemainsDigestLocked(t *testing.T) {
	type fileRecord struct {
		ByteLength string `json:"byte_length"`
		Path       string `json:"path"`
		SHA256     string `json:"sha256"`
	}
	type sourceLock struct {
		Files []fileRecord `json:"files"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata", "upstream", "contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := sha256.Sum256(raw); hex.EncodeToString(got[:]) != "a349152cb2f838cf5acfaa66ef1676554f6d5f453d07314b3cc1b8c5579c7974" {
		t.Fatal("imported contract.json identity drifted")
	}
	var lock sourceLock
	if err := json.Unmarshal(raw, &lock); err != nil {
		t.Fatal(err)
	}
	locked := make(map[string]struct{})
	for _, record := range lock.Files {
		if !strings.HasPrefix(record.Path, "s/") {
			continue
		}
		data, err := os.ReadFile(filepath.Join("testdata", "upstream", filepath.FromSlash(record.Path)))
		if err != nil {
			t.Fatalf("%s: %v", record.Path, err)
		}
		sum := sha256.Sum256(data)
		if "sha256:"+hex.EncodeToString(sum[:]) != record.SHA256 {
			t.Fatalf("%s: SHA-256 drift", record.Path)
		}
		locked[filepath.Clean(filepath.FromSlash(record.Path))] = struct{}{}
	}
	err = filepath.WalkDir(filepath.Join("testdata", "upstream", "s"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(filepath.Join("testdata", "upstream"), path)
		if err != nil {
			return err
		}
		if _, ok := locked[relative]; !ok {
			t.Errorf("unlocked imported file: %s", relative)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestVerifyRejectsBundleBoundaryViolations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, string)
		code   rejection.Code
	}{
		{
			"unlisted root entry",
			func(t *testing.T, root string) { write(t, filepath.Join(root, "extra"), []byte{}) },
			rejection.ManifestCoverage,
		},
		{
			"unlisted blob",
			func(t *testing.T, root string) {
				write(t, filepath.Join(root, "blobs", "sha256", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), []byte{})
			},
			rejection.ManifestCoverage,
		},
		{
			"short blob name",
			func(t *testing.T, root string) {
				old := filepath.Join(root, "blobs", "sha256", blobName)
				if err := os.Rename(old, filepath.Join(root, "blobs", "sha256", blobName[:8])); err != nil {
					t.Fatal(err)
				}
			},
			rejection.IsolationBoundaryViolation,
		},
		{
			"uppercase blob name",
			func(t *testing.T, root string) {
				old := filepath.Join(root, "blobs", "sha256", blobName)
				upper := "44136FA355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a"
				if err := os.Rename(old, filepath.Join(root, "blobs", "sha256", upper)); err != nil {
					t.Fatal(err)
				}
			},
			rejection.IsolationBoundaryViolation,
		},
		{
			"missing blob",
			func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, "blobs", "sha256", blobName)); err != nil {
					t.Fatal(err)
				}
			},
			rejection.ArtifactMissing,
		},
		{
			"wrong blob length",
			func(t *testing.T, root string) {
				write(t, filepath.Join(root, "blobs", "sha256", blobName), []byte("x"))
			},
			rejection.LengthMismatch,
		},
		{
			"wrong blob digest",
			func(t *testing.T, root string) {
				write(t, filepath.Join(root, "blobs", "sha256", blobName), []byte("[]"))
			},
			rejection.DigestMismatch,
		},
		{
			"manifest request binding",
			func(t *testing.T, root string) {
				manifest := []byte(`{"artifacts":[{"byte_length":"2","content_digest":"sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a","format":"axiom-evidence","format_version":"0.2","roles":["evidence"]}],"bundle_version":"0.1"}`)
				write(t, filepath.Join(root, "manifest.jcs"), manifest)
			},
			rejection.RequestBindingMismatch,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := validBundle(t)
			test.mutate(t, root)
			_, err := Verify(root)
			assertBundleCode(t, err, test.code)
		})
	}
}

func TestVerifyRejectsSymlinkedInput(t *testing.T) {
	root := validBundle(t)
	blob := filepath.Join(root, "blobs", "sha256", blobName)
	target := filepath.Join(t.TempDir(), "target")
	write(t, target, []byte("{}"))
	if err := os.Remove(blob); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, blob); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	_, err := Verify(root)
	assertBundleCode(t, err, rejection.IsolationBoundaryViolation)
}

func validBundle(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	shaDir := filepath.Join(root, "blobs", "sha256")
	if err := os.MkdirAll(shaDir, 0o700); err != nil {
		t.Fatal(err)
	}
	copyFixture(t, filepath.Join("..", "protocol", "testdata", "upstream", "valid", "request.jcs"), filepath.Join(root, "request.jcs"))
	copyFixture(t, filepath.Join("..", "protocol", "testdata", "upstream", "valid", "manifest.jcs"), filepath.Join(root, "manifest.jcs"))
	copyFixture(t, filepath.Join("..", "protocol", "testdata", "upstream", "valid", "blobs", "sha256", blobName), filepath.Join(shaDir, blobName))
	return root
}

func copyFixture(t *testing.T, source, target string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	write(t, target, data)
}

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertBundleCode(t *testing.T, err error, want rejection.Code) {
	t.Helper()
	if got, ok := rejection.CodeOf(err); err == nil || !ok || got != want {
		t.Fatalf("expected %s, got %v", want, err)
	}
}
