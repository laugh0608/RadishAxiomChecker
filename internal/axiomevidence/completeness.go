package axiomevidence

import (
	"bytes"
	"encoding/json"
	"sort"

	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/resourcebudget"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

// VerifyObligationCompleteness independently reconstructs the locked v0.1
// obligation definition set from the parsed IR, Evidence profile, explicit
// benchmark execution boundary, and declared trust entries. It compares exact
// definitions and domain IDs. It does not judge result states or executions.
func (d Document) VerifyObligationCompleteness(ir axiomir.Document) error {
	return d.VerifyObligationCompletenessWithLedger(ir, nil)
}

func (d Document) VerifyObligationCompletenessWithLedger(
	ir axiomir.Document,
	ledger *resourcebudget.Ledger,
) error {
	if err := d.VerifyIRSubject(ir.ContentDigest, ir.DomainDigest); err != nil {
		return err
	}
	expected, err := d.reconstructObligations(ir)
	if err != nil {
		return err
	}
	if ledger != nil {
		if err := ledger.ChargeSemanticSteps(uint64(len(expected))); err != nil {
			return err
		}
	}
	if len(expected) != len(d.obligations) {
		return rejection.New(rejection.ObligationMismatch, "Axiom Evidence obligation set cardinality mismatch")
	}
	for id, definition := range expected {
		if ledger != nil {
			if err := ledger.ChargeSemanticSteps(1); err != nil {
				return err
			}
		}
		observed, ok := d.obligations[id]
		if !ok {
			return rejection.New(rejection.ObligationMismatch, "Axiom Evidence is missing an independently reconstructed obligation")
		}
		if !equalObligationDefinition(definition, observed) {
			return rejection.New(rejection.ObligationMismatch, "Axiom Evidence obligation definition mismatch")
		}
	}
	return nil
}

func (d Document) reconstructObligations(ir axiomir.Document) (map[protocol.Digest]axiomir.ObligationDefinition, error) {
	definitions := ir.StaticObligationDefinitions()

	trustIDs := make([]protocol.Digest, 0, len(d.trust))
	for id := range d.trust {
		trustIDs = append(trustIDs, id)
	}
	sortDigests(trustIDs)
	for _, id := range trustIDs {
		definitions = append(definitions, axiomir.ObligationDefinition{
			Expectation: "trust",
			Kind:        "trust-boundary",
			Subject: axiomir.ObligationSubject{
				Kind:     "trust",
				Category: d.trust[id].category,
				Scope:    id,
			},
		})
	}

	if d.obligationProfile == "keyed-finite-table-benchmark" {
		for _, name := range ir.InputInterfaces() {
			definitions = append(definitions, interfaceDefinition("input-conformance", "input", name))
		}

		inputArtifacts := make(map[protocol.Digest]struct{})
		hostOutputs := make(map[protocol.Digest]struct{})
		goldenOutputs := make(map[protocol.Digest]struct{})
		for _, execution := range d.executions {
			for _, input := range execution.inputs {
				switch input.role {
				case "host-input":
					inputArtifacts[input.artifact] = struct{}{}
				case "golden-output":
					goldenOutputs[input.artifact] = struct{}{}
				}
			}
			for _, output := range execution.outputs {
				if output.role == "host-output" {
					hostOutputs[output.artifact] = struct{}{}
				}
			}
		}
		definitions = appendArtifactDefinitions(definitions, "input-conformance", inputArtifacts)
		definitions = appendArtifactDefinitions(definitions, "host-conformance", hostOutputs)
		definitions = appendArtifactDefinitions(definitions, "output-conformance", goldenOutputs)
		if len(goldenOutputs) != 0 {
			for _, name := range ir.OutputInterfaces() {
				definitions = append(definitions, interfaceDefinition("output-conformance", "output", name))
			}
		}
	}

	result := make(map[protocol.Digest]axiomir.ObligationDefinition, len(definitions))
	for _, definition := range definitions {
		encoded, err := encodeObligationDefinition(definition)
		if err != nil {
			return nil, err
		}
		id := domainDigest(domainObligation, encoded)
		if _, duplicate := result[id]; duplicate {
			return nil, rejection.New(rejection.ObligationMismatch, "independent obligation reconstruction produced a duplicate definition")
		}
		result[id] = definition
	}
	return result, nil
}

func interfaceDefinition(kind, direction, name string) axiomir.ObligationDefinition {
	return axiomir.ObligationDefinition{
		Expectation: "check",
		Kind:        kind,
		Subject: axiomir.ObligationSubject{
			Kind:      "interface",
			Direction: direction,
			Name:      name,
		},
	}
}

func appendArtifactDefinitions(
	destination []axiomir.ObligationDefinition,
	kind string,
	artifacts map[protocol.Digest]struct{},
) []axiomir.ObligationDefinition {
	ids := make([]protocol.Digest, 0, len(artifacts))
	for id := range artifacts {
		ids = append(ids, id)
	}
	sortDigests(ids)
	for _, id := range ids {
		destination = append(destination, axiomir.ObligationDefinition{
			Expectation: "check",
			Kind:        kind,
			Subject:     axiomir.ObligationSubject{Kind: "artifact", Artifact: id},
		})
	}
	return destination
}

func sortDigests(values []protocol.Digest) {
	sort.Slice(values, func(left, right int) bool {
		return values[left].String() < values[right].String()
	})
}

func equalObligationDefinition(left, right axiomir.ObligationDefinition) bool {
	if left.Expectation != right.Expectation || left.Kind != right.Kind {
		return false
	}
	leftBytes, leftErr := encodeObligationDefinition(left)
	rightBytes, rightErr := encodeObligationDefinition(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftBytes, rightBytes)
}

type wireObligationDefinition struct {
	Expectation string `json:"expectation"`
	Kind        string `json:"kind"`
	Subject     any    `json:"subject"`
}

func encodeObligationDefinition(definition axiomir.ObligationDefinition) ([]byte, error) {
	var subject any
	switch definition.Subject.Kind {
	case "artifact":
		subject = struct {
			Artifact string `json:"artifact"`
			Kind     string `json:"kind"`
		}{definition.Subject.Artifact.String(), "artifact"}
	case "contract", "node":
		subject = struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		}{definition.Subject.ID.String(), definition.Subject.Kind}
	case "contract-path", "node-path":
		subject = struct {
			ID   string   `json:"id"`
			Kind string   `json:"kind"`
			Path []string `json:"path"`
		}{definition.Subject.ID.String(), definition.Subject.Kind, definition.Subject.Path}
	case "document", "program":
		subject = struct {
			IRDocumentDigest string `json:"ir_document_digest"`
			Kind             string `json:"kind"`
		}{definition.Subject.IRDocumentDigest.String(), definition.Subject.Kind}
	case "field":
		subject = struct {
			Direction string `json:"direction"`
			Interface string `json:"interface"`
			Kind      string `json:"kind"`
			Name      string `json:"name"`
		}{definition.Subject.Direction, definition.Subject.Interface, "field", definition.Subject.Name}
	case "interface":
		subject = struct {
			Direction string `json:"direction"`
			Kind      string `json:"kind"`
			Name      string `json:"name"`
		}{definition.Subject.Direction, "interface", definition.Subject.Name}
	case "trust":
		subject = struct {
			Category string `json:"category"`
			Kind     string `json:"kind"`
			Scope    string `json:"scope"`
		}{definition.Subject.Category, "trust", definition.Subject.Scope.String()}
	default:
		return nil, rejection.New(rejection.ObligationMismatch, "cannot encode an unknown reconstructed obligation subject")
	}
	raw, err := json.Marshal(wireObligationDefinition{
		Expectation: definition.Expectation,
		Kind:        definition.Kind,
		Subject:     subject,
	})
	if err != nil {
		return nil, rejection.New(rejection.ObligationMismatch, "cannot encode a reconstructed obligation definition")
	}
	value, err := strictjson.ParseCanonical(raw, strictjson.Limits{
		MaxBytes: 1 << 16,
		MaxDepth: 64,
		MaxItems: 1024,
		MaxSteps: 1 << 20,
	})
	if err != nil {
		return nil, rejection.New(rejection.ObligationMismatch, "reconstructed obligation is outside the canonical JSON profile")
	}
	return strictjson.CanonicalBytes(value)
}

