package axiomir

import (
	"math/big"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

// AssumeEvaluation records which retained assume contracts evaluated false.
// It is a dynamic result for one concrete input, not a proof.
type AssumeEvaluation struct {
	AllTrue bool
	Failed  []protocol.Digest
}

type evaluationValue struct {
	kind       valueKind
	truth      bool
	integer    *big.Int
	text       string
	enumType   protocol.Digest
	enumMember string
	record     *ConcreteRecord
	some       bool
}

type inputEvaluator struct {
	document Document
	world    ConcreteWorld
	steps    uint64
	maxSteps uint64
}

// EvaluateAssumes evaluates the locked input-only assume subset independently
// from production execution. The current subset covers scalar literals,
// De Bruijn bound records, fields, Bool connectives, exact equality, Int <=,
// if, forall_rows, primary-key lookup, and match_option.
func (document Document) EvaluateAssumes(world ConcreteWorld, maxSteps uint64) (AssumeEvaluation, error) {
	evaluator := inputEvaluator{document: document, world: world, maxSteps: maxSteps}
	result := AssumeEvaluation{AllTrue: true}
	for _, id := range document.assumeContracts {
		contract, exists := document.contracts[id]
		if !exists || contract.kind != "formula" || contract.role != "assume" {
			return AssumeEvaluation{}, rejection.New(rejection.ConcreteCheckMismatch, "retained assume contract is unavailable")
		}
		fields, err := object(contract.definition, "expression", "kind", "role")
		if err != nil {
			return AssumeEvaluation{}, err
		}
		value, err := evaluator.evaluate(fields["expression"], nil)
		if err != nil {
			return AssumeEvaluation{}, err
		}
		if value.kind != boolValue {
			return AssumeEvaluation{}, rejection.New(rejection.ConcreteCheckMismatch, "assume evaluation did not produce Bool")
		}
		if !value.truth {
			result.AllTrue = false
			result.Failed = append(result.Failed, id)
		}
	}
	return result, nil
}

func (evaluator *inputEvaluator) evaluate(
	value strictjson.Value,
	environment []evaluationValue,
) (evaluationValue, error) {
	if err := evaluator.step(); err != nil {
		return evaluationValue{}, err
	}
	opValue, err := member(value, "op")
	if err != nil {
		return evaluationValue{}, err
	}
	op, err := text(opValue)
	if err != nil {
		return evaluationValue{}, err
	}
	switch op {
	case "literal_bool":
		fields, err := object(value, "op", "value")
		if err != nil {
			return evaluationValue{}, err
		}
		truth, err := boolean(fields["value"])
		return evaluationValue{kind: boolValue, truth: truth}, err
	case "literal_int":
		fields, err := object(value, "op", "type", "value")
		if err != nil {
			return evaluationValue{}, err
		}
		integer, err := canonicalInteger(fields["value"], false)
		return evaluationValue{kind: intValue, integer: integer}, err
	case "literal_text":
		fields, err := object(value, "op", "value")
		if err != nil {
			return evaluationValue{}, err
		}
		spelling, err := text(fields["value"])
		return evaluationValue{kind: textValue, text: spelling}, err
	case "literal_enum":
		fields, err := object(value, "enum_type", "member", "op")
		if err != nil {
			return evaluationValue{}, err
		}
		enumType, err := digest(fields["enum_type"])
		if err != nil {
			return evaluationValue{}, err
		}
		member, err := text(fields["member"])
		return evaluationValue{kind: enumValue, enumType: enumType, enumMember: member}, err
	case "bound":
		fields, err := object(value, "index", "op")
		if err != nil {
			return evaluationValue{}, err
		}
		index, err := canonicalInteger(fields["index"], true)
		if err != nil {
			return evaluationValue{}, err
		}
		if !index.IsUint64() || index.Uint64() >= uint64(len(environment)) {
			return evaluationValue{}, rejection.New(rejection.ConcreteCheckMismatch, "bound index is outside the concrete evaluation environment")
		}
		return environment[index.Uint64()], nil
	case "field":
		fields, err := object(value, "field", "op", "record")
		if err != nil {
			return evaluationValue{}, err
		}
		record, err := evaluator.evaluate(fields["record"], environment)
		if err != nil {
			return evaluationValue{}, err
		}
		if record.kind != recordValue || record.record == nil {
			return evaluationValue{}, rejection.New(rejection.ConcreteCheckMismatch, "field evaluation requires a concrete record")
		}
		fieldName, err := name(fields["field"])
		if err != nil {
			return evaluationValue{}, err
		}
		field, exists := concreteRecordField(*record.record, fieldName)
		if !exists {
			return evaluationValue{}, rejection.New(rejection.ConcreteCheckMismatch, "field evaluation cannot resolve the concrete field")
		}
		return concreteScalar(field)
	case "not":
		fields, err := object(value, "op", "value")
		if err != nil {
			return evaluationValue{}, err
		}
		operand, err := evaluator.evaluate(fields["value"], environment)
		if err != nil || operand.kind != boolValue {
			return evaluationValue{}, concreteBoolError(err, "not evaluation requires Bool")
		}
		return evaluationValue{kind: boolValue, truth: !operand.truth}, nil
	case "and":
		fields, err := object(value, "op", "values")
		if err != nil {
			return evaluationValue{}, err
		}
		values, err := array(fields["values"])
		if err != nil {
			return evaluationValue{}, err
		}
		truth := true
		for _, child := range values {
			operand, err := evaluator.evaluate(child, environment)
			if err != nil || operand.kind != boolValue {
				return evaluationValue{}, concreteBoolError(err, "and evaluation requires Bool operands")
			}
			truth = truth && operand.truth
		}
		return evaluationValue{kind: boolValue, truth: truth}, nil
	case "eq":
		fields, err := object(value, "left", "op", "right")
		if err != nil {
			return evaluationValue{}, err
		}
		left, err := evaluator.evaluate(fields["left"], environment)
		if err != nil {
			return evaluationValue{}, err
		}
		right, err := evaluator.evaluate(fields["right"], environment)
		if err != nil {
			return evaluationValue{}, err
		}
		equal, err := equalEvaluationValues(left, right)
		return evaluationValue{kind: boolValue, truth: equal}, err
	case "le":
		fields, err := object(value, "left", "op", "right")
		if err != nil {
			return evaluationValue{}, err
		}
		left, err := evaluator.evaluate(fields["left"], environment)
		if err != nil {
			return evaluationValue{}, err
		}
		right, err := evaluator.evaluate(fields["right"], environment)
		if err != nil {
			return evaluationValue{}, err
		}
		if left.kind != intValue || right.kind != intValue || left.integer == nil || right.integer == nil {
			return evaluationValue{}, rejection.New(rejection.ConcreteCheckMismatch, "le evaluation requires concrete Int operands")
		}
		return evaluationValue{kind: boolValue, truth: left.integer.Cmp(right.integer) <= 0}, nil
	case "if":
		fields, err := object(value, "condition", "else", "op", "result_type", "then")
		if err != nil {
			return evaluationValue{}, err
		}
		condition, err := evaluator.evaluate(fields["condition"], environment)
		if err != nil || condition.kind != boolValue {
			return evaluationValue{}, concreteBoolError(err, "if evaluation requires a Bool condition")
		}
		if condition.truth {
			return evaluator.evaluate(fields["then"], environment)
		}
		return evaluator.evaluate(fields["else"], environment)
	case "forall_rows":
		fields, err := object(value, "body", "op", "table")
		if err != nil {
			return evaluationValue{}, err
		}
		table, _, err := evaluator.inputTable(fields["table"])
		if err != nil {
			return evaluationValue{}, err
		}
		for rowIndex := range table.Rows {
			if err := evaluator.step(); err != nil {
				return evaluationValue{}, err
			}
			bodyEnvironment := prependEvaluation(
				evaluationValue{kind: recordValue, record: &table.Rows[rowIndex]}, environment,
			)
			body, err := evaluator.evaluate(fields["body"], bodyEnvironment)
			if err != nil || body.kind != boolValue {
				return evaluationValue{}, concreteBoolError(err, "forall_rows body evaluation requires Bool")
			}
			if !body.truth {
				return evaluationValue{kind: boolValue, truth: false}, nil
			}
		}
		return evaluationValue{kind: boolValue, truth: true}, nil
	case "lookup":
		fields, err := object(value, "keys", "op", "table")
		if err != nil {
			return evaluationValue{}, err
		}
		table, definition, err := evaluator.inputTable(fields["table"])
		if err != nil {
			return evaluationValue{}, err
		}
		keyExpressions, err := array(fields["keys"])
		if err != nil {
			return evaluationValue{}, err
		}
		if len(keyExpressions) != len(definition.primaryKey) {
			return evaluationValue{}, rejection.New(rejection.ConcreteCheckMismatch, "lookup key arity differs from the concrete table primary key")
		}
		keys := make([]evaluationValue, len(keyExpressions))
		for index, expression := range keyExpressions {
			keys[index], err = evaluator.evaluate(expression, environment)
			if err != nil {
				return evaluationValue{}, err
			}
		}
		var match *ConcreteRecord
		for rowIndex := range table.Rows {
			if err := evaluator.step(); err != nil {
				return evaluationValue{}, err
			}
			matches := true
			for keyIndex, fieldName := range definition.primaryKey {
				field, exists := concreteRecordField(table.Rows[rowIndex], fieldName)
				if !exists {
					return evaluationValue{}, rejection.New(rejection.ConcreteCheckMismatch, "lookup row omits a primary-key field")
				}
				actual, err := concreteScalar(field)
				if err != nil {
					return evaluationValue{}, err
				}
				equal, err := equalEvaluationValues(keys[keyIndex], actual)
				if err != nil {
					return evaluationValue{}, err
				}
				matches = matches && equal
			}
			if matches {
				if match != nil {
					return evaluationValue{}, rejection.New(rejection.ConcreteCheckMismatch, "lookup found multiple rows for one primary key")
				}
				match = &table.Rows[rowIndex]
			}
		}
		return evaluationValue{kind: optionRecordValue, record: match, some: match != nil}, nil
	case "match_option":
		fields, err := object(value, "none", "op", "result_type", "some", "subject")
		if err != nil {
			return evaluationValue{}, err
		}
		subject, err := evaluator.evaluate(fields["subject"], environment)
		if err != nil {
			return evaluationValue{}, err
		}
		if subject.kind != optionRecordValue {
			return evaluationValue{}, rejection.New(rejection.ConcreteCheckMismatch, "match_option evaluation requires an optional record")
		}
		if !subject.some {
			return evaluator.evaluate(fields["none"], environment)
		}
		return evaluator.evaluate(fields["some"], prependEvaluation(
			evaluationValue{kind: recordValue, record: subject.record}, environment,
		))
	default:
		return evaluationValue{}, rejection.New(rejection.ConcreteCheckMismatch, "assume expression op is outside the locked concrete evaluator")
	}
}

func (evaluator *inputEvaluator) inputTable(value strictjson.Value) (*ConcreteTable, tableDefinition, error) {
	fields, err := object(value, "kind", "name")
	if err != nil {
		return nil, tableDefinition{}, err
	}
	if err := requireText(fields["kind"], "input", rejection.ConcreteCheckMismatch, "assume table reference must select input"); err != nil {
		return nil, tableDefinition{}, err
	}
	interfaceName, err := name(fields["name"])
	if err != nil {
		return nil, tableDefinition{}, err
	}
	tableID, exists := evaluator.document.inputTables[interfaceName]
	if !exists {
		return nil, tableDefinition{}, rejection.New(rejection.ConcreteCheckMismatch, "assume input interface is unavailable")
	}
	definition, exists := evaluator.document.tables[tableID]
	if !exists {
		return nil, tableDefinition{}, rejection.New(rejection.ConcreteCheckMismatch, "assume input table definition is unavailable")
	}
	for index := range evaluator.world.Tables {
		if evaluator.world.Tables[index].Name == interfaceName {
			return &evaluator.world.Tables[index], definition, nil
		}
	}
	return nil, tableDefinition{}, rejection.New(rejection.ConcreteCheckMismatch, "concrete input omits an assume table")
}

func (evaluator *inputEvaluator) step() error {
	if evaluator.steps >= evaluator.maxSteps {
		return rejection.New(rejection.ResourceLimit, "concrete assume evaluation exceeds its semantic step limit")
	}
	evaluator.steps++
	return nil
}

func concreteRecordField(record ConcreteRecord, name string) (ConcreteValue, bool) {
	for _, field := range record.Fields {
		if field.Name == name {
			return field.Value, true
		}
	}
	return ConcreteValue{}, false
}

func concreteScalar(value ConcreteValue) (evaluationValue, error) {
	switch value.Kind {
	case "bool":
		return evaluationValue{kind: boolValue, truth: value.Bool}, nil
	case "int":
		integer, ok := parseConcreteInteger(value.Integer)
		if !ok {
			return evaluationValue{}, rejection.New(rejection.ConcreteCheckMismatch, "concrete Int is not canonical")
		}
		return evaluationValue{kind: intValue, integer: integer}, nil
	case "text":
		return evaluationValue{kind: textValue, text: value.Text}, nil
	case "enum":
		return evaluationValue{kind: enumValue, enumType: value.EnumType, enumMember: value.EnumMember}, nil
	default:
		return evaluationValue{}, rejection.New(rejection.ConcreteCheckMismatch, "concrete field is not a supported scalar value")
	}
}

func equalEvaluationValues(left, right evaluationValue) (bool, error) {
	if left.kind != right.kind {
		return false, rejection.New(rejection.ConcreteCheckMismatch, "concrete equality operands have different kinds")
	}
	switch left.kind {
	case boolValue:
		return left.truth == right.truth, nil
	case intValue:
		if left.integer == nil || right.integer == nil {
			return false, rejection.New(rejection.ConcreteCheckMismatch, "concrete Int equality has an unavailable operand")
		}
		return left.integer.Cmp(right.integer) == 0, nil
	case textValue:
		return left.text == right.text, nil
	case enumValue:
		return left.enumType == right.enumType && left.enumMember == right.enumMember, nil
	default:
		return false, rejection.New(rejection.ConcreteCheckMismatch, "concrete equality kind is unsupported")
	}
}

func prependEvaluation(value evaluationValue, environment []evaluationValue) []evaluationValue {
	result := make([]evaluationValue, 0, len(environment)+1)
	result = append(result, value)
	result = append(result, environment...)
	return result
}

func concreteBoolError(err error, detail string) error {
	if err != nil {
		return err
	}
	return rejection.New(rejection.ConcreteCheckMismatch, detail)
}
