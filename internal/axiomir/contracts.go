package axiomir

import (
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

func (p *parser) parseContractDefinition(value strictjson.Value, _ protocol.Digest) error {
	kindValue, err := member(value, "kind")
	if err != nil {
		return err
	}
	kind, err := text(kindValue)
	if err != nil {
		return err
	}
	switch kind {
	case "formula":
		fields, err := object(value, "expression", "kind", "role")
		if err != nil {
			return err
		}
		role, err := text(fields["role"])
		if err != nil {
			return err
		}
		var scope expressionScope
		switch role {
		case "assume":
			scope = assumeExpression
		case "guarantee":
			scope = guaranteeExpression
		default:
			return rejection.New(rejection.UnknownTag, "unknown formula contract role")
		}
		if _, err := p.parseExpression(fields["expression"], 0, scope); err != nil {
			return err
		}
		formulaType, err := p.inferExpression(fields["expression"], nil, scope)
		if err != nil {
			return err
		}
		if formulaType.kind != boolValue {
			return rejection.New(rejection.InvalidJSON, "formula contract expression must be Bool")
		}
		return nil
	case "noninterference":
		fields, err := object(value, "inputs", "kind", "outputs")
		if err != nil {
			return err
		}
		inputs, err := parseSortedNames(fields["inputs"], "noninterference inputs are not sorted and unique")
		if err != nil {
			return err
		}
		outputs, err := parseSortedNames(fields["outputs"], "noninterference outputs are not sorted and unique")
		if err != nil {
			return err
		}
		if len(inputs) == 0 || len(outputs) == 0 {
			return rejection.New(rejection.InvalidJSON, "noninterference interfaces must not be empty")
		}
		for _, input := range inputs {
			if _, exists := p.inputPorts[input]; !exists {
				return rejection.New(rejection.InvalidJSON, "noninterference input reference does not resolve")
			}
		}
		for _, output := range outputs {
			if _, exists := p.outputNames[output]; !exists {
				return rejection.New(rejection.InvalidJSON, "noninterference output reference does not resolve")
			}
		}
		return nil
	default:
		return rejection.New(rejection.UnknownTag, "contract kind is outside the locked Axiom IR structure profile")
	}
}

func parseSortedNames(value strictjson.Value, detail string) ([]string, error) {
	items, err := array(value)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(items))
	var previous string
	for index, item := range items {
		current, err := name(item)
		if err != nil {
			return nil, err
		}
		if err := requireStrictOrder(previous, current, index != 0, detail); err != nil {
			return nil, err
		}
		previous = current
		result = append(result, current)
	}
	return result, nil
}