func cloneExecutions(source map[protocol.Digest]executionDefinition) map[protocol.Digest]executionDefinition {
	result := make(map[protocol.Digest]executionDefinition, len(source))
	for id, definition := range source {
		definition.inputs = append([]executionIO(nil), definition.inputs...)
		definition.outputs = append([]executionIO(nil), definition.outputs...)
		result[id] = definition
	}
	return result
}

func cloneArtifacts(source map[protocol.Digest]artifactDefinition) map[protocol.Digest]artifactDefinition {
	result := make(map[protocol.Digest]artifactDefinition, len(source))
	for id, definition := range source {
		result[id] = definition
	}
	return result
}

func cloneTools(source map[protocol.Digest]toolDefinition) map[protocol.Digest]toolDefinition {
	result := make(map[protocol.Digest]toolDefinition, len(source))
	for id, definition := range source {
		roles := make(map[string]struct{}, len(definition.roles))
		for role := range definition.roles {
			roles[role] = struct{}{}
		}
		definition.roles = roles
		result[id] = definition
	}
	return result
}

func cloneResults(source map[protocol.Digest]obligationResult) map[protocol.Digest]obligationResult {
	result := make(map[protocol.Digest]obligationResult, len(source))
	for id, value := range source {
		value.artifacts = append([]protocol.Digest(nil), value.artifacts...)
		value.assumptions = append([]protocol.Digest(nil), value.assumptions...)
		value.attempts = append([]protocol.Digest(nil), value.attempts...)
		value.counterexample = cloneCounterexample(value.counterexample)
		result[id] = value
	}
	return result
}

