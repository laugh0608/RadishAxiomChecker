package axiomir

import (
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

func (p *parser) parseEnumDefinition(value strictjson.Value, id protocol.Digest) error {
	fields, err := object(value, "members", "name")
	if err != nil {
		return err
	}
	if _, err := name(fields["name"]); err != nil {
		return err
	}
	items, err := array(fields["members"])
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return rejection.New(rejection.InvalidJSON, "enum members must not be empty")
	}
	members := make(map[string]struct{}, len(items))
	for _, item := range items {
		memberName, err := name(item)
		if err != nil {
			return err
		}
		if _, exists := members[memberName]; exists {
			return rejection.New(rejection.InvalidJSON, "enum members must be unique")
		}
		members[memberName] = struct{}{}
	}
	p.enums[id] = enumDefinition{members: members}
	return nil
}

func (p *parser) parseRecordDefinition(value strictjson.Value, id protocol.Digest) error {
	fields, err := object(value, "fields")
	if err != nil {
		return err
	}
	items, err := array(fields["fields"])
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return rejection.New(rejection.InvalidJSON, "record fields must not be empty")
	}
	var previous string
	for index, item := range items {
		field, err := object(item, "label", "name", "type")
		if err != nil {
			return err
		}
		fieldName, err := name(field["name"])
		if err != nil {
			return err
		}
		if err := requireStrictOrder(previous, fieldName, index != 0, "record fields are not sorted and unique by name"); err != nil {
			return err
		}
		previous = fieldName
		label, err := text(field["label"])
		if err != nil {
			return err
		}
		if label != "public" && label != "sensitive" {
			return rejection.New(rejection.UnknownTag, "unknown record field label")
		}
		if _, err := p.parseValueType(field["type"]); err != nil {
			return err
		}
	}
	p.records[id] = struct{}{}
	return nil
}

func (p *parser) parseTableDefinition(value strictjson.Value, id protocol.Digest) error {
	fields, err := object(value, "capacity", "primary_key", "record_type")
	if err != nil {
		return err
	}
	if _, err := canonicalInteger(fields["capacity"], true); err != nil {
		return err
	}
	recordType, err := digest(fields["record_type"])
	if err != nil {
		return err
	}
	if _, exists := p.records[recordType]; !exists {
		return rejection.New(rejection.InvalidJSON, "table record type reference does not resolve")
	}
	primaryKey, err := array(fields["primary_key"])
	if err != nil {
		return err
	}
	if len(primaryKey) == 0 {
		return rejection.New(rejection.InvalidJSON, "table primary key must not be empty")
	}
	seen := make(map[string]struct{}, len(primaryKey))
	for _, item := range primaryKey {
		fieldName, err := name(item)
		if err != nil {
			return err
		}
		if _, exists := seen[fieldName]; exists {
			return rejection.New(rejection.InvalidJSON, "table primary key fields must be unique")
		}
		seen[fieldName] = struct{}{}
	}
	p.tables[id] = struct{}{}
	return nil
}

func (p *parser) parseValueType(value strictjson.Value) (string, error) {
	kindValue, err := member(value, "kind")
	if err != nil {
		return "", err
	}
	kind, err := text(kindValue)
	if err != nil {
		return "", err
	}
	switch kind {
	case "bool", "text":
		if _, err := object(value, "kind"); err != nil {
			return "", err
		}
	case "int":
		fields, err := object(value, "kind", "lower", "upper")
		if err != nil {
			return "", err
		}
		lower, err := canonicalInteger(fields["lower"], false)
		if err != nil {
			return "", err
		}
		upper, err := canonicalInteger(fields["upper"], false)
		if err != nil {
			return "", err
		}
		if lower.Cmp(upper) > 0 {
			return "", rejection.New(rejection.InvalidJSON, "integer type lower bound exceeds upper bound")
		}
	case "enum":
		fields, err := object(value, "enum_type", "kind")
		if err != nil {
			return "", err
		}
		enumType, err := digest(fields["enum_type"])
		if err != nil {
			return "", err
		}
		if _, exists := p.enums[enumType]; !exists {
			return "", rejection.New(rejection.InvalidJSON, "enum type reference does not resolve")
		}
	default:
		return "", rejection.New(rejection.UnknownTag, "value type is outside the locked Axiom IR structure profile")
	}
	return kind, nil
}
