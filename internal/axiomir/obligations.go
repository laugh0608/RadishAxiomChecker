package axiomir

import (
	"sort"
	"strconv"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

func (p *parser) reconstructObligationModel(
	documentDigest protocol.Digest,
) ([]ObligationDefinition, []string, []string, error) {
	definitions := []ObligationDefinition{
		definition("check", "ir-structure", ObligationSubject{
			Kind:             "document",
			IRDocumentDigest: documentDigest,
		}),
		definition("prove", "effect-empty", ObligationSubject{
			Kind:             "program",
			IRDocumentDigest: documentDigest,
		}),
	}

	for _, id := range p.nodeOrder {
		node := p.nodes[id]
		if node.kind == "input" {
			continue
		}
		definitions = append(definitions,
			definition("prove", "totality", idSubject("node", id)),
			definition("prove", "key-cardinality", idSubject("node", id)),
		)
		switch node.kind {
		case "filter", "map", "lookup_join":
			definitions = append(definitions, definition("prove", "row-coverage", idSubject("node", id)))
		case "group":
			definitions = append(definitions,
				definition("prove", "row-coverage", idSubject("node", id)),
				definition("prove", "group-conservation", idSubject("node", id)),
			)
			for index := range node.aggregates {
				definitions = append(definitions, definition("prove", "numeric-range", ObligationSubject{
					Kind: "node-path",
					ID:   id,
					Path: []string{"aggregates", strconv.Itoa(index)},
				}))
			}
		default:
			return nil, nil, nil, rejection.New(rejection.InvalidJSON, "cannot reconstruct obligations for an unknown node kind")
		}
		for _, path := range numericExpressionPaths(node.definition) {
			definitions = append(definitions, definition("prove", "numeric-range", ObligationSubject{
				Kind: "node-path",
				ID:   id,
				Path: path,
			}))
		}
	}

	for _, id := range p.contractOrder {
		contract := p.contracts[id]
		switch contract.kind {
		case "formula":
			if contract.role == "guarantee" {
				definitions = append(definitions, definition("prove", "contract-guarantee", idSubject("contract", id)))
			}
			for _, path := range numericExpressionPaths(contract.definition) {
				definitions = append(definitions, definition("prove", "numeric-range", ObligationSubject{
					Kind: "contract-path",
					ID:   id,
					Path: path,
				}))
			}
		case "noninterference":
			definitions = append(definitions, definition("prove", "noninterference", idSubject("contract", id)))
		default:
			return nil, nil, nil, rejection.New(rejection.InvalidJSON, "cannot reconstruct obligations for an unknown contract kind")
		}
	}

	inputInterfaces := make([]string, 0, len(p.inputPorts))
	for name := range p.inputPorts {
		inputInterfaces = append(inputInterfaces, name)
	}
	sort.Strings(inputInterfaces)

	outputInterfaces := make([]string, 0, len(p.outputNames))
	for name := range p.outputNames {
		outputInterfaces = append(outputInterfaces, name)
	}
	sort.Strings(outputInterfaces)
	for _, output := range outputInterfaces {
		node := p.nodes[p.outputNames[output]]
		table := p.tables[node.tableType]
		record, ok := p.records[table.recordType]
		if !ok {
			return nil, nil, nil, rejection.New(rejection.InvalidJSON, "output record type does not resolve during obligation reconstruction")
		}
		fields := make([]string, 0, len(record.fields))
		for name := range record.fields {
			fields = append(fields, name)
		}
		sort.Strings(fields)
		for _, field := range fields {
			definitions = append(definitions, definition("prove", "field-origin", ObligationSubject{
				Kind:      "field",
				Direction: "output",
				Interface: output,
				Name:      field,
			}))
		}
	}

	return definitions, inputInterfaces, outputInterfaces, nil
}

func definition(expectation, kind string, subject ObligationSubject) ObligationDefinition {
	return ObligationDefinition{Expectation: expectation, Kind: kind, Subject: subject}
}

func idSubject(kind string, id protocol.Digest) ObligationSubject {
	return ObligationSubject{Kind: kind, ID: id}
}

func numericExpressionPaths(value strictjson.Value) [][]string {
	var result [][]string
	var visit func(strictjson.Value, []string)
	visit = func(current strictjson.Value, path []string) {
		if members, ok := current.Members(); ok {
			for _, member := range members {
				if member.Name == "op" {
					if op, text := member.Value.Text(); text && numericExpressionOps[op] {
						result = append(result, append([]string(nil), path...))
					}
					break
				}
			}
			for _, member := range members {
				visit(member.Value, appendPath(path, member.Name))
			}
			return
		}
		if items, ok := current.Items(); ok {
			for index, item := range items {
				visit(item, appendPath(path, strconv.Itoa(index)))
			}
		}
	}
	visit(value, nil)
	return result
}

var numericExpressionOps = map[string]bool{
	"count_where": true,
	"int_add":     true,
	"int_sub":     true,
	"sum_where":   true,
}

func appendPath(path []string, element string) []string {
	result := make([]string, len(path), len(path)+1)
	copy(result, path)
	return append(result, element)
}
