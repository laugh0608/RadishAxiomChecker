package axiomir

import (
	"math/big"

	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

func (p *parser) validateNodeTableRelationships() error {
	for _, nodeID := range p.nodeOrder {
		node := p.nodes[nodeID]
		output := p.tables[node.tableType]
		switch node.kind {
		case "input":
			continue
		case "filter":
			source := p.tables[p.nodes[node.predecessors[0]].tableType]
			if output.recordType != source.recordType || !sameNames(output.primaryKey, source.primaryKey) {
				return rejection.New(rejection.InvalidJSON, "filter output record and primary key must equal its source")
			}
			comparison, err := compareCapacities(output.capacity, source.capacity)
			if err != nil {
				return err
			}
			if comparison > 0 {
				return rejection.New(rejection.InvalidJSON, "filter output capacity must not exceed its source")
			}
		case "map":
			source := p.tables[p.nodes[node.predecessors[0]].tableType]
			if output.capacity != source.capacity {
				return rejection.New(rejection.InvalidJSON, "map output capacity must equal its source")
			}
			environment := []valueType{{kind: recordValue, recordType: source.recordType}}
			if err := p.validateProjection(node, output, environment); err != nil {
				return err
			}
			if err := p.validatePreservedPrimaryKey(node, output, source, 0, "map output primary key must preserve or rename its source primary key"); err != nil {
				return err
			}
		case "lookup_join":
			left := p.tables[p.nodes[node.predecessors[0]].tableType]
			right := p.tables[p.nodes[node.predecessors[1]].tableType]
			if output.capacity != left.capacity {
				return rejection.New(rejection.InvalidJSON, "lookup_join output capacity must equal its left source")
			}
			if err := p.validateJoinPairs(node, left, right); err != nil {
				return err
			}
			environment := []valueType{
				{kind: recordValue, recordType: left.recordType},
				{kind: recordValue, recordType: right.recordType},
			}
			if err := p.validateProjection(node, output, environment); err != nil {
				return err
			}
			if err := p.validatePreservedPrimaryKey(node, output, left, 0, "lookup_join output primary key must preserve or rename its left primary key"); err != nil {
				return err
			}
		case "group":
			source := p.tables[p.nodes[node.predecessors[0]].tableType]
			if err := p.validateGroup(node, output, source); err != nil {
				return err
			}
		default:
			return rejection.New(rejection.UnknownTag, "node kind is outside the locked Axiom IR table relationship profile")
		}
	}
	return nil
}

func (p *parser) validateProjection(
	node nodeDefinition,
	output tableDefinition,
	environment []valueType,
) error {
	record, exists := p.records[output.recordType]
	if !exists {
		return rejection.New(rejection.InvalidJSON, "projection output record type does not resolve")
	}
	if len(node.expressions) != len(record.fields) {
		return rejection.New(rejection.InvalidJSON, "projection fields must exactly cover the output record")
	}
	for _, projection := range node.expressions {
		field, exists := record.fields[projection.fieldName]
		if !exists {
			return rejection.New(rejection.InvalidJSON, "projection field does not exist in the output record")
		}
		actual, err := p.inferExpression(projection.value, environment, nodeExpression)
		if err != nil {
			return err
		}
		if !actual.equal(field.typeInfo) {
			return rejection.New(rejection.InvalidJSON, "projection expression type must equal its output field type")
		}
	}
	return nil
}

func (p *parser) validatePreservedPrimaryKey(
	node nodeDefinition,
	output tableDefinition,
	source tableDefinition,
	boundIndex uint64,
	detail string,
) error {
	if len(output.primaryKey) != len(source.primaryKey) {
		return rejection.New(rejection.InvalidJSON, detail)
	}
	for index, outputName := range output.primaryKey {
		projection, exists := projectionByName(node.expressions, outputName)
		if !exists {
			return rejection.New(rejection.InvalidJSON, detail)
		}
		sourceName, direct, err := directBoundField(projection.value, boundIndex)
		if err != nil {
			return err
		}
		if !direct || sourceName != source.primaryKey[index] {
			return rejection.New(rejection.InvalidJSON, detail)
		}
	}
	return nil
}

func (p *parser) validateJoinPairs(node nodeDefinition, left, right tableDefinition) error {
	leftRecord := p.records[left.recordType]
	rightRecord := p.records[right.recordType]
	for _, pair := range node.joinPairs {
		leftField, leftExists := leftRecord.fields[pair.left]
		rightField, rightExists := rightRecord.fields[pair.right]
		if !leftExists || !rightExists {
			return rejection.New(rejection.InvalidJSON, "lookup_join pair fields must exist in their respective records")
		}
		if !leftField.typeInfo.equal(rightField.typeInfo) {
			return rejection.New(rejection.InvalidJSON, "lookup_join pair fields must have exactly the same type")
		}
	}
	return nil
}

func (p *parser) validateGroup(node nodeDefinition, output, source tableDefinition) error {
	comparison, err := compareCapacities(output.capacity, source.capacity)
	if err != nil {
		return err
	}
	if comparison > 0 {
		return rejection.New(rejection.InvalidJSON, "group output capacity must not exceed its source")
	}
	outputRecord := p.records[output.recordType]
	sourceRecord := p.records[source.recordType]
	if len(node.groupKeys)+len(node.aggregates) != len(outputRecord.fields) {
		return rejection.New(rejection.InvalidJSON, "group keys and aggregates must exactly cover the output record")
	}
	if len(node.groupKeys) != len(output.primaryKey) {
		return rejection.New(rejection.InvalidJSON, "group keys must equal the output primary key")
	}
	for index, key := range node.groupKeys {
		if key.name != output.primaryKey[index] {
			return rejection.New(rejection.InvalidJSON, "group key order must equal the output primary-key order")
		}
		sourceField, sourceExists := sourceRecord.fields[key.sourceField]
		outputField, outputExists := outputRecord.fields[key.name]
		if !sourceExists || !outputExists {
			return rejection.New(rejection.InvalidJSON, "group key fields must exist in their respective records")
		}
		if sourceField.label != publicLabel || !sourceField.typeInfo.keyCompatible() {
			return rejection.New(rejection.InvalidJSON, "group source key field must be public and key-compatible")
		}
		if !sourceField.typeInfo.equal(outputField.typeInfo) {
			return rejection.New(rejection.InvalidJSON, "group output key type must equal its source field type")
		}
	}
	for _, aggregate := range node.aggregates {
		outputField, exists := outputRecord.fields[aggregate.name]
		if !exists {
			return rejection.New(rejection.InvalidJSON, "group aggregate field must exist in the output record")
		}
		switch aggregate.kind {
		case "count":
			expected := valueType{kind: intValue, lower: "0", upper: source.capacity}
			if !outputField.typeInfo.equal(expected) {
				return rejection.New(rejection.InvalidJSON, "group count output type must be Int[0, source capacity]")
			}
		case "sum":
			sourceField, exists := sourceRecord.fields[aggregate.field]
			if !exists {
				return rejection.New(rejection.InvalidJSON, "group sum source field does not exist")
			}
			if sourceField.typeInfo.kind != intValue || outputField.typeInfo.kind != intValue {
				return rejection.New(rejection.InvalidJSON, "group sum source and output fields must be Int in the locked profile")
			}
		default:
			return rejection.New(rejection.UnknownTag, "unknown group aggregate kind")
		}
	}
	return nil
}

func projectionByName(values []nodeExpressionCheck, fieldName string) (nodeExpressionCheck, bool) {
	for _, value := range values {
		if value.fieldName == fieldName {
			return value, true
		}
	}
	return nodeExpressionCheck{}, false
}

func directBoundField(value strictjson.Value, expectedIndex uint64) (string, bool, error) {
	opValue, err := member(value, "op")
	if err != nil {
		return "", false, err
	}
	op, err := text(opValue)
	if err != nil {
		return "", false, err
	}
	if op != "field" {
		return "", false, nil
	}
	fields, err := object(value, "field", "op", "record")
	if err != nil {
		return "", false, err
	}
	recordOpValue, err := member(fields["record"], "op")
	if err != nil {
		return "", false, err
	}
	recordOp, err := text(recordOpValue)
	if err != nil {
		return "", false, err
	}
	if recordOp != "bound" {
		return "", false, nil
	}
	bound, err := object(fields["record"], "index", "op")
	if err != nil {
		return "", false, err
	}
	index, err := canonicalInteger(bound["index"], true)
	if err != nil {
		return "", false, err
	}
	if !index.IsUint64() || index.Uint64() != expectedIndex {
		return "", false, nil
	}
	fieldName, err := name(fields["field"])
	if err != nil {
		return "", false, err
	}
	return fieldName, true, nil
}

func sameNames(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func compareCapacities(left, right string) (int, error) {
	leftInteger, ok := new(big.Int).SetString(left, 10)
	if !ok {
		return 0, rejection.New(rejection.InvalidJSON, "left table capacity cannot be decoded")
	}
	rightInteger, ok := new(big.Int).SetString(right, 10)
	if !ok {
		return 0, rejection.New(rejection.InvalidJSON, "right table capacity cannot be decoded")
	}
	return leftInteger.Cmp(rightInteger), nil
}
