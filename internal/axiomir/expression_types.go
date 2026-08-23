package axiomir

import (
	"math/big"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

func (p *parser) inferExpression(
	value strictjson.Value,
	environment []valueType,
	scope expressionScope,
) (valueType, error) {
	opValue, err := member(value, "op")
	if err != nil {
		return valueType{}, err
	}
	op, err := text(opValue)
	if err != nil {
		return valueType{}, err
	}
	switch op {
	case "literal_bool":
		return valueType{kind: boolValue}, nil
	case "literal_int":
		fields, err := object(value, "op", "type", "value")
		if err != nil {
			return valueType{}, err
		}
		return p.parseValueType(fields["type"])
	case "literal_text":
		return valueType{kind: textValue}, nil
	case "literal_enum":
		fields, err := object(value, "enum_type", "member", "op")
		if err != nil {
			return valueType{}, err
		}
		enumType, err := digest(fields["enum_type"])
		if err != nil {
			return valueType{}, err
		}
		return valueType{kind: enumValue, enumType: enumType}, nil
	case "bound":
		fields, err := object(value, "index", "op")
		if err != nil {
			return valueType{}, err
		}
		index, err := canonicalInteger(fields["index"], true)
		if err != nil {
			return valueType{}, err
		}
		if !index.IsUint64() || index.Cmp(new(big.Int).SetUint64(uint64(len(environment)))) >= 0 {
			return valueType{}, rejection.New(rejection.InvalidJSON, "bound index is outside the typed expression environment")
		}
		return environment[index.Uint64()], nil
	case "field":
		fields, err := object(value, "field", "op", "record")
		if err != nil {
			return valueType{}, err
		}
		recordType, err := p.inferExpression(fields["record"], environment, scope)
		if err != nil {
			return valueType{}, err
		}
		if recordType.kind != recordValue {
			return valueType{}, rejection.New(rejection.InvalidJSON, "field expression requires a record operand")
		}
		fieldName, err := name(fields["field"])
		if err != nil {
			return valueType{}, err
		}
		record, exists := p.records[recordType.recordType]
		if !exists {
			return valueType{}, rejection.New(rejection.InvalidJSON, "field expression record type does not resolve")
		}
		field, exists := record.fields[fieldName]
		if !exists {
			return valueType{}, rejection.New(rejection.InvalidJSON, "field expression name does not resolve in its record type")
		}
		return field.typeInfo, nil
	case "not":
		fields, err := object(value, "op", "value")
		if err != nil {
			return valueType{}, err
		}
		if err := p.requireExpressionType(fields["value"], environment, scope, valueType{kind: boolValue}, "not operand must be Bool"); err != nil {
			return valueType{}, err
		}
		return valueType{kind: boolValue}, nil
	case "and":
		fields, err := object(value, "op", "values")
		if err != nil {
			return valueType{}, err
		}
		values, err := array(fields["values"])
		if err != nil {
			return valueType{}, err
		}
		for _, child := range values {
			if err := p.requireExpressionType(child, environment, scope, valueType{kind: boolValue}, "and operands must be Bool"); err != nil {
				return valueType{}, err
			}
		}
		return valueType{kind: boolValue}, nil
	case "eq":
		fields, err := object(value, "left", "op", "right")
		if err != nil {
			return valueType{}, err
		}
		left, err := p.inferExpression(fields["left"], environment, scope)
		if err != nil {
			return valueType{}, err
		}
		right, err := p.inferExpression(fields["right"], environment, scope)
		if err != nil {
			return valueType{}, err
		}
		if !left.equal(right) || !left.equalityCompatible() {
			return valueType{}, rejection.New(rejection.InvalidJSON, "eq operands must have exactly the same equality-compatible type")
		}
		return valueType{kind: boolValue}, nil
	case "le":
		fields, err := object(value, "left", "op", "right")
		if err != nil {
			return valueType{}, err
		}
		left, err := p.inferExpression(fields["left"], environment, scope)
		if err != nil {
			return valueType{}, err
		}
		right, err := p.inferExpression(fields["right"], environment, scope)
		if err != nil {
			return valueType{}, err
		}
		if left.kind != intValue || !left.equal(right) {
			return valueType{}, rejection.New(rejection.InvalidJSON, "le operands must have exactly the same Int type")
		}
		return valueType{kind: boolValue}, nil
	case "int_add":
		fields, err := object(value, "op", "result_type", "values")
		if err != nil {
			return valueType{}, err
		}
		resultType, err := p.parseValueType(fields["result_type"])
		if err != nil {
			return valueType{}, err
		}
		values, err := array(fields["values"])
		if err != nil {
			return valueType{}, err
		}
		if err := p.requireSameIntOperands(values, environment, scope, "int_add operands must have exactly the same Int type"); err != nil {
			return valueType{}, err
		}
		return resultType, nil
	case "int_sub":
		fields, err := object(value, "left", "op", "result_type", "right")
		if err != nil {
			return valueType{}, err
		}
		resultType, err := p.parseValueType(fields["result_type"])
		if err != nil {
			return valueType{}, err
		}
		if err := p.requireSameIntOperands([]strictjson.Value{fields["left"], fields["right"]}, environment, scope, "int_sub operands must have exactly the same Int type"); err != nil {
			return valueType{}, err
		}
		return resultType, nil
	case "if":
		fields, err := object(value, "condition", "else", "op", "result_type", "then")
		if err != nil {
			return valueType{}, err
		}
		resultType, err := p.parseValueType(fields["result_type"])
		if err != nil {
			return valueType{}, err
		}
		if err := p.requireExpressionType(fields["condition"], environment, scope, valueType{kind: boolValue}, "if condition must be Bool"); err != nil {
			return valueType{}, err
		}
		for _, branch := range []string{"then", "else"} {
			if err := p.requireExpressionType(fields[branch], environment, scope, resultType, "if branch type must equal result_type"); err != nil {
				return valueType{}, err
			}
		}
		return resultType, nil
	case "match_option":
		fields, err := object(value, "none", "op", "result_type", "some", "subject")
		if err != nil {
			return valueType{}, err
		}
		resultType, err := p.parseValueType(fields["result_type"])
		if err != nil {
			return valueType{}, err
		}
		subjectType, err := p.inferExpression(fields["subject"], environment, scope)
		if err != nil {
			return valueType{}, err
		}
		if subjectType.kind != optionRecordValue {
			return valueType{}, rejection.New(rejection.InvalidJSON, "match_option subject must be Option<Record>")
		}
		if err := p.requireExpressionType(fields["none"], environment, scope, resultType, "match_option none branch type must equal result_type"); err != nil {
			return valueType{}, err
		}
		someEnvironment := prependType(valueType{kind: recordValue, recordType: subjectType.recordType}, environment)
		if err := p.requireExpressionType(fields["some"], someEnvironment, scope, resultType, "match_option some branch type must equal result_type"); err != nil {
			return valueType{}, err
		}
		return resultType, nil
	case "forall_rows":
		fields, err := object(value, "body", "op", "table")
		if err != nil {
			return valueType{}, err
		}
		table, err := p.resolveTableReference(fields["table"], scope)
		if err != nil {
			return valueType{}, err
		}
		rowEnvironment := prependType(valueType{kind: recordValue, recordType: table.recordType}, environment)
		if err := p.requireExpressionType(fields["body"], rowEnvironment, scope, valueType{kind: boolValue}, "forall_rows body must be Bool"); err != nil {
			return valueType{}, err
		}
		return valueType{kind: boolValue}, nil
	case "lookup":
		fields, err := object(value, "keys", "op", "table")
		if err != nil {
			return valueType{}, err
		}
		table, err := p.resolveTableReference(fields["table"], scope)
		if err != nil {
			return valueType{}, err
		}
		keys, err := array(fields["keys"])
		if err != nil {
			return valueType{}, err
		}
		if len(keys) != len(table.primaryKey) {
			return valueType{}, rejection.New(rejection.InvalidJSON, "lookup key count must equal the table primary-key arity")
		}
		record := p.records[table.recordType]
		for index, key := range keys {
			expected := record.fields[table.primaryKey[index]].typeInfo
			if err := p.requireExpressionType(key, environment, scope, expected, "lookup key type must equal its primary-key field type"); err != nil {
				return valueType{}, err
			}
		}
		return valueType{kind: optionRecordValue, recordType: table.recordType}, nil
	case "count_where":
		fields, err := object(value, "op", "predicate", "result_type", "table")
		if err != nil {
			return valueType{}, err
		}
		resultType, err := p.parseValueType(fields["result_type"])
		if err != nil {
			return valueType{}, err
		}
		table, err := p.resolveTableReference(fields["table"], scope)
		if err != nil {
			return valueType{}, err
		}
		rowEnvironment := prependType(valueType{kind: recordValue, recordType: table.recordType}, environment)
		if err := p.requireExpressionType(fields["predicate"], rowEnvironment, scope, valueType{kind: boolValue}, "count_where predicate must be Bool"); err != nil {
			return valueType{}, err
		}
		return resultType, nil
	case "sum_where":
		fields, err := object(value, "op", "predicate", "result_type", "table", "value")
		if err != nil {
			return valueType{}, err
		}
		resultType, err := p.parseValueType(fields["result_type"])
		if err != nil {
			return valueType{}, err
		}
		table, err := p.resolveTableReference(fields["table"], scope)
		if err != nil {
			return valueType{}, err
		}
		rowEnvironment := prependType(valueType{kind: recordValue, recordType: table.recordType}, environment)
		if err := p.requireExpressionType(fields["predicate"], rowEnvironment, scope, valueType{kind: boolValue}, "sum_where predicate must be Bool"); err != nil {
			return valueType{}, err
		}
		actualValueType, err := p.inferExpression(fields["value"], rowEnvironment, scope)
		if err != nil {
			return valueType{}, err
		}
		if actualValueType.kind != intValue {
			return valueType{}, rejection.New(rejection.InvalidJSON, "sum_where value must be Int")
		}
		return resultType, nil
	default:
		return valueType{}, rejection.New(rejection.UnknownTag, "expression op is outside the locked Axiom IR type profile")
	}
}

func (p *parser) requireExpressionType(
	value strictjson.Value,
	environment []valueType,
	scope expressionScope,
	expected valueType,
	detail string,
) error {
	actual, err := p.inferExpression(value, environment, scope)
	if err != nil {
		return err
	}
	if !actual.equal(expected) {
		return rejection.New(rejection.InvalidJSON, detail)
	}
	return nil
}

func (p *parser) requireSameIntOperands(
	values []strictjson.Value,
	environment []valueType,
	scope expressionScope,
	detail string,
) error {
	var first valueType
	for index, value := range values {
		actual, err := p.inferExpression(value, environment, scope)
		if err != nil {
			return err
		}
		if actual.kind != intValue || index != 0 && !actual.equal(first) {
			return rejection.New(rejection.InvalidJSON, detail)
		}
		first = actual
	}
	return nil
}

func prependType(value valueType, environment []valueType) []valueType {
	result := make([]valueType, 0, len(environment)+1)
	result = append(result, value)
	result = append(result, environment...)
	return result
}

func (p *parser) resolveTableReference(value strictjson.Value, scope expressionScope) (tableDefinition, error) {
	if err := p.parseTableReference(value, scope); err != nil {
		return tableDefinition{}, err
	}
	fields, err := object(value, "kind", "name")
	if err != nil {
		return tableDefinition{}, err
	}
	kind, err := text(fields["kind"])
	if err != nil {
		return tableDefinition{}, err
	}
	interfaceName, err := name(fields["name"])
	if err != nil {
		return tableDefinition{}, err
	}
	var nodeID protocol.Digest
	switch kind {
	case "input":
		nodeID = p.inputPorts[interfaceName]
	case "output":
		nodeID = p.outputNames[interfaceName]
	default:
		return tableDefinition{}, rejection.New(rejection.UnknownTag, "unknown contract table reference kind")
	}
	node, exists := p.nodes[nodeID]
	if !exists {
		return tableDefinition{}, rejection.New(rejection.InvalidJSON, "contract table reference node does not resolve")
	}
	table, exists := p.tables[node.tableType]
	if !exists {
		return tableDefinition{}, rejection.New(rejection.InvalidJSON, "contract table reference type does not resolve")
	}
	return table, nil
}

func (p *parser) validateNodeExpressionTypes() error {
	for _, nodeID := range p.nodeOrder {
		node := p.nodes[nodeID]
		var environment []valueType
		switch node.kind {
		case "input", "group":
			continue
		case "filter", "map":
			rowType, err := p.nodeRowType(node.predecessors[0])
			if err != nil {
				return err
			}
			environment = []valueType{rowType}
		case "lookup_join":
			left, err := p.nodeRowType(node.predecessors[0])
			if err != nil {
				return err
			}
			right, err := p.nodeRowType(node.predecessors[1])
			if err != nil {
				return err
			}
			environment = []valueType{left, right}
		default:
			return rejection.New(rejection.UnknownTag, "node kind is outside the locked Axiom IR type profile")
		}
		for _, check := range node.expressions {
			actual, err := p.inferExpression(check.value, environment, nodeExpression)
			if err != nil {
				return err
			}
			if check.requireBool && actual.kind != boolValue {
				return rejection.New(rejection.InvalidJSON, "filter predicate must be Bool")
			}
		}
	}
	return nil
}

func (p *parser) nodeRowType(nodeID protocol.Digest) (valueType, error) {
	node, exists := p.nodes[nodeID]
	if !exists {
		return valueType{}, rejection.New(rejection.InvalidJSON, "node expression predecessor does not resolve")
	}
	table, exists := p.tables[node.tableType]
	if !exists {
		return valueType{}, rejection.New(rejection.InvalidJSON, "node expression predecessor table type does not resolve")
	}
	return valueType{kind: recordValue, recordType: table.recordType}, nil
}
