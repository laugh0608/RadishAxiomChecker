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
	memberOrder := make(map[string]int, len(items))
	for index, item := range items {
		memberName, err := name(item)
		if err != nil {
			return err
		}
		if _, exists := members[memberName]; exists {
			return rejection.New(rejection.InvalidJSON, "enum members must be unique")
		}
		members[memberName] = struct{}{}
		memberOrder[memberName] = index
	}
	p.enums[id] = enumDefinition{members: members, memberOrder: memberOrder}
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
	recordFields := make(map[string]fieldDefinition, len(items))
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
		labelText, err := text(field["label"])
		if err != nil {
			return err
		}
		var label fieldLabel
		switch labelText {
		case "public":
			label = publicLabel
		case "sensitive":
			label = sensitiveLabel
		default:
			return rejection.New(rejection.UnknownTag, "unknown record field label")
		}
		fieldType, err := p.parseValueType(field["type"])
		if err != nil {
			return err
		}
		recordFields[fieldName] = fieldDefinition{label: label, typeInfo: fieldType}
	}
	p.records[id] = recordDefinition{fields: recordFields}
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
	capacity, err := text(fields["capacity"])
	if err != nil {
		return err
	}
	recordType, err := digest(fields["record_type"])
	if err != nil {
		return err
	}
	record, exists := p.records[recordType]
	if !exists {
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
	keyNames := make([]string, 0, len(primaryKey))
	for _, item := range primaryKey {
		fieldName, err := name(item)
		if err != nil {
			return err
		}
		if _, exists := seen[fieldName]; exists {
			return rejection.New(rejection.InvalidJSON, "table primary key fields must be unique")
		}
		definition, exists := record.fields[fieldName]
		if !exists {
			return rejection.New(rejection.InvalidJSON, "table primary key field does not exist in its record type")
		}
		if definition.label != publicLabel {
			return rejection.New(rejection.InvalidJSON, "table primary key field must be public")
		}
		if !definition.typeInfo.keyCompatible() {
			return rejection.New(rejection.InvalidJSON, "table primary key field type is not key-compatible")
		}
		seen[fieldName] = struct{}{}
		keyNames = append(keyNames, fieldName)
	}
	p.tables[id] = tableDefinition{
		capacity:   capacity,
		primaryKey: keyNames,
		recordType: recordType,
	}
	return nil
}

func (p *parser) parseValueType(value strictjson.Value) (valueType, error) {
	kindValue, err := member(value, "kind")
	if err != nil {
		return valueType{}, err
	}
	kind, err := text(kindValue)
	if err != nil {
		return valueType{}, err
	}
	switch kind {
	case "bool":
		if _, err := object(value, "kind"); err != nil {
			return valueType{}, err
		}
		return valueType{kind: boolValue}, nil
	case "text":
		if _, err := object(value, "kind"); err != nil {
			return valueType{}, err
		}
		return valueType{kind: textValue}, nil
	case "int":
		fields, err := object(value, "kind", "lower", "upper")
		if err != nil {
			return valueType{}, err
		}
		lower, err := canonicalInteger(fields["lower"], false)
		if err != nil {
			return valueType{}, err
		}
		upper, err := canonicalInteger(fields["upper"], false)
		if err != nil {
			return valueType{}, err
		}
		if lower.Cmp(upper) > 0 {
			return valueType{}, rejection.New(rejection.InvalidJSON, "integer type lower bound exceeds upper bound")
		}
		return valueType{kind: intValue, lower: lower.String(), upper: upper.String()}, nil
	case "enum":
		fields, err := object(value, "enum_type", "kind")
		if err != nil {
			return valueType{}, err
		}
		enumType, err := digest(fields["enum_type"])
		if err != nil {
			return valueType{}, err
		}
		if _, exists := p.enums[enumType]; !exists {
			return valueType{}, rejection.New(rejection.InvalidJSON, "enum type reference does not resolve")
		}
		return valueType{kind: enumValue, enumType: enumType}, nil
	default:
		return valueType{}, rejection.New(rejection.UnknownTag, "value type is outside the locked Axiom IR structure profile")
	}
}
