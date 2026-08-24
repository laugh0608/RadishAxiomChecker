package axiomevidence

import (
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

var counterexampleKinds = map[string]struct{}{
	"group":        {},
	"missing-key":  {},
	"paired-input": {},
	"row-pair":     {},
	"single-row":   {},
}

func (p *parser) parseCounterexample(value strictjson.Value) error {
	fields, err := object(value, "kind", "minimality", "observed", "preconditions", "trace", "worlds")
	if err != nil {
		return err
	}
	if _, err := requireOneOf(fields["kind"], counterexampleKinds, "unsupported Axiom Evidence counterexample kind"); err != nil {
		return err
	}
	if err := parseMinimality(fields["minimality"]); err != nil {
		return err
	}
	if _, err := parseDigestSet(fields["preconditions"], false, "Axiom Evidence counterexample preconditions are unsorted or duplicate"); err != nil {
		return err
	}
	if err := p.parseTrace(fields["trace"]); err != nil {
		return err
	}
	if err := p.parseWorlds(fields["worlds"]); err != nil {
		return err
	}
	return p.parseObserved(fields["observed"])
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

func (p *parser) parseTrace(value strictjson.Value) error {
	steps, err := array(value)
	if err != nil {
		return err
	}
	if len(steps) == 0 {
		return rejection.New(rejection.InvalidJSON, "Axiom Evidence counterexample trace must not be empty")
	}
	for _, step := range steps {
		tagValue, err := member(step, "kind")
		if err != nil {
			return err
		}
		tag, err := text(tagValue)
		if err != nil {
			return err
		}
		switch tag {
		case "document":
			fields, err := object(step, "kind", "ref")
			if err != nil {
				return err
			}
			document, err := digest(fields["ref"])
			if err != nil {
				return err
			}
			p.documentRefs[document] = struct{}{}
		case "obligation":
			fields, err := object(step, "kind", "ref")
			if err != nil {
				return err
			}
			obligation, err := digest(fields["ref"])
			if err != nil {
				return err
			}
			p.obligationRefs[obligation] = struct{}{}
		case "observation":
			fields, err := object(step, "kind", "value")
			if err != nil {
				return err
			}
			if err := requireText(fields["value"], "failed", rejection.UnknownTag, "unsupported counterexample observation"); err != nil {
				return err
			}
		default:
			return rejection.New(rejection.UnknownTag, "unsupported Axiom Evidence counterexample trace step")
		}
	}
	return nil
}

func (p *parser) parseWorlds(value strictjson.Value) error {
	worlds, err := array(value)
	if err != nil {
		return err
	}
	for _, world := range worlds {
		fields, err := object(world, "tables")
		if err != nil {
			return err
		}
		if err := p.parseTables(fields["tables"]); err != nil {
			return err
		}
	}
	return nil
}

func (p *parser) parseTables(value strictjson.Value) error {
	tables, err := array(value)
	if err != nil {
		return err
	}
	var previous string
	for index, table := range tables {
		fields, err := object(table, "name", "rows")
		if err != nil {
			return err
		}
		name, err := nonemptyText(fields["name"])
		if err != nil {
			return err
		}
		if err := requireStrictOrder(previous, name, index != 0, "counterexample tables are not sorted and unique by input name"); err != nil {
			return err
		}
		previous = name
		rows, err := array(fields["rows"])
		if err != nil {
			return err
		}
		for _, row := range rows {
			if err := p.parseRecord(row); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *parser) parseRecord(value strictjson.Value) error {
	fields, err := object(value, "fields", "kind", "record_type")
	if err != nil {
		return err
	}
	if err := requireText(fields["kind"], "record", rejection.UnknownTag, "unsupported counterexample row kind"); err != nil {
		return err
	}
	if _, err := digest(fields["record_type"]); err != nil {
		return err
	}
	fieldItems, err := array(fields["fields"])
	if err != nil {
		return err
	}
	var previous string
	for index, field := range fieldItems {
		fieldFields, err := object(field, "name", "value")
		if err != nil {
			return err
		}
		name, err := nonemptyText(fieldFields["name"])
		if err != nil {
			return err
		}
		if err := requireStrictOrder(previous, name, index != 0, "counterexample record fields are not sorted and unique by name"); err != nil {
			return err
		}
		previous = name
		if err := parseWitnessValue(fieldFields["value"]); err != nil {
			return err
		}
	}
	return nil
}

func parseWitnessValue(value strictjson.Value) error {
	tagValue, err := member(value, "kind")
	if err != nil {
		return err
	}
	tag, err := text(tagValue)
	if err != nil {
		return err
	}
	switch tag {
	case "int":
		fields, err := object(value, "kind", "value")
		if err != nil {
			return err
		}
		_, err = canonicalSigned(fields["value"])
		return err
	case "text":
		fields, err := object(value, "kind", "value")
		if err != nil {
			return err
		}
		_, err = text(fields["value"])
		return err
	case "enum":
		fields, err := object(value, "enum_type", "kind", "member")
		if err != nil {
			return err
		}
		if _, err := digest(fields["enum_type"]); err != nil {
			return err
		}
		_, err = nonemptyText(fields["member"])
		return err
	default:
		return rejection.New(rejection.UnknownTag, "unsupported Axiom Evidence witness value kind")
	}
}

func (p *parser) parseObserved(value strictjson.Value) error {
	tagValue, err := member(value, "kind")
	if err != nil {
		return err
	}
	tag, err := text(tagValue)
	if err != nil {
		return err
	}
	switch tag {
	case "obligation-failure":
		fields, err := object(value, "kind", "obligation", "required_fields", "required_keys")
		if err != nil {
			return err
		}
		obligation, err := digest(fields["obligation"])
		if err != nil {
			return err
		}
		p.obligationRefs[obligation] = struct{}{}
		if _, err := parseStringSet(fields["required_fields"], false, nil, "counterexample required fields are unsorted or duplicate"); err != nil {
			return err
		}
		_, err = parseStringSet(fields["required_keys"], false, nil, "counterexample required keys are unsorted or duplicate")
		return err
	case "host-output-mismatch":
		fields, err := object(value, "actual", "expected", "kind")
		if err != nil {
			return err
		}
		for _, name := range []string{"actual", "expected"} {
			artifact, err := digest(fields[name])
			if err != nil {
				return err
			}
			p.artifactRefs[artifact] = struct{}{}
		}
		return nil
	default:
		return rejection.New(rejection.UnknownTag, "unsupported Axiom Evidence counterexample observation kind")
	}
}
