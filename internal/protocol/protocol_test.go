package protocol

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

var envelopeLimits = strictjson.Limits{
	MaxBytes: 1 << 20,
	MaxDepth: 128,
	MaxItems: 10_000,
	MaxSteps: 1_000_000,
}

func TestParseImportedRequestAndManifest(t *testing.T) {
	requestBytes := fixture(t, "valid", "request.jcs")
	request, err := ParseRequest(requestBytes, envelopeLimits)
	if err != nil {
		t.Fatal(err)
	}
	if request.Version != "0.1" || request.CheckerProfile.Version != "0.1" {
		t.Fatalf("unexpected request identity: %#v", request)
	}
	if request.DomainDigest.String() != "sha256:2763823df8835c17dd3c3bdc19bbc70c3045cddc0edca9689b0af8d06e85f1ca" {
		t.Fatalf("unexpected request document domain digest: %s", request.DomainDigest)
	}
	if got, _ := request.Limit("bundle-bytes"); got != 4<<20 {
		t.Fatalf("unexpected bundle limit: %d", got)
	}

	manifestBytes := fixture(t, "valid", "manifest.jcs")
	manifest, err := ParseManifest(manifestBytes, envelopeLimits)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Artifacts) != 1 || !manifest.Artifacts[0].HasRole("evidence") {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
}

func TestImportedFixtureDigestsRemainLocked(t *testing.T) {
	tests := []struct {
		path string
		sha  string
	}{
		{"valid/request.jcs", "f11516fd4bfe9c835b8899813daa1cd6b2791e85214eb72e6c9868dcd042182b"},
		{"valid/manifest.jcs", "3d859d0d9fc84626f36b7dc8e553783d10abc5b08c0b19e19b161b7eb668607f"},
		{"valid/blobs/sha256/44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a", "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a"},
		{"negative/request-duplicate-member.invalid.jcs", "2aeca3debecfa45b2f4982b0e2d9e307a9d81bf5a6775aed5758ee89b7320108"},
		{"negative/request-missing-limit.invalid.jcs", "19f6f3e466e15a54bafad88228ec8fe5cb7b07545482a9616ef72e473a510c8f"},
		{"negative/request-noncanonical-whitespace.invalid.jcs", "1b888b78fa90e0c0f91ab0653381aeb4da542eb29cba4eb7d621f6a174c2811d"},
		{"negative/request-unknown-member.invalid.jcs", "1c34a41f828917fbdd70b8bcac1000bc04f487d8e3aec9bea27886b7db641b63"},
		{"negative/request-unknown-proof-support.invalid.jcs", "57dd235f73ba78634a0b92ed708b9ba8de5db37690a107e85b2f4d3c9cc52eb0"},
		{"negative/request-unknown-version.invalid.jcs", "0e4bce55b368538d8f4194b0dcd00331082610954acb42da215f0c9cf1f34d5c"},
		{"negative/request-unsorted-trust.invalid.jcs", "1371f764f429b96f433da5f227253cc7777637ab32b0795ceafa350b12a79730"},
		{"negative/manifest-duplicate-artifact.invalid.jcs", "5893432d3a0144e48e4e9e14855c5b3173497ad7c6c37f23130b490d576dbfa5"},
		{"negative/manifest-two-evidence.invalid.jcs", "f218412eba3b1402457b023031177b6f63fb1bd49da67b16c43230f1f62f6be7"},
		{"negative/manifest-unknown-role.invalid.jcs", "8cab275cea04bb47f0bf931f647db7f6c762b593d734fed58eacea581dd797a3"},
		{"negative/manifest-unsorted-artifacts.invalid.jcs", "fb144f3e59a6197025beadd6e4656fc723f526546da4032f779fdc6b5718b20d"},
	}
	for _, test := range tests {
		data := fixturePath(t, test.path)
		sum := sha256.Sum256(data)
		if got := protocolHex(sum[:]); got != test.sha {
			t.Fatalf("%s: expected %s, got %s", test.path, test.sha, got)
		}
	}
}

func TestImportedRequestRejections(t *testing.T) {
	tests := []struct {
		name string
		code rejection.Code
	}{
		{"request-duplicate-member.invalid.jcs", rejection.DuplicateMember},
		{"request-missing-limit.invalid.jcs", rejection.LimitSetMismatch},
		{"request-noncanonical-whitespace.invalid.jcs", rejection.NoncanonicalJSON},
		{"request-unknown-member.invalid.jcs", rejection.UnknownMember},
		{"request-unknown-proof-support.invalid.jcs", rejection.UnknownTag},
		{"request-unknown-version.invalid.jcs", rejection.UnsupportedVersion},
		{"request-unsorted-trust.invalid.jcs", rejection.NoncanonicalOrder},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseRequest(fixture(t, "negative", test.name), envelopeLimits)
			assertProtocolCode(t, err, test.code)
		})
	}
}

func TestImportedManifestRejections(t *testing.T) {
	tests := []struct {
		name string
		code rejection.Code
	}{
		{"manifest-duplicate-artifact.invalid.jcs", rejection.DuplicateArtifact},
		{"manifest-two-evidence.invalid.jcs", rejection.EvidenceCardinality},
		{"manifest-unknown-role.invalid.jcs", rejection.UnknownTag},
		{"manifest-unsorted-artifacts.invalid.jcs", rejection.NoncanonicalOrder},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseManifest(fixture(t, "negative", test.name), envelopeLimits)
			assertProtocolCode(t, err, test.code)
		})
	}
}

func TestParseDigestRejectsAliases(t *testing.T) {
	tests := []string{
		"44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
		"sha256:44136fa3",
		"sha256:44136FA355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
		"sha512:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
	}
	for _, value := range tests {
		if _, err := ParseDigest(value); err == nil {
			t.Fatalf("ParseDigest(%q) accepted an alias", value)
		}
	}
}

func fixture(t *testing.T, parts ...string) []byte {
	t.Helper()
	return fixturePath(t, filepath.Join(parts...))
}

func fixturePath(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "upstream", path))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func protocolHex(data []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(data)*2)
	for i, b := range data {
		out[i*2] = digits[b>>4]
		out[i*2+1] = digits[b&15]
	}
	return string(out)
}

func assertProtocolCode(t *testing.T, err error, want rejection.Code) {
	t.Helper()
	if got, ok := rejection.CodeOf(err); err == nil || !ok || got != want {
		t.Fatalf("expected %s, got %v", want, err)
	}
}
