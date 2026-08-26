package axiomir_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/bundle"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

var fixtureDocumentDigests = map[string]string{
	"sha256:04237985285e5005afe8c8dcd3af9141db7cf9e27340aeb217c572eb7275480f": "sha256:88e8e6be1748acbdf0491fcc95d4134772a87bc446221f325409f2cfba2a6f24",
	"sha256:048a9dc0a64d78199c1fed10f5fca6c4be7f14a28b4da871be9252fde5b419ff": "sha256:e78b481ab473e047c9a26978cf203104086baac60057485f4ae696cec641976b",
	"sha256:104487f51b802522dd33fd29933fee0d14829b48e83d032a663952e5e39df161": "sha256:f4c6520e0e536b1908e696008f3df3971fbda2687e1b8abafc0a645006018dd1",
	"sha256:6c6111650778602b0989424f58975aea4879fc648dd09f7519a0e04dc369d209": "sha256:319c4968cba20a67ba0433094dfbe7e3664b4b0b986decdfa96c8f9d05de1101",
	"sha256:862ac3ece8ba13d378adc6fd8a5ccc09a7f81d099730171006849b1fb3a94237": "sha256:9769d5fc550f34ff15cd7199bb9cdfcba8eb6ab71b848b6ebee60189eec05802",
	"sha256:8c6f651f433bcb6dcd879cf536875e60330fdbe398c0088e8115880872afc944": "sha256:e6ea5438be9616c71a5b32de5937ea7b0e95cc2dcaf79b886a1546a15fac8aff",
	"sha256:8d6ec839cf3cf795539e121c488b3bc4be84a33f24ac8c349d5ceebb29e559e7": "sha256:1fa8846fb3ba15937e3e4b5848e74d84d89050711086d7462eb16175510b4154",
	"sha256:95ea7ae8cf2b95e68c2b59c8c5f1729a4678409d008bef94dae0bcf262104be0": "sha256:7dedf589ed14d8b8507608f84100e3afef3a604bcf505aa5c18ca27eee094adf",
	"sha256:ac12631a3437d53f7282eb4be04d7f12390ba295df842a84d132c8b7dc68bd9d": "sha256:88fc732d2adae560dc714d2f098c2c181734f231bc64c8d00de29e0ecc987671",
	"sha256:c4c063c5b7ad9714b79f5ef3704d90c4e5dc53df90324724081dc34ee8d05900": "sha256:91bfa0b2e6cfa700b2228c224bd82750fe12102c5cb0a0c209d2fcfe30423db3",
	"sha256:e1d15f3b00dd061166044c9d67a461314055c2b44fc0b370606c9c7ae995e219": "sha256:66964758f77a0f398ebd193716657780424d59d7959cd409ace2516a50aef507",
	"sha256:f8be39c0b85266fa44c78b83a5bd2dc9b4ca939f2d449c28d3cb149927f9ca42": "sha256:a2521de9589f758de397e19bc1acfd39ff891fc6998af26e54af5208ee01ceb3",
}

