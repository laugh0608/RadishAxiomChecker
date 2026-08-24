package axiomir

import (
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

func (p *parser) parseNodeDefinition(value strictjson.Value, id protocol.Digest) error {
	kindValue, err := member(value, "kind")
	if err != nil {
		return err
	}
	kind, err := text(kindValue)
	if err != nil {
		return err
	}
	definition := nodeDefinition{kind: kind, definition: value}
	switch kind {
	case "input":
		fields, err := object(value, "kind", "port", "table_type")
		if err != nil {
			return err
		}
		port, err := name(fields["port"])
		if err != nil {
			return err
		}
		if _, exists := p.inputPorts[port]; exists {
			return rejection.New(rejection.InvalidJSON, "input port names must be unique")
		}
		tableType, err := p.requireTableType(fields["table_type"])
		if err != nil {
			return err
		}
		definition.tableType = tableType
		p.inputPorts[port] = id
	case "filter":
		fields, err := object(value, "kind", "predicate", "source", "table_type")
		if err != nil {
			return err
		}
		source, err := digest(fields["source"])
		if err != nil {
			return err
		}
		definition.predecessors = []protocol.Digest{source}
		tableType, err := p.requireTableType(fields["table_type"])
		if err != nil {
			return err
		}
		definition.tableType = tableType
		if _, err := p.parseExpression(fields["predicate"], 1, nodeExpression); err != nil {
			return err
		}
		definition.expressions = []nodeExpressionCheck{{value: fields["predicate"], requireBool: true}}
	case "map":
		fields, err := object(value, "fields", "kind", "source", "table_type")
		if err != nil {
			return err
		}
		source, err := digest(fields["source"])
		if err != nil {
			return err
		}
		definition.predecessors = []protocol.Digest{source}
		tableType, err := p.requireTableType(fields["table_type"])
		if err != nil {
			return err
		}
		definition.tableType = tableType
		expressions, err := p.parseProjectionFields(fields["fields"], 1)
		if err != nil {
			return err
		}
		definition.expressions = expressions
	case "lookup_join":
		fields, err := object(value, "fields", "kind", "left", "pairs", "right", "table_type")
		if err != nil {
			return err
		}
		left, err := digest(fields["left"])
		if err != nil {
			return err
		}
		right, err := digest(fields["right"])
		if err != nil {
			return err
		}
		definition.predecessors = []protocol.Digest{left, right}
		tableType, err := p.requireTableType(fields["table_type"])
		if err != nil {
			return err
		}
		definition.tableType = tableType
		expressions, err := p.parseProjectionFields(fields["fields"], 2)
		if err != nil {
			return err
		}
		definition.expressions = expressions
		pairs, err := parseJoinPairs(fields["pairs"])
		if err != nil {
			return err
		}
		definition.joinPairs = pairs
	case "group":
		fields, err := object(value, "aggregates", "keys", "kind", "source", "table_type")
		if err != nil {
			return err
		}
		source, err := digest(fields["source"])
		if err != nil {
			return err
		}
		definition.predecessors = []protocol.Digest{source}
		tableType, err := p.requireTableType(fields["table_type"])
		if err != nil {
			return err
		}
		definition.tableType = tableType
		keys, err := parseGroupKeys(fields["keys"])
		if err != nil {
			return err
		}
		aggregates, err := parseAggregates(fields["aggregates"], keys)
		if err != nil {
			return err
		}
		definition.groupKeys = keys
		definition.aggregates = aggregates
	default:
		return rejection.New(rejection.UnknownTag, "node kind is outside the locked Axiom IR structure profile")
	}
	p.nodes[id] = definition
	p.nodeOrder = append(p.nodeOrder, id)
	return nil
}

func (p *parser) requireTableType(value strictjson.Value) (protocol.Digest, error) {
	tableType, err := digest(value)
	if err != nil {
		return protocol.Digest{}, err
	}
	if _, exists := p.tables[tableType]; !exists {
		return protocol.Digest{}, rejection.New(rejection.InvalidJSON, "node table type reference does not resolve")
	}
	return tableType, nil
}

func (p *parser) parseProjectionFields(value strictjson.Value, depth uint64) ([]nodeExpressionCheck, error) {
	items, err := array(value)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, rejection.New(rejection.InvalidJSON, "projection fields must not be empty")
	}
	expressions := make([]nodeExpressionCheck, 0, len(items))
	var previous string
	for index, item := range items {
		fields, err := object(item, "expression", "name")
		if err != nil {
			return nil, err
		}
		fieldName, err := name(fields["name"])
		if err != nil {
			return nil, err
		}
		if err := requireStrictOrder(previous, fieldName, index != 0, "projection fields are not sorted and unique by name"); err != nil {
			return nil, err
		}
		previous = fieldName
		if _, err := p.parseExpression(fields["expression"], depth, nodeExpression); err != nil {
			return nil, err
		}
		expressions = append(expressions, nodeExpressionCheck{fieldName: fieldName, value: fields["expression"]})
	}
	return expressions, nil
}

