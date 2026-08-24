package axiomevidence

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/bundle"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

func TestVerifyObligationCompletenessRejectsSemanticSetDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *Document)
	}{
		{
			name: "extra obligation",
			mutate: func(t *testing.T, document *Document) {
				definition := axiomir.ObligationDefinition{
					Expectation: "prove",
					Kind:        "field-origin",
					Subject: axiomir.ObligationSubject{
						Kind:      "field",
						Direction: "input",
						Interface: "orders",
						Name:      "order_id",
					},
				}
				insertDefinition(t, document, definition)
			},
		},
		{
			name: "expectation drift",
			mutate: func(t *testing.T, document *Document) {
				id, definition := findDefinition(t, document, func(candidate axiomir.ObligationDefinition) bool {
					return candidate.Kind == "field-origin"
				})
				delete(document.obligations, id)
				definition.Expectation = "check"
				insertDefinition(t, document, definition)
			},
		},
		{
			name: "path drift",
			mutate: func(t *testing.T, document *Document) {
				id, definition := findDefinition(t, document, func(candidate axiomir.ObligationDefinition) bool {
					return candidate.Subject.Kind == "node-path"
				})
				delete(document.obligations, id)
				definition.Subject.Path = append(definition.Subject.Path, "left")
				insertDefinition(t, document, definition)
			},
		},
		{
			name: "anchor drift",
			mutate: func(t *testing.T, document *Document) {
				id, definition := findDefinition(t, document, func(candidate axiomir.ObligationDefinition) bool {
					return candidate.Subject.Kind == "node"
				})
				delete(document.obligations, id)
				definition.Subject.ID = protocol.Digest{}
				insertDefinition(t, document, definition)
			},
		},
		{
			name: "duplicate anchor with conflicting expectation",
			mutate: func(t *testing.T, document *Document) {
				_, definition := findDefinition(t, document, func(candidate axiomir.ObligationDefinition) bool {
					return candidate.Kind == "numeric-range"
				})
				definition.Expectation = "check"
				insertDefinition(t, document, definition)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document, ir := completenessFixture(t, "ax-b01-correct")
			test.mutate(t, &document)
			assertCompletenessCode(t, document.VerifyObligationCompleteness(ir), rejection.ObligationMismatch)
		})
	}
}

func TestReconstructObligationsIsStableAcrossRepeatedTraversal(t *testing.T) {
	document, ir := completenessFixture(t, "ax-b03-correct")
	first, err := document.reconstructObligations(ir)
	if err != nil {
		t.Fatal(err)
	}
	want := sortedDefinitionIDs(first)
	for iteration := 0; iteration < 100; iteration++ {
		current, err := document.reconstructObligations(ir)
		if err != nil {
			t.Fatal(err)
		}
		got := sortedDefinitionIDs(current)
		if len(got) != len(want) {
			t.Fatalf("reconstruction cardinality changed: got %d, want %d", len(got), len(want))
		}
		for index := range want {
			if got[index] != want[index] {
				t.Fatalf("reconstruction identity changed at index %d", index)
			}
		}
	}
}

func completenessFixture(t *testing.T, scenario string) (Document, axiomir.Document) {
	t.Helper()
	root := filepath.Join("..", "bundle", "testdata", "upstream", "s", scenario)
	verified, err := bundle.Verify(filepath.Join(root, "bundle"))
	if err != nil {
		t.Fatal(err)
	}
	limits := completenessLimits(verified.Request)
	evidenceArtifact := findArtifact(t, verified.Manifest, "axiom-evidence")
	irArtifact := findArtifact(t, verified.Manifest, "axiom-ir")
	evidence, err := ParseStructure(readCompletenessBlob(t, root, evidenceArtifact.ContentDigest), limits)
	if err != nil {
		t.Fatal(err)
	}
	ir, err := axiomir.ParseStructure(readCompletenessBlob(t, root, irArtifact.ContentDigest), limits)
	if err != nil {
		t.Fatal(err)
	}
	return evidence, ir
}

func findArtifact(t *testing.T, manifest protocol.Manifest, format string) protocol.Artifact {
	t.Helper()
	var result protocol.Artifact
	found := false
	for _, artifact := range manifest.Artifacts {
		if artifact.Format != format || artifact.FormatVersion != "0.1" {
			continue
		}
		if found {
			t.Fatalf("multiple %s artifacts", format)
		}
		result = artifact
		found = true
	}
	if !found {
		t.Fatalf("missing %s artifact", format)
	}
	return result
}

func readCompletenessBlob(t *testing.T, root string, digest protocol.Digest) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "bundle", "blobs", "sha256", digest.BlobName()))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func completenessLimits(request protocol.Request) strictjson.Limits {
	bytes, _ := request.Limit("artifact-bytes")
	depth, _ := request.Limit("json-depth")
	items, _ := request.Limit("collection-items")
	steps, _ := request.Limit("semantic-steps")
	return strictjson.Limits{MaxBytes: bytes, MaxDepth: depth, MaxItems: items, MaxSteps: steps}
}

func findDefinition(
	t *testing.T,
	document *Document,
	match func(axiomir.ObligationDefinition) bool,
) (protocol.Digest, axiomir.ObligationDefinition) {
	t.Helper()
	for id, definition := range document.obligations {
		if match(definition) {
			return id, definition
		}
	}
	t.Fatal("matching obligation definition not found")
	return protocol.Digest{}, axiomir.ObligationDefinition{}
}

func insertDefinition(t *testing.T, document *Document, definition axiomir.ObligationDefinition) {
	t.Helper()
	encoded, err := encodeObligationDefinition(definition)
	if err != nil {
		t.Fatal(err)
	}
	document.obligations[domainDigest(domainObligation, encoded)] = definition
}

func sortedDefinitionIDs(definitions map[protocol.Digest]axiomir.ObligationDefinition) []string {
	result := make([]string, 0, len(definitions))
	for id := range definitions {
		result = append(result, id.String())
	}
	sort.Strings(result)
	return result
}

func assertCompletenessCode(t *testing.T, err error, want rejection.Code) {
	t.Helper()
	if got, ok := rejection.CodeOf(err); err == nil || !ok || got != want {
		t.Fatalf("expected %s, got %v", want, err)
	}
}
