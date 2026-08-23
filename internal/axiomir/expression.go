package axiomir

import (
	"math/big"

	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

type expressionScope uint8

const (
	nodeExpression expressionScope = iota
	assumeExpression
	guaranteeExpression
)

func (p *parser) parseExpression(value strictjson.Value, depth uint64, scope expressionScope) (string, error) {
	opValue, err := member(value, "op")
	if err != nil {
		return "", err
	}
	op, err := text(opValue)
	if err != nil {
		return "", err
	}
	switch op {
	case "literal_bool":
		fields, err := object(value, "op", "value")
		if err != nil {
			return "", err
		}
		if _, err := boolean(fields["value"]); err != nil {
			return "", err
		}
	case "literal_int":
		fields, err := object(value, "op", "type", "value")
		if err != nil {
			return "", err
		}
		literalType, err := p.parseValueType(fields["type"])
		if err != nil {
			return "", err
		}
		if literalType.kind != intValue {
			return "", rejection.New(rejection.InvalidJSON, "literal_int requires an int type")
		}
		integer, err := canonicalInteger(fields["value"], false)
		if err != nil {
			return "", err
		}
		lower, ok := new(big.Int).SetString(literalType.lower, 10)
		if !ok {
			return "", rejection.New(rejection.InvalidJSON, "literal_int type lower bound cannot be decoded")
		}
		upper, ok := new(big.Int).SetString(literalType.upper, 10)
		if !ok {
			return "", rejection.New(rejection.InvalidJSON, "literal_int type upper bound cannot be decoded")
		}
		if integer.Cmp(lower) < 0 || integer.Cmp(upper) > 0 {
			return "", rejection.New(rejection.InvalidJSON, "literal_int value is outside its declared type")
		}
	case "literal_text":
		fields, err := object(value, "op", "value")
		if err != nil {
			return "", err
		}
		if _, err := text(fields["value"]); err != nil {
			return "", err
		}
	case "literal_enum":
		fields, err := object(value, "enum_type", "member", "op")
		if err != nil {
			return "", err
		}
		enumType, err := digest(fields["enum_type"])
		if err != nil {
			return "", err
		}
		definition, exists := p.enums[enumType]
		if !exists {
			return "", rejection.New(rejection.InvalidJSON, "literal_enum type reference does not resolve")
		}
		memberName, err := name(fields["member"])
		if err != nil {
			return "", err
		}
		if _, exists := definition.members[memberName]; !exists {
			return "", rejection.New(rejection.InvalidJSON, "literal_enum member does not resolve")
		}
	case "bound":
		fields, err := object(value, "index", "op")
		if err != nil {
			return "", err
		}
		index, err := canonicalInteger(fields["index"], true)
		if err != nil {
			return "", err
		}
		if index.Cmp(new(big.Int).SetUint64(depth)) >= 0 {
			return "", rejection.New(rejection.InvalidJSON, "bound index is outside the expression environment")
		}
	case "field":
		fields, err := object(value, "field", "op", "record")
		if err != nil {
			return "", err
		}
		if _, err := name(fields["field"]); err != nil {
			return "", err
		}
		if _, err := p.parseExpression(fields["record"], depth, scope); err != nil {
			return "", err
		}
	case "not":
		fields, err := object(value, "op", "value")
		if err != nil {
			return "", err
		}
		if _, err := p.parseExpression(fields["value"], depth, scope); err != nil {
			return "", err
		}
	case "and":
		fields, err := object(value, "op", "values")
		if err != nil {
			return "", err
		}
		values, err := array(fields["values"])
		if err != nil {
			return "", err
		}
		if len(values) < 2 {
			return "", rejection.New(rejection.InvalidJSON, "and requires at least two normalized values")
		}
		if err := requireCanonicalOrder(values, true, "and values are not sorted and unique by canonical bytes"); err != nil {
			return "", err
		}
		for _, child := range values {
			childOp, err := p.parseExpression(child, depth, scope)
			if err != nil {
				return "", err
			}
			if childOp == "and" {
				return "", rejection.New(rejection.NoncanonicalOrder, "nested and expression was not flattened")
			}
		}
	case "eq":
		fields, err := object(value, "left", "op", "right")
		if err != nil {
			return "", err
		}
		if err := requireCanonicalOrder([]strictjson.Value{fields["left"], fields["right"]}, false, "eq operands are not sorted by canonical bytes"); err != nil {
			return "", err
		}
		if _, err := p.parseExpression(fields["left"], depth, scope); err != nil {
			return "", err
		}
		if _, err := p.parseExpression(fields["right"], depth, scope); err != nil {
			return "", err
		}
	case "le":
		fields, err := object(value, "left", "op", "right")
		if err != nil {
			return "", err
		}
		if _, err := p.parseExpression(fields["left"], depth, scope); err != nil {
			return "", err
		}
		if _, err := p.parseExpression(fields["right"], depth, scope); err != nil {
			return "", err
		}
	case "int_add":
		fields, err := object(value, "op", "result_type", "values")
		if err != nil {
			return "", err
		}
		if err := p.parseResultType(fields["result_type"], intValue); err != nil {
			return "", err
		}
		values, err := array(fields["values"])
		if err != nil {
			return "", err
		}
		if len(values) != 2 {
			return "", rejection.New(rejection.InvalidJSON, "int_add requires exactly two values")
		}
		if err := requireCanonicalOrder(values, false, "int_add values are not sorted by canonical bytes"); err != nil {
			return "", err
		}
		if err := p.parseExpressions(values, depth, scope); err != nil {
			return "", err
		}
	case "int_sub":
		fields, err := object(value, "left", "op", "result_type", "right")
		if err != nil {
			return "", err
		}
		if err := p.parseResultType(fields["result_type"], intValue); err != nil {
			return "", err
		}
		if _, err := p.parseExpression(fields["left"], depth, scope); err != nil {
			return "", err
		}
		if _, err := p.parseExpression(fields["right"], depth, scope); err != nil {
			return "", err
		}
	case "if":
		fields, err := object(value, "condition", "else", "op", "result_type", "then")
		if err != nil {
			return "", err
		}
		if _, err := p.parseValueType(fields["result_type"]); err != nil {
			return "", err
		}
		for _, field := range []string{"condition", "else", "then"} {
			if _, err := p.parseExpression(fields[field], depth, scope); err != nil {
				return "", err
			}
		}
	case "match_option":
		fields, err := object(value, "none", "op", "result_type", "some", "subject")
		if err != nil {
			return "", err
		}
		if _, err := p.parseValueType(fields["result_type"]); err != nil {
			return "", err
		}
		if _, err := p.parseExpression(fields["subject"], depth, scope); err != nil {
			return "", err
		}
		if _, err := p.parseExpression(fields["none"], depth, scope); err != nil {
			return "", err
		}
		if _, err := p.parseExpression(fields["some"], depth+1, scope); err != nil {
			return "", err
		}
	case "forall_rows":
		fields, err := object(value, "body", "op", "table")
		if err != nil {
			return "", err
		}
		if err := p.parseTableReference(fields["table"], scope); err != nil {
			return "", err
		}
		if _, err := p.parseExpression(fields["body"], depth+1, scope); err != nil {
			return "", err
		}
	case "lookup":
		fields, err := object(value, "keys", "op", "table")
		if err != nil {
			return "", err
		}
		if err := p.parseTableReference(fields["table"], scope); err != nil {
			return "", err
		}
		keys, err := array(fields["keys"])
		if err != nil {
			return "", err
		}
		if len(keys) == 0 {
			return "", rejection.New(rejection.InvalidJSON, "lookup keys must not be empty")
		}
		if err := p.parseExpressions(keys, depth, scope); err != nil {
			return "", err
		}
	case "count_where":
		fields, err := object(value, "op", "predicate", "result_type", "table")
		if err != nil {
			return "", err
		}
		if err := p.parseResultType(fields["result_type"], intValue); err != nil {
			return "", err
		}
		if err := p.parseTableReference(fields["table"], scope); err != nil {
			return "", err
		}
		if _, err := p.parseExpression(fields["predicate"], depth+1, scope); err != nil {
			return "", err
		}
	case "sum_where":
		fields, err := object(value, "op", "predicate", "result_type", "table", "value")
		if err != nil {
			return "", err
		}
		if err := p.parseResultType(fields["result_type"], intValue); err != nil {
			return "", err
		}
		if err := p.parseTableReference(fields["table"], scope); err != nil {
			return "", err
		}
		if _, err := p.parseExpression(fields["predicate"], depth+1, scope); err != nil {
			return "", err
		}
		if _, err := p.parseExpression(fields["value"], depth+1, scope); err != nil {
			return "", err
		}
	default:
		return "", rejection.New(rejection.UnknownTag, "expression op is outside the locked Axiom IR structure profile")
	}
	return op, nil
}

func (p *parser) parseExpressions(values []strictjson.Value, depth uint64, scope expressionScope) error {
	for _, value := range values {
		if _, err := p.parseExpression(value, depth, scope); err != nil {
			return err
		}
	}
	return nil
}

func (p *parser) parseResultType(value strictjson.Value, expected valueKind) error {
	resultType, err := p.parseValueType(value)
	if err != nil {
		return err
	}
	if resultType.kind != expected {
		return rejection.New(rejection.InvalidJSON, "expression result type has the wrong kind")
	}
	return nil
}

func (p *parser) parseTableReference(value strictjson.Value, scope expressionScope) error {
	if scope == nodeExpression {
		return rejection.New(rejection.UnknownTag, "table expression is not permitted in a node expression")
	}
	fields, err := object(value, "kind", "name")
	if err != nil {
		return err
	}
	kind, err := text(fields["kind"])
	if err != nil {
		return err
	}
	interfaceName, err := name(fields["name"])
	if err != nil {
		return err
	}
	switch kind {
	case "input":
		if _, exists := p.inputPorts[interfaceName]; !exists {
			return rejection.New(rejection.InvalidJSON, "contract input interface reference does not resolve")
		}
	case "output":
		if scope == assumeExpression {
			return rejection.New(rejection.InvalidJSON, "assume contract cannot reference an output interface")
		}
		if _, exists := p.outputNames[interfaceName]; !exists {
			return rejection.New(rejection.InvalidJSON, "contract output interface reference does not resolve")
		}
	default:
		return rejection.New(rejection.UnknownTag, "unknown contract table reference kind")
	}
	return nil
}