func TestParseLockedTwentyEightBundleIRBoundary(t *testing.T) {
	root := filepath.Join("..", "bundle", "testdata", "upstream", "s")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 28 {
		t.Fatalf("expected 28 imported scenarios, got %d", len(entries))
	}
	rejections := map[string]rejection.Code{
		"chk-digest-01":   rejection.DigestMismatch,
		"chk-resource-01": rejection.ResourceLimit,
	}
	unique := make(map[protocol.Digest]struct{})
	parsed := 0
	for _, entry := range entries {
		scenario := entry.Name()
		t.Run(scenario, func(t *testing.T) {
			bundleRoot := filepath.Join(root, scenario, "bundle")
			verified, err := bundle.Verify(bundleRoot)
			if want, rejected := rejections[scenario]; rejected {
				assertCode(t, err, want)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "chk-bundle-01" {
				if len(verified.MissingArtifacts) != 1 {
					t.Fatalf("expected one retained missing artifact, got %v", verified.MissingArtifacts)
				}
			} else if len(verified.MissingArtifacts) != 0 {
				t.Fatalf("unexpected missing artifacts: %v", verified.MissingArtifacts)
			}
			artifact, ok := irArtifact(verified.Manifest)
			if !ok {
				t.Fatal("verified bundle has no unique Axiom IR artifact")
			}
			data, err := os.ReadFile(filepath.Join(bundleRoot, "blobs", "sha256", artifact.ContentDigest.BlobName()))
			if err != nil {
				t.Fatal(err)
			}
			document, err := axiomir.ParseStructure(data, limitsFrom(verified.Request))
			if err != nil {
				t.Fatal(err)
			}
			if document.ContentDigest != artifact.ContentDigest {
				t.Fatal("parser content digest differs from verified bundle identity")
			}
			expectedText, ok := fixtureDocumentDigests[artifact.ContentDigest.String()]
			if !ok {
				t.Fatal("Axiom IR content digest has no locked document-domain identity")
			}
			expected, err := protocol.ParseDigest(expectedText)
			if err != nil {
				t.Fatal(err)
			}
			if err := document.VerifyDomainDigest(expected); err != nil {
				t.Fatal(err)
			}
			unique[artifact.ContentDigest] = struct{}{}
			parsed++
		})
	}
	if parsed != 26 {
		t.Fatalf("expected 26 identity-valid IR scenarios, got %d", parsed)
	}
	if len(unique) != 12 {
		t.Fatalf("expected 12 unique identity-valid IR documents, got %d", len(unique))
	}
}

func TestParseStructureRejectsClosedBoundaryViolations(t *testing.T) {
	data := lockedB01(t)
	tests := []struct {
		name   string
		mutate func(*testing.T, []byte) []byte
		code   rejection.Code
	}{
		{
			"unknown member",
			func(t *testing.T, input []byte) []byte {
				return replaceOnce(t, input, []byte(`"effects":[]`), []byte(`"effectz":[]`))
			},
			rejection.UnknownMember,
		},
		{
			"unsupported version",
			func(t *testing.T, input []byte) []byte {
				return replaceOnce(t, input, []byte(`"ir_version":"0.1"`), []byte(`"ir_version":"0.2"`))
			},
			rejection.UnsupportedVersion,
		},
		{
			"unknown node tag",
			func(t *testing.T, input []byte) []byte {
				return replaceOnce(t, input, []byte(`"kind":"filter"`), []byte(`"kind":"falter"`))
			},
			rejection.UnknownTag,
		},
		{
			"noncanonical contract order",
			func(t *testing.T, input []byte) []byte {
				return swapFirstTwoObjects(t, input, "contracts")
			},
			rejection.NoncanonicalOrder,
		},
		{
			"definition domain digest mismatch",
			func(t *testing.T, input []byte) []byte {
				return mutateDigestAfter(t, input, 0, []byte(`"id":"sha256:`))
			},
			rejection.DigestMismatch,
		},
		{
			"dangling output node",
			func(t *testing.T, input []byte) []byte {
				start := bytes.Index(input, []byte(`"outputs":[`))
				if start < 0 {
					t.Fatal("outputs array not found")
				}
				return zeroDigestAfter(t, input, start, []byte(`"node":"sha256:`))
			},
			rejection.InvalidJSON,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := axiomir.ParseStructure(test.mutate(t, data), fixtureLimits())
			assertCode(t, err, test.code)
		})
	}
}

func TestVerifyDomainDigestRejectsMismatch(t *testing.T) {
	document, err := axiomir.ParseStructure(lockedB01(t), fixtureLimits())
	if err != nil {
		t.Fatal(err)
	}
	if err := document.VerifyDomainDigest(protocol.Digest{}); err == nil {
		t.Fatal("expected document domain digest mismatch")
	} else {
		assertCode(t, err, rejection.DigestMismatch)
	}
}