func cloneCounterexample(source *counterexampleDefinition) *counterexampleDefinition {
	if source == nil {
		return nil
	}
	result := &counterexampleDefinition{
		kind:          source.kind,
		preconditions: append([]protocol.Digest(nil), source.preconditions...),
		trace:         append([]counterexampleTraceStep(nil), source.trace...),
		worlds:        make([]axiomir.ConcreteWorld, len(source.worlds)),
		observed: counterexampleObserved{
			kind:           source.observed.kind,
			obligation:     source.observed.obligation,
			requiredFields: append([]string(nil), source.observed.requiredFields...),
			requiredKeys:   append([]string(nil), source.observed.requiredKeys...),
			actual:         source.observed.actual,
			expected:       source.observed.expected,
		},
	}
	for worldIndex, world := range source.worlds {
		result.worlds[worldIndex].Tables = make([]axiomir.ConcreteTable, len(world.Tables))
		for tableIndex, table := range world.Tables {
			result.worlds[worldIndex].Tables[tableIndex].Name = table.Name
			result.worlds[worldIndex].Tables[tableIndex].Rows = make([]axiomir.ConcreteRecord, len(table.Rows))
			for rowIndex, row := range table.Rows {
				result.worlds[worldIndex].Tables[tableIndex].Rows[rowIndex] = axiomir.ConcreteRecord{
					RecordType: row.RecordType,
					Fields:     append([]axiomir.ConcreteField(nil), row.Fields...),
				}
			}
		}
	}
	return result
}

func cloneObligations(source map[protocol.Digest]axiomir.ObligationDefinition) map[protocol.Digest]axiomir.ObligationDefinition {
	result := make(map[protocol.Digest]axiomir.ObligationDefinition, len(source))
	for id, definition := range source {
		definition.Subject.Path = append([]string(nil), definition.Subject.Path...)
		result[id] = definition
	}
	return result
}

func cloneTrust(source map[protocol.Digest]trustDefinition) map[protocol.Digest]trustDefinition {
	result := make(map[protocol.Digest]trustDefinition, len(source))
	for id, definition := range source {
		result[id] = definition
	}
	return result
}
