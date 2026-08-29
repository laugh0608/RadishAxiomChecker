package payloadarchive

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestPackIsDeterministicAndVerifiable(t *testing.T) {
	root := canonicalTempDir(t)
	sourceRoot := repositoryRoot(t)
	buildA := filepath.Join(root, "build-a")
	buildB := filepath.Join(root, "build-b")
	writeSyntheticAcceptedBuild(t, buildA, time.Unix(10, 0))
	writeSyntheticAcceptedBuild(t, buildB, time.Unix(20, 0))
	archiveA := filepath.Join(root, "candidate-a.tar")
	archiveB := filepath.Join(root, "candidate-b.tar")

	resultA, err := Pack(PackConfig{
		BuildRoot: buildA, OutputFile: archiveA,
		SourceRoot: sourceRoot, Version: "0.1-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	resultB, err := Pack(PackConfig{
		BuildRoot: buildB, OutputFile: archiveB,
		SourceRoot: sourceRoot, Version: "0.1-test",
	})
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
	if !bytes.Equal(left, right) {
		t.Fatal("payload archive depends on source paths or mtimes")
	}
	if resultA.ArchiveSHA256 != resultB.ArchiveSHA256 ||
		resultA.ManifestSHA256 != resultB.ManifestSHA256 ||
		resultA.Source == resultA.ArchiveSHA256 || resultA.Version != "0.1-test" {
		t.Fatal("payload archive result identities are inconsistent")
	}
	verified, err := Verify(archiveA)
	if err != nil {
		t.Fatal(err)
	}
	if verified.ArchiveSHA256 != resultA.ArchiveSHA256 ||
		verified.ManifestSHA256 != resultA.ManifestSHA256 ||
		verified.ArchiveBytes != int64(len(left)) {
		t.Fatal("payload archive verification result drifted")
	}
}

func TestPackRejectsOpenAcceptedBuildRoot(t *testing.T) {
	root := canonicalTempDir(t)
	buildRoot := filepath.Join(root, "build")
	writeSyntheticAcceptedBuild(t, buildRoot, time.Unix(10, 0))
	if err := os.WriteFile(filepath.Join(buildRoot, "unexpected"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Pack(PackConfig{
		BuildRoot:  buildRoot,
		OutputFile: filepath.Join(root, "candidate.tar"),
		SourceRoot: repositoryRoot(t),
		Version:    "0.1-test",
	})
	if err == nil {
		t.Fatal("open accepted build root was packaged")
	}
}

func TestPackRejectsWrongPayloadMode(t *testing.T) {
	root := canonicalTempDir(t)
	buildRoot := filepath.Join(root, "build")
	writeSyntheticAcceptedBuild(t, buildRoot, time.Unix(10, 0))
	if err := os.Chmod(filepath.Join(buildRoot, ExecutableName), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Pack(PackConfig{
		BuildRoot:  buildRoot,
		OutputFile: filepath.Join(root, "candidate.tar"),
		SourceRoot: repositoryRoot(t),
		Version:    "0.1-test",
	})
	if err == nil {
		t.Fatal("payload with the wrong executable mode was packaged")
	}
}

func TestPackRejectsExistingOutput(t *testing.T) {
	root := canonicalTempDir(t)
	buildRoot := filepath.Join(root, "build")
	writeSyntheticAcceptedBuild(t, buildRoot, time.Unix(10, 0))
	output := filepath.Join(root, "candidate.tar")
	if err := os.WriteFile(output, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Pack(PackConfig{
		BuildRoot: buildRoot, OutputFile: output,
		SourceRoot: repositoryRoot(t), Version: "0.1-test",
	})
	if err == nil {
		t.Fatal("existing output was overwritten")
	}
}

func TestVerifyRejectsTrailingArchiveBytes(t *testing.T) {
	root := canonicalTempDir(t)
	buildRoot := filepath.Join(root, "build")
	writeSyntheticAcceptedBuild(t, buildRoot, time.Unix(10, 0))
	archive := filepath.Join(root, "candidate.tar")
	if _, err := Pack(PackConfig{
		BuildRoot: buildRoot, OutputFile: archive,
		SourceRoot: repositoryRoot(t), Version: "0.1-test",
	}); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(archive, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("unexpected")); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(archive); err == nil {
		t.Fatal("archive with trailing bytes was accepted")
	}
}

func TestVerifyRejectsPayloadMutation(t *testing.T) {
	root := canonicalTempDir(t)
	buildRoot := filepath.Join(root, "build")
	writeSyntheticAcceptedBuild(t, buildRoot, time.Unix(10, 0))
	archive := filepath.Join(root, "candidate.tar")
	if _, err := Pack(PackConfig{
		BuildRoot: buildRoot, OutputFile: archive,
		SourceRoot: repositoryRoot(t), Version: "0.1-test",
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)/2] ^= 0xff
	if err := os.WriteFile(archive, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(archive); err == nil {
		t.Fatal("mutated payload archive was accepted")
	}
}

func TestDecodeManifestRejectsUnknownAndNonCanonicalJSON(t *testing.T) {
	inputs := []member{
		{mode: 0o644, name: ProvenanceName, raw: []byte("provenance")},
		{mode: 0o644, name: AcceptanceName, raw: []byte("acceptance")},
		{mode: 0o755, name: ExecutableName, raw: []byte("artifact")},
	}
	raw, err := encodeManifest(
		inputs,
		"sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"0.1-test",
	)
	if err != nil {
		t.Fatal(err)
	}
	unknown := append(append([]byte(nil), raw[:len(raw)-1]...), []byte(`,"unexpected":"member"}`)...)
	if _, err := decodeManifest(unknown); err == nil {
		t.Fatal("retention manifest with an unknown member was accepted")
	}
	nonCanonical := append([]byte(" "), raw...)
	if _, err := decodeManifest(nonCanonical); err == nil {
		t.Fatal("non-canonical retention manifest was accepted")
	}
}

func writeSyntheticAcceptedBuild(t *testing.T, root string, modified time.Time) {
	t.Helper()
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	files := []struct {
		mode os.FileMode
		name string
		raw  string
	}{
		{mode: 0o755, name: ExecutableName, raw: "synthetic checker artifact"},
		{mode: 0o644, name: ProvenanceName, raw: "{\"synthetic\":\"provenance\"}"},
		{mode: 0o644, name: AcceptanceName, raw: "{\"synthetic\":\"acceptance\"}"},
	}
	for _, file := range files {
		path := filepath.Join(root, file.name)
		if err := os.WriteFile(path, []byte(file.raw), file.mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, modified, modified); err != nil {
			t.Fatal(err)
		}
	}
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
