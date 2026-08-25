package axiomir

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

func TestExecuteUsesTopologyInsteadOfSerializedNodeOrder(t *testing.T) {
	document, world := internalExecutionFixture(t, "ax-b01-correct")
	for left, right := 0, len(document.nodeOrder)-1; left < right; left, right = left+1, right-1 {
		document.nodeOrder[left], document.nodeOrder[right] = document.nodeOrder[right], document.nodeOrder[left]
	}
	if _, err := document.Execute(world, internalExecutionLimits()); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteRejectsRetainedInputAndPredecessorBindingDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Document)
	}{
		{
			name: "input port",
			mutate: func(document *Document) {
				for id, node := range document.nodes {
					if node.kind == "input" {
						node.port = "missing"
						document.nodes[id] = node
						return
					}
				}
			},
		},
		{
			name: "predecessor",
			mutate: func(document *Document) {
				for id, node := range document.nodes {
					if len(node.predecessors) != 0 {
						node.predecessors[0] = protocol.Digest{}
						document.nodes[id] = node
						return
					}
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document, world := internalExecutionFixture(t, "ax-b01-correct")
			test.mutate(&document)
			_, err := document.Execute(world, internalExecutionLimits())
			assertInternalCode(t, err, rejection.ConcreteCheckMismatch)
		})
	}
}

func TestExecuteRejectsProjectionAndAggregateDrift(t *testing.T) {
	document, world := internalExecutionFixture(t, "ax-b01-correct")
	for id, node := range document.nodes {
		if node.kind == "map" {
			node.expressions = node.expressions[:1]
			document.nodes[id] = node
			break
		}
	}
	_, err := document.Execute(world, internalExecutionLimits())
	var projectionFailure *ExecutionFailure
	if !errors.As(err, &projectionFailure) || projectionFailure.Kind != "table-wf" {
		t.Fatalf("expected projection table-wf failure, got %v", err)
	}

	document, world = internalExecutionFixture(t, "ax-b03-correct")
	for id, node := range document.nodes {
		if node.kind == "group" {
			for index := range node.aggregates {
				if node.aggregates[index].kind == "sum" {
					node.aggregates[index].field = "missing"
				}
			}
			document.nodes[id] = node
			break
		}
	}
	_, err = document.Execute(world, internalExecutionLimits())
	assertInternalCode(t, err, rejection.ConcreteCheckMismatch)
}

func TestExecuteFailsClosedOnArithmeticRange(t *testing.T) {
	document, world := internalExecutionFixture(t, "ax-b01-wrong-add")
	for tableIndex := range world.Tables {
		for rowIndex := range world.Tables[tableIndex].Rows {
			row := &world.Tables[tableIndex].Rows[rowIndex]
			for fieldIndex := range row.Fields {
				switch row.Fields[fieldIndex].Name {
				case "discount_cents", "subtotal_cents":
					row.Fields[fieldIndex].Value = ConcreteValue{Kind: "int", Integer: "100000"}
				case "state":
					value := row.Fields[fieldIndex].Value
					value.EnumMember = "settled"
					row.Fields[fieldIndex].Value = value
				}
			}
		}
	}
	_, err := document.Execute(world, internalExecutionLimits())
	var failure *ExecutionFailure
	if !errors.As(err, &failure) || failure.Kind != "numeric-range" {
		t.Fatalf("expected numeric-range execution failure, got %v", err)
	}
}

func internalExecutionFixture(t *testing.T, scenario string) (Document, ConcreteWorld) {
	t.Helper()
	root := filepath.Join("..", "bundle", "testdata", "upstream", "s", scenario, "bundle")
	manifestBytes, err := os.ReadFile(filepath.Join(root, "manifest.jcs"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Artifacts []struct {
			ContentDigest string `json:"content_digest"`
			Format        string `json:"format"`
			FormatVersion string `json:"format_version"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	limits := internalJSONLimits()
	var document Document
	for _, artifact := range manifest.Artifacts {
		if artifact.Format != "axiom-ir" || artifact.FormatVersion != "0.1" {
			continue
		}
		data := internalBlob(t, root, artifact.ContentDigest)
		document, err = ParseStructure(data, limits)
		if err != nil {
			t.Fatal(err)
		}
	}
	if document.ContentDigest == (protocol.Digest{}) {
		t.Fatal("fixture has no Axiom IR artifact")
	}
	for _, artifact := range manifest.Artifacts {
		if artifact.Format != "axiom-benchmark-data" || artifact.FormatVersion != "0.1" {
			continue
		}
		input, err := document.DecodeBenchmarkInput(internalBlob(t, root, artifact.ContentDigest), limits)
		if err != nil || input.Role != "input" || !document.CheckCompleteInputWorld(input.World).WellFormed {
			continue
		}
		pre, err := document.EvaluateAssumes(input.World, 1_000_000)
		if err == nil && pre.AllTrue && concreteWorldRows(input.World) != 0 {
			return document, input.World
		}
	}
	t.Fatal("fixture has no nonempty complete WF and Pre input")
	return Document{}, ConcreteWorld{}
}

func internalBlob(t *testing.T, root, digestText string) []byte {
	t.Helper()
	digest, err := protocol.ParseDigest(digestText)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "blobs", "sha256", digest.BlobName()))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func concreteWorldRows(world ConcreteWorld) int {
	result := 0
	for _, table := range world.Tables {
		result += len(table.Rows)
	}
	return result
}

func internalExecutionLimits() ExecutionLimits {
	return ExecutionLimits{MaxLogicalBytes: 1 << 20, MaxSemanticSteps: 1_000_000}
}

func internalJSONLimits() strictjson.Limits {
	return strictjson.Limits{MaxBytes: 1 << 20, MaxDepth: 128, MaxItems: 10_000, MaxSteps: 1_000_000}
}

func assertInternalCode(t *testing.T, err error, want rejection.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s, got nil", want)
	}
	got, ok := rejection.CodeOf(err)
	if !ok || got != want {
		t.Fatalf("expected %s, got %v", want, err)
	}
}