func parseJoinPairs(value strictjson.Value) ([]joinPair, error) {
	items, err := array(value)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, rejection.New(rejection.InvalidJSON, "lookup_join pairs must not be empty")
	}
	result := make([]joinPair, 0, len(items))
	var previous string
	for index, item := range items {
		fields, err := object(item, "left", "right")
		if err != nil {
			return nil, err
		}
		left, err := name(fields["left"])
		if err != nil {
			return nil, err
		}
		right, err := name(fields["right"])
		if err != nil {
			return nil, err
		}
		pair := left + "\x00" + right
		if err := requireStrictOrder(previous, pair, index != 0, "lookup_join pairs are not sorted and unique"); err != nil {
			return nil, err
		}
		previous = pair
		result = append(result, joinPair{left: left, right: right})
	}
	return result, nil
}

func parseGroupKeys(value strictjson.Value) ([]groupKey, error) {
	items, err := array(value)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, rejection.New(rejection.InvalidJSON, "group keys must not be empty")
	}
	result := make([]groupKey, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		fields, err := object(item, "name", "source_field")
		if err != nil {
			return nil, err
		}
		keyName, err := name(fields["name"])
		if err != nil {
			return nil, err
		}
		sourceField, err := name(fields["source_field"])
		if err != nil {
			return nil, err
		}
		if _, exists := seen[keyName]; exists {
			return nil, rejection.New(rejection.InvalidJSON, "group key names must be unique")
		}
		seen[keyName] = struct{}{}
		result = append(result, groupKey{name: keyName, sourceField: sourceField})
	}
	return result, nil
}

func parseAggregates(value strictjson.Value, keys []groupKey) ([]groupAggregate, error) {
	items, err := array(value)
	if err != nil {
		return nil, err
	}
	keyNames := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keyNames[key.name] = struct{}{}
	}
	result := make([]groupAggregate, 0, len(items))
	var previous string
	for index, item := range items {
		kindValue, err := member(item, "kind")
		if err != nil {
			return nil, err
		}
		kind, err := text(kindValue)
		if err != nil {
			return nil, err
		}
		var aggregate groupAggregate
		aggregate.kind = kind
		switch kind {
		case "count":
			fields, err := object(item, "kind", "name")
			if err != nil {
				return nil, err
			}
			aggregate.name, err = name(fields["name"])
			if err != nil {
				return nil, err
			}
		case "sum":
			fields, err := object(item, "field", "kind", "name")
			if err != nil {
				return nil, err
			}
			aggregate.field, err = name(fields["field"])
			if err != nil {
				return nil, err
			}
			aggregate.name, err = name(fields["name"])
			if err != nil {
				return nil, err
			}
		default:
			return nil, rejection.New(rejection.UnknownTag, "unknown group aggregate kind")
		}
		if err := requireStrictOrder(previous, aggregate.name, index != 0, "group aggregates are not sorted and unique by name"); err != nil {
			return nil, err
		}
		if _, exists := keyNames[aggregate.name]; exists {
			return nil, rejection.New(rejection.InvalidJSON, "group aggregate name conflicts with a key name")
		}
		previous = aggregate.name
		result = append(result, aggregate)
	}
	return result, nil
}

func (p *parser) parseOutputs(value strictjson.Value) (int, error) {
	items, err := array(value)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, rejection.New(rejection.InvalidJSON, "Axiom IR must contain at least one output")
	}
	var previous string
	for index, item := range items {
		fields, err := object(item, "name", "node")
		if err != nil {
			return 0, err
		}
		outputName, err := name(fields["name"])
		if err != nil {
			return 0, err
		}
		if err := requireStrictOrder(previous, outputName, index != 0, "outputs are not sorted and unique by name"); err != nil {
			return 0, err
		}
		previous = outputName
		node, err := digest(fields["node"])
		if err != nil {
			return 0, err
		}
		if _, exists := p.nodes[node]; !exists {
			return 0, rejection.New(rejection.InvalidJSON, "output node reference does not resolve")
		}
		p.outputNames[outputName] = node
	}
	return len(items), nil
}

func (p *parser) validateNodeGraph() error {
	if len(p.inputPorts) == 0 {
		return rejection.New(rejection.InvalidJSON, "Axiom IR must contain at least one input node")
	}
	for _, node := range p.nodes {
		for _, predecessor := range node.predecessors {
			if _, exists := p.nodes[predecessor]; !exists {
				return rejection.New(rejection.InvalidJSON, "node predecessor reference does not resolve")
			}
		}
	}
	colors := make(map[protocol.Digest]uint8, len(p.nodes))
	var visit func(protocol.Digest) error
	visit = func(id protocol.Digest) error {
		switch colors[id] {
		case 1:
			return rejection.New(rejection.InvalidJSON, "node graph contains a cycle")
		case 2:
			return nil
		}
		colors[id] = 1
		for _, predecessor := range p.nodes[id].predecessors {
			if err := visit(predecessor); err != nil {
				return err
			}
		}
		colors[id] = 2
		return nil
	}
	for id := range p.nodes {
		if err := visit(id); err != nil {
			return err
		}
	}
	reachable := make(map[protocol.Digest]struct{}, len(p.nodes))
	var mark func(protocol.Digest)
	mark = func(id protocol.Digest) {
		if _, seen := reachable[id]; seen {
			return
		}
		reachable[id] = struct{}{}
		for _, predecessor := range p.nodes[id].predecessors {
			mark(predecessor)
		}
	}
	for _, node := range p.outputNames {
		mark(node)
	}
	for id, node := range p.nodes {
		if node.kind == "input" {
			continue
		}
		if _, seen := reachable[id]; !seen {
			return rejection.New(rejection.InvalidJSON, "node graph contains a dead non-input node")
		}
	}
	return nil
}