func irArtifact(manifest protocol.Manifest) (protocol.Artifact, bool) {
	var result protocol.Artifact
	found := false
	for _, artifact := range manifest.Artifacts {
		if artifact.Format != "axiom-ir" || artifact.FormatVersion != "0.1" {
			continue
		}
		if found {
			return protocol.Artifact{}, false
		}
		result = artifact
		found = true
	}
	return result, found
}

func limitsFrom(request protocol.Request) strictjson.Limits {
	artifactBytes, _ := request.Limit("artifact-bytes")
	depth, _ := request.Limit("json-depth")
	items, _ := request.Limit("collection-items")
	steps, _ := request.Limit("semantic-steps")
	return strictjson.Limits{MaxBytes: artifactBytes, MaxDepth: depth, MaxItems: items, MaxSteps: steps}
}

func fixtureLimits() strictjson.Limits {
	return strictjson.Limits{MaxBytes: 1 << 20, MaxDepth: 128, MaxItems: 10_000, MaxSteps: 1_000_000}
}

func lockedB01(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join(
		"..", "bundle", "testdata", "upstream", "s", "ax-b01-correct", "bundle", "blobs", "sha256",
		"8d6ec839cf3cf795539e121c488b3bc4be84a33f24ac8c349d5ceebb29e559e7",
	)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func replaceOnce(t *testing.T, input, old, replacement []byte) []byte {
	t.Helper()
	if bytes.Count(input, old) != 1 {
		t.Fatalf("expected one replacement target, got %d", bytes.Count(input, old))
	}
	return bytes.Replace(input, old, replacement, 1)
}

func mutateDigestAfter(t *testing.T, input []byte, start int, marker []byte) []byte {
	t.Helper()
	result := append([]byte(nil), input...)
	index := bytes.Index(result[start:], marker)
	if index < 0 {
		t.Fatal("digest marker not found")
	}
	index += start + len(marker)
	if result[index] == '0' {
		result[index] = '1'
	} else {
		result[index] = '0'
	}
	return result
}

func zeroDigestAfter(t *testing.T, input []byte, start int, marker []byte) []byte {
	t.Helper()
	result := append([]byte(nil), input...)
	index := bytes.Index(result[start:], marker)
	if index < 0 {
		t.Fatal("digest marker not found")
	}
	index += start + len(marker)
	copy(result[index:index+64], bytes.Repeat([]byte{'0'}, 64))
	return result
}

func swapFirstTwoObjects(t *testing.T, input []byte, arrayName string) []byte {
	t.Helper()
	marker := []byte(`"` + arrayName + `":[`)
	start := bytes.Index(input, marker)
	if start < 0 {
		t.Fatal("array marker not found")
	}
	firstStart := start + len(marker)
	firstEnd := objectEnd(t, input, firstStart)
	if firstEnd+1 >= len(input) || input[firstEnd+1] != ',' {
		t.Fatal("array does not contain two leading objects")
	}
	secondStart := firstEnd + 2
	secondEnd := objectEnd(t, input, secondStart)
	result := make([]byte, 0, len(input))
	result = append(result, input[:firstStart]...)
	result = append(result, input[secondStart:secondEnd+1]...)
	result = append(result, ',')
	result = append(result, input[firstStart:firstEnd+1]...)
	result = append(result, input[secondEnd+1:]...)
	return result
}

func objectEnd(t *testing.T, input []byte, start int) int {
	t.Helper()
	if start >= len(input) || input[start] != '{' {
		t.Fatal("object does not start at expected byte")
	}
	depth := 0
	inString := false
	escaped := false
	for index := start; index < len(input); index++ {
		b := input[index]
		if inString {
			if escaped {
				escaped = false
			} else if b == '\\' {
				escaped = true
			} else if b == '"' {
				inString = false
			}
			continue
		}
		if b == '"' {
			inString = true
			continue
		}
		switch b {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	t.Fatal("unterminated object")
	return -1
}

func assertCode(t *testing.T, err error, want rejection.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s, got nil", want)
	}
	got, ok := rejection.CodeOf(err)
	if !ok || got != want {
		t.Fatalf("expected %s, got %v", want, err)
	}
}
