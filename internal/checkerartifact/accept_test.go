package checkerartifact

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testDigest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestVerifyResultIdentityAcceptsIndependentCanonicalSubset(t *testing.T) {
	raw := canonicalTestResult("accepted-with-trust")
	if err := verifyResultIdentity(raw, testDigest, testDigest, "0.1-dev", "accepted-with-trust"); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyResultIdentityRejectsRepresentationAndIdentityDrift(t *testing.T) {
	valid := canonicalTestResult("rejected")
	tests := [][]byte{
		append([]byte(" "), valid...),
		bytes.Replace(valid, []byte(testDigest), []byte("sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), 1),
		bytes.Replace(valid, []byte(`"result_version":"0.1"`), []byte(`"result_version":null`), 1),
		bytes.Replace(valid, []byte(`"checks":[]`), []byte(`"checks":[1]`), 1),
	}
	for index, raw := range tests {
		if err := verifyResultIdentity(raw, testDigest, testDigest, "0.1-dev", "rejected"); err == nil {
			t.Fatalf("drift %d was accepted", index)
		}
	}
}

func TestExpectedProvenanceBindsReproductionInputs(t *testing.T) {
	raw := encodeExpectedProvenance(123, strings.TrimPrefix(testDigest, "sha256:"), testDigest, "0.1-dev")
	for _, required := range []string{
		`"build_count":"2"`, `"raw_bytes_identical":true`,
		`"toolchain_payload":{"byte_length":"64772572"`, `"version":"0.1-dev"`,
	} {
		if !bytes.Contains(raw, []byte(required)) {
			t.Fatalf("expected provenance lacks %s", required)
		}
	}
	if bytes.Contains(raw, []byte("/private/")) || bytes.Contains(raw, []byte("\n")) {
		t.Fatal("expected provenance contains ambient path or newline")
	}
}

func TestInspectArtifactRejectsNonGoPayload(t *testing.T) {
	path := filepath.Join(canonicalTestPath(t, t.TempDir()), "artifact")
	if err := os.WriteFile(path, []byte("not a Mach-O Go executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectArtifact(path); err == nil {
		t.Fatal("non-Go payload was accepted")
	}
}

func TestValidateBuildRootRequiresClosedInputs(t *testing.T) {
	root := canonicalTestPath(t, t.TempDir())
	for name, mode := range map[string]os.FileMode{
		provenanceName: 0o644,
		executableName: 0o755,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), mode); err != nil {
			t.Fatal(err)
		}
	}
	if err := validateBuildRoot(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "extra"), []byte("extra"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateBuildRoot(root); err == nil {
		t.Fatal("build root with extra input was accepted")
	}
}

func canonicalTestResult(outcome string) []byte {
	return []byte(`{"checker":{"artifact":"` + testDigest + `","name":"radishaxiom-independent-checker-go","source":"` + testDigest + `","toolchain":"go1.26.7","version":"0.1-dev"},"checks":[],"evidence":{},"missing_artifacts":[],"remaining_trust":[],"request":{},"result":{"kind":"` + outcome + `","refs":[]},"result_version":"0.1","tcb":[{"artifact":"` + testDigest + `","category":"canonicalization","version":"0.1-dev"},{"artifact":"` + testDigest + `","category":"checker-core","version":"0.1-dev"},{"artifact":"` + testDigest + `","category":"cryptographic-primitive","version":"0.1-dev"},{"artifact":"` + testDigest + `","category":"rule-interpreter","version":"0.1-dev"}]}`)
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
