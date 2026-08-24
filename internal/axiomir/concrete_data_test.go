package axiomir

import (
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
)

func TestCheckInputWorldCoversClosedScalarAndTableWFProfile(t *testing.T) {
	document, enumID, recordID := concreteDataDocument()
	world := ConcreteWorld{Tables: []ConcreteTable{{
		Name: "input",
		Rows: []ConcreteRecord{
			concreteDataRow(enumID, recordID, false, "-1", "low", "alpha"),
			concreteDataRow(enumID, recordID, true, "2", "high", "omega"),
		},
	}}}
	check := document.CheckInputWorld(world)
	if !check.Anchored || !check.WellFormed || len(check.Violations) != 0 {
		t.Fatalf("valid concrete world rejected: %+v", check)
	}

	tests := []struct {
		name   string
		mutate func(*ConcreteWorld)
	}{
		{
			name: "capacity exceeded",
			mutate: func(world *ConcreteWorld) {
				world.Tables[0].Rows = append(world.Tables[0].Rows, concreteDataRow(enumID, recordID, true, "1", "high", "extra"))
			},
		},
		{
			name: "enum primary key order reversed",
			mutate: func(world *ConcreteWorld) {
				world.Tables[0].Rows[0], world.Tables[0].Rows[1] = world.Tables[0].Rows[1], world.Tables[0].Rows[0]
			},
		},
		{
			name: "duplicate primary key",
			mutate: func(world *ConcreteWorld) {
				world.Tables[0].Rows[1].Fields[1].Value.EnumMember = "low"
			},
		},
		{
			name: "bool kind drift",
			mutate: func(world *ConcreteWorld) {
				world.Tables[0].Rows[0].Fields[0].Value = ConcreteValue{Kind: "text", Text: "false"}
			},
		},
		{
			name: "integer range exceeded",
			mutate: func(world *ConcreteWorld) {
				world.Tables[0].Rows[0].Fields[2].Value.Integer = "3"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := cloneConcreteWorld(world)
			test.mutate(&mutated)
			check := document.CheckInputWorld(mutated)
			if !check.Anchored || check.WellFormed || len(check.Violations) == 0 {
				t.Fatalf("invalid concrete world was not classified as anchored non-WF: %+v", check)
			}
		})
	}
}

func concreteDataDocument() (Document, protocol.Digest, protocol.Digest) {
	enumID := concreteDataDigest(1)
	recordID := concreteDataDigest(2)
	tableID := concreteDataDigest(3)
	return Document{
		enums: map[protocol.Digest]enumDefinition{
			enumID: {
				members:     map[string]struct{}{"high": {}, "low": {}},
				memberOrder: map[string]int{"low": 0, "high": 1},
			},
		},
		records: map[protocol.Digest]recordDefinition{
			recordID: {fields: map[string]fieldDefinition{
				"b": {label: publicLabel, typeInfo: valueType{kind: boolValue}},
				"e": {label: publicLabel, typeInfo: valueType{kind: enumValue, enumType: enumID}},
				"i": {label: publicLabel, typeInfo: valueType{kind: intValue, lower: "-1", upper: "2"}},
				"t": {label: publicLabel, typeInfo: valueType{kind: textValue}},
			}},
		},
		tables: map[protocol.Digest]tableDefinition{
			tableID: {capacity: "2", primaryKey: []string{"e"}, recordType: recordID},
		},
		inputTables: map[string]protocol.Digest{"input": tableID},
	}, enumID, recordID
}

func concreteDataRow(
	enumID, recordID protocol.Digest,
	truth bool,
	integer, enumMember, text string,
) ConcreteRecord {
	return ConcreteRecord{
		RecordType: recordID,
		Fields: []ConcreteField{
			{Name: "b", Value: ConcreteValue{Kind: "bool", Bool: truth}},
			{Name: "e", Value: ConcreteValue{Kind: "enum", EnumType: enumID, EnumMember: enumMember}},
			{Name: "i", Value: ConcreteValue{Kind: "int", Integer: integer}},
			{Name: "t", Value: ConcreteValue{Kind: "text", Text: text}},
		},
	}
}

func cloneConcreteWorld(source ConcreteWorld) ConcreteWorld {
	result := ConcreteWorld{Tables: make([]ConcreteTable, len(source.Tables))}
	for tableIndex, table := range source.Tables {
		result.Tables[tableIndex].Name = table.Name
		result.Tables[tableIndex].Rows = make([]ConcreteRecord, len(table.Rows))
		for rowIndex, row := range table.Rows {
			result.Tables[tableIndex].Rows[rowIndex] = ConcreteRecord{
				RecordType: row.RecordType,
				Fields:     append([]ConcreteField(nil), row.Fields...),
			}
		}
	}
	return result
}

func concreteDataDigest(seed byte) protocol.Digest {
	var result protocol.Digest
	result[0] = seed
	return result
}
