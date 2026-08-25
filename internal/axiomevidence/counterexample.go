package axiomevidence

import (
	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

type counterexampleDefinition struct {
	kind          string
	preconditions []protocol.Digest
	trace         []counterexampleTraceStep
	worlds        []axiomir.ConcreteWorld
	observed      counterexampleObserved
}

type counterexampleTraceStep struct {
	kind  string
	ref   protocol.Digest
	value string
}

type counterexampleObserved struct {
	kind           string
	obligation     protocol.Digest
	requiredFields []string
	requiredKeys   []string
	actual         protocol.Digest
	expected       protocol.Digest
}

var counterexampleKinds = map[string]struct{}{
	"group":        {},
	"missing-key":  {},
	"paired-input": {},
	"row-pair":     {},
	"single-row":   {},
}

func (p *parser) parseCounterexample(value strictjson.Value) (counterexampleDefinition, error) {
	fields, err := object(value, "kind", "minimality", "observed", "preconditions", "trace", "worlds")
	if err != nil {
		return counterexampleDefinition{}, err
	}
	kind, err := requireOneOf(fields["kind"], counterexampleKinds, "unsupported Axiom Evidence counterexample kind")
	if err != nil {
		return counterexampleDefinition{}, err
	}
	if err := parseMinimality(fields["minimality"]); err != nil {
		return counterexampleDefinition{}, err
	}
	preconditions, err := parseDigestSet(fields["preconditions"], false, "Axiom Evidence counterexample preconditions are unsorted or duplicate")
	if err != nil {
		return counterexampleDefinition{}, err
	}
	trace, err := p.parseTrace(fields["trace"])
	if err != nil {
		return counterexampleDefinition{}, err
	}
	worlds, err := p.parseWorlds(fields["worlds"])
	if err != nil {
		return counterexampleDefinition{}, err
	}
	observed, err := p.parseObserved(fields["observed"])
	if err != nil {
		return counterexampleDefinition{}, err
	}
	return counterexampleDefinition{
		kind: kind, preconditions: preconditions, trace: trace, worlds: worlds, observed: observed,
	}, nil
}

func parseMinimality(value strictjson.Value) error {
	fields, err := object(value, "kind", "order")
	if err != nil {
		return err
	}
	if err := requireText(fields["kind"], "reduced", rejection.UnknownTag, "unsupported counterexample minimality kind"); err != nil {
		return err
	}
	return requireText(fields["order"], "axiom-witness-order-v0.1", rejection.UnknownTag, "unsupported counterexample reduction order")
}

func (p *parser) parseTrace(value strictjson.Value) ([]counterexampleTraceStep, error) {
	steps, err := array(value)
	if err != nil {
		return nil, err
	}
	if len(steps) == 0 {
		return nil, rejection.New(rejection.InvalidJSON, "Axiom Evidence counterexample trace must not be empty")
	}
	result := make([]counterexampleTraceStep, 0, len(steps))
	for _, step := range steps {
		tagValue, err := member(step, "kind")
		if err != nil {
			return nil, err
		}
		tag, err := text(tagValue)
		if err != nil {
			return nil, err
		}
		switch tag {
		case "document":
			fields, err := object(step, "kind", "ref")
			if err != nil {
				return nil, err
			}
			document, err := digest(fields["ref"])
			if err != nil {
				return nil, err
			}
			p.documentRefs[document] = struct{}{}
			result = append(result, counterexampleTraceStep{kind: tag, ref: document})
		case "obligation":
			fields, err := object(step, "kind", "ref")
			if err != nil {
				return nil, err
			}
			obligation, err := digest(fields["ref"])
			if err != nil {
				return nil, err
			}
			p.obligationRefs[obligation] = struct{}{}
			result = append(result, counterexampleTraceStep{kind: tag, ref: obligation})
		case "observation":
			fields, err := object(step, "kind", "value")
			if err != nil {
				return nil, err
			}
			if err := requireText(fields["value"], "failed", rejection.UnknownTag, "unsupported counterexample observation"); err != nil {
				return nil, err
			}
			result = append(result, counterexampleTraceStep{kind: tag, value: "failed"})
		default:
			return nil, rejection.New(rejection.UnknownTag, "unsupported Axiom Evidence counterexample trace step")
		}
	}
	return result, nil
}

func (p *parser) parseWorlds(value strictjson.Value) ([]axiomir.ConcreteWorld, error) {
	worlds, err := array(value)
	if err != nil {
		return nil, err
	}
	result := make([]axiomir.ConcreteWorld, 0, len(worlds))
	for _, world := range worlds {
		fields, err := object(world, "tables")
		if err != nil {
			return nil, err
		}
		tables, err := p.parseTables(fields["tables"])
		if err != nil {
			return nil, err
		}
		result = append(result, axiomir.ConcreteWorld{Tables: tables})
	}
	return result, nil
}

func (p *parser) parseTables(value strictjson.Value) ([]axiomir.ConcreteTable, error) {
	tables, err := array(value)
	if err != nil {
		return nil, err
	}
	result := make([]axiomir.ConcreteTable, 0, len(tables))
	var previous string
	for index, table := range tables {
		fields, err := object(table, "name", "rows")
		if err != nil {
			return nil, err
		}
		name, err := nonemptyText(fields["name"])
		if err != nil {
			return nil, err
		}
		if err := requireStrictOrder(previous, name, index != 0, "counterexample tables are not sorted and unique by input name"); err != nil {
			return nil, err
		}
		previous = name
		rows, err := array(fields["rows"])
		if err != nil {
			return nil, err
		}
		parsedRows := make([]axiomir.ConcreteRecord, 0, len(rows))
		for _, row := range rows {
			parsed, err := p.parseRecord(row)
			if err != nil {
				return nil, err
			}
			parsedRows = append(parsedRows, parsed)
		}
		result = append(result, axiomir.ConcreteTable{Name: name, Rows: parsedRows})
	}
	return result, nil
}

func (p *parser) parseRecord(value strictjson.Value) (axiomir.ConcreteRecord, error) {
	fields, err := object(value, "fields", "kind", "record_type")
	if err != nil {
		return axiomir.ConcreteRecord{}, err
	}
	if err := requireText(fields["kind"], "record", rejection.UnknownTag, "unsupported counterexample row kind"); err != nil {
		return axiomir.ConcreteRecord{}, err
	}
	recordType, err := digest(fields["record_type"])
	if err != nil {
		return axiomir.ConcreteRecord{}, err
	}
	fieldItems, err := array(fields["fields"])
	if err != nil {
		return axiomir.ConcreteRecord{}, err
	}
	parsedFields := make([]axiomir.ConcreteField, 0, len(fieldItems))
	var previous string
	for index, field := range fieldItems {
		fieldFields, err := object(field, "name", "value")
		if err != nil {
			return axiomir.ConcreteRecord{}, err
		}
		name, err := nonemptyText(fieldFields["name"])
		if err != nil {
			return axiomir.ConcreteRecord{}, err
		}
		if err := requireStrictOrder(previous, name, index != 0, "counterexample record fields are not sorted and unique by name"); err != nil {
			return axiomir.ConcreteRecord{}, err
		}
		previous = name
		parsedValue, err := parseWitnessValue(fieldFields["value"])
		if err != nil {
			return axiomir.ConcreteRecord{}, err
		}
		parsedFields = append(parsedFields, axiomir.ConcreteField{Name: name, Value: parsedValue})
	}
	return axiomir.ConcreteRecord{RecordType: recordType, Fields: parsedFields}, nil
}

func parseWitnessValue(value strictjson.Value) (axiomir.ConcreteValue, error) {
	tagValue, err := member(value, "kind")
	if err != nil {
		return axiomir.ConcreteValue{}, err
	}
	tag, err := text(tagValue)
	if err != nil {
		return axiomir.ConcreteValue{}, err
	}
	switch tag {
	case "bool":
		fields, err := object(value, "kind", "value")
		if err != nil {
			return axiomir.ConcreteValue{}, err
		}
		truth, ok := fields["value"].Bool()
		if !ok {
			return axiomir.ConcreteValue{}, rejection.New(rejection.InvalidJSON, "counterexample Bool value must be a JSON boolean")
		}
		return axiomir.ConcreteValue{Kind: tag, Bool: truth}, nil
	case "int":
		fields, err := object(value, "kind", "value")
		if err != nil {
			return axiomir.ConcreteValue{}, err
		}
		integer, err := canonicalSigned(fields["value"])
		return axiomir.ConcreteValue{Kind: tag, Integer: integer}, err
	case "text":
		fields, err := object(value, "kind", "value")
		if err != nil {
			return axiomir.ConcreteValue{}, err
		}
		value, err := text(fields["value"])
		return axiomir.ConcreteValue{Kind: tag, Text: value}, err
	case "enum":
		fields, err := object(value, "enum_type", "kind", "member")
		if err != nil {
			return axiomir.ConcreteValue{}, err
		}
		enumType, err := digest(fields["enum_type"])
		if err != nil {
			return axiomir.ConcreteValue{}, err
		}
		member, err := nonemptyText(fields["member"])
		return axiomir.ConcreteValue{Kind: tag, EnumType: enumType, EnumMember: member}, err
	default:
		return axiomir.ConcreteValue{}, rejection.New(rejection.UnknownTag, "unsupported Axiom Evidence witness value kind")
	}
}

func (p *parser) parseObserved(value strictjson.Value) (counterexampleObserved, error) {
	tagValue, err := member(value, "kind")
	if err != nil {
		return counterexampleObserved{}, err
	}
	tag, err := text(tagValue)
	if err != nil {
		return counterexampleObserved{}, err
	}
	switch tag {
	case "obligation-failure":
		fields, err := object(value, "kind", "obligation", "required_fields", "required_keys")
		if err != nil {
			return counterexampleObserved{}, err
		}
		obligation, err := digest(fields["obligation"])
		if err != nil {
			return counterexampleObserved{}, err
		}
		p.obligationRefs[obligation] = struct{}{}
		requiredFields, err := parseStringSet(fields["required_fields"], false, nil, "counterexample required fields are unsorted or duplicate")
		if err != nil {
			return counterexampleObserved{}, err
		}
		requiredKeys, err := parseStringSet(fields["required_keys"], false, nil, "counterexample required keys are unsorted or duplicate")
		return counterexampleObserved{
			kind: tag, obligation: obligation, requiredFields: requiredFields, requiredKeys: requiredKeys,
		}, err
	case "host-output-mismatch":
		fields, err := object(value, "actual", "expected", "kind")
		if err != nil {
			return counterexampleObserved{}, err
		}
		actual, err := digest(fields["actual"])
		if err != nil {
			return counterexampleObserved{}, err
		}
		expected, err := digest(fields["expected"])
		if err != nil {
			return counterexampleObserved{}, err
		}
		p.artifactRefs[actual] = struct{}{}
		p.artifactRefs[expected] = struct{}{}
		return counterexampleObserved{kind: tag, actual: actual, expected: expected}, nil
	default:
		return counterexampleObserved{}, rejection.New(rejection.UnknownTag, "unsupported Axiom Evidence counterexample observation kind")
	}
}
