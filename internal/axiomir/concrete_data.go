package axiomir

import (
	"math/big"
	"sort"
	"strings"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

// ConcreteValue is the closed scalar value profile retained from locked
// Evidence counterexample worlds. Integer text is canonical but remains a
// mathematical integer until it is checked against its IR type.
type ConcreteValue struct {
	Kind       string
	Bool       bool
	Integer    string
	Text       string
	EnumType   protocol.Digest
	EnumMember string
}

type ConcreteField struct {
	Name  string
	Value ConcreteValue
}

type ConcreteRecord struct {
	RecordType protocol.Digest
	Fields     []ConcreteField
}

type ConcreteTable struct {
	Name string
	Rows []ConcreteRecord
}

// ConcreteWorld is an input-world projection. The locked counterexample
// profile may retain only the tables needed by the witness, but every retained
// table and row is checked against the corresponding IR input interface.
type ConcreteWorld struct {
	Tables []ConcreteTable
}

// WorldCheck distinguishes whether a world resolves to declared interfaces
// from whether its values satisfy WF. input-conformance counterexamples may be
// anchored while intentionally not well formed.
type WorldCheck struct {
	Anchored   bool
	WellFormed bool
	Violations []string
}

// CheckInputWorld independently checks the presented input tables against the
// retained IR declarations. It does not evaluate assume contracts or execute
// the program DAG.
func (document Document) CheckInputWorld(world ConcreteWorld) WorldCheck {
	return document.checkWorld(world, document.inputTables)
}

// CheckCompleteInputWorld requires the world to contain exactly the complete
// input interface set in addition to satisfying declaration-level WF.
func (document Document) CheckCompleteInputWorld(world ConcreteWorld) WorldCheck {
	check := document.checkWorld(world, document.inputTables)
	seen := make(map[string]struct{}, len(world.Tables))
	for _, table := range world.Tables {
		seen[table.Name] = struct{}{}
	}
	if len(seen) != len(document.inputTables) {
		check.Anchored = false
		check.Violations = append(check.Violations, "concrete input does not contain the complete IR input interface set")
	} else {
		for name := range document.inputTables {
			if _, exists := seen[name]; !exists {
				check.Anchored = false
				check.Violations = append(check.Violations, "concrete input is missing a declared IR input interface")
			}
		}
	}
	return finishWorldCheck(check)
}

// CheckOutputWorld applies the same declaration-level check to named outputs.
// It is retained for the following concrete-artifact slice.
func (document Document) CheckOutputWorld(world ConcreteWorld) WorldCheck {
	return document.checkWorld(world, document.outputTables)
}

func (document Document) checkWorld(world ConcreteWorld, interfaces map[string]protocol.Digest) WorldCheck {
	check := WorldCheck{Anchored: true}
	if len(world.Tables) == 0 {
		check.Anchored = false
		check.Violations = append(check.Violations, "world contains no interface table")
		return finishWorldCheck(check)
	}

	var previousTable string
	seenTables := make(map[string]struct{}, len(world.Tables))
	for tableIndex, concreteTable := range world.Tables {
		if tableIndex != 0 && previousTable >= concreteTable.Name {
			check.Anchored = false
			check.Violations = append(check.Violations, "world tables are not sorted and unique by interface name")
		}
		previousTable = concreteTable.Name
		if _, duplicate := seenTables[concreteTable.Name]; duplicate {
			check.Anchored = false
			check.Violations = append(check.Violations, "world contains a duplicate interface table")
			continue
		}
		seenTables[concreteTable.Name] = struct{}{}

		tableID, exists := interfaces[concreteTable.Name]
		if !exists {
			check.Anchored = false
			check.Violations = append(check.Violations, "world table does not resolve to the selected IR interface direction")
			continue
		}
		table, exists := document.tables[tableID]
		if !exists {
			check.Anchored = false
			check.Violations = append(check.Violations, "world table type is unavailable")
			continue
		}
		record, exists := document.records[table.recordType]
		if !exists {
			check.Anchored = false
			check.Violations = append(check.Violations, "world record type is unavailable")
			continue
		}

		capacity, ok := new(big.Int).SetString(table.capacity, 10)
		if !ok {
			check.Anchored = false
			check.Violations = append(check.Violations, "world table capacity cannot be decoded")
		} else if new(big.Int).SetInt64(int64(len(concreteTable.Rows))).Cmp(capacity) > 0 {
			check.Violations = append(check.Violations, "world table exceeds its declared capacity")
		}

		var previousKey []concreteKeyPart
		for rowIndex, concreteRecord := range concreteTable.Rows {
			key, validKey := document.checkConcreteRecord(&check, concreteRecord, table, record)
			if !validKey {
				continue
			}
			if rowIndex != 0 && compareConcreteKeys(previousKey, key) >= 0 {
				check.Violations = append(check.Violations, "world table rows are not in strict canonical primary-key order")
			}
			previousKey = key
		}
	}
	return finishWorldCheck(check)
}

func (document Document) checkConcreteRecord(
	check *WorldCheck,
	concrete ConcreteRecord,
	table tableDefinition,
	record recordDefinition,
) ([]concreteKeyPart, bool) {
	if concrete.RecordType != table.recordType {
		check.Violations = append(check.Violations, "world row record type differs from its interface table")
	}

	expectedNames := make([]string, 0, len(record.fields))
	for name := range record.fields {
		expectedNames = append(expectedNames, name)
	}
	sort.Strings(expectedNames)
	observed := make(map[string]ConcreteValue, len(concrete.Fields))
	var previous string
	for index, field := range concrete.Fields {
		if index != 0 && previous >= field.Name {
			check.Violations = append(check.Violations, "world row fields are not sorted and unique by name")
		}
		previous = field.Name
		if _, duplicate := observed[field.Name]; duplicate {
			check.Violations = append(check.Violations, "world row contains a duplicate field")
			continue
		}
		observed[field.Name] = field.Value
	}
	if len(observed) != len(expectedNames) {
		check.Violations = append(check.Violations, "world row does not contain the closed IR field set")
	}

	decoded := make(map[string]concreteKeyPart, len(expectedNames))
	for _, name := range expectedNames {
		field, exists := observed[name]
		if !exists {
			continue
		}
		part, valid := document.checkConcreteValue(check, record.fields[name].typeInfo, field)
		if valid {
			decoded[name] = part
		}
	}
	for name := range observed {
		if _, exists := record.fields[name]; !exists {
			check.Violations = append(check.Violations, "world row contains a field outside its closed IR record")
		}
	}

	key := make([]concreteKeyPart, 0, len(table.primaryKey))
	for _, name := range table.primaryKey {
		part, exists := decoded[name]
		if !exists {
			return nil, false
		}
		key = append(key, part)
	}
	return key, true
}

type concreteKeyPart struct {
	kind      valueKind
	truth     bool
	integer   *big.Int
	text      string
	enumOrder int
}

func (document Document) checkConcreteValue(
	check *WorldCheck,
	expected valueType,
	actual ConcreteValue,
) (concreteKeyPart, bool) {
	switch expected.kind {
	case boolValue:
		if actual.Kind != "bool" {
			check.Violations = append(check.Violations, "world value kind differs from IR Bool")
			return concreteKeyPart{}, false
		}
		return concreteKeyPart{kind: boolValue, truth: actual.Bool}, true
	case intValue:
		if actual.Kind != "int" {
			check.Violations = append(check.Violations, "world value kind differs from IR Int")
			return concreteKeyPart{}, false
		}
		integer, ok := parseConcreteInteger(actual.Integer)
		lower, lowerOK := new(big.Int).SetString(expected.lower, 10)
		upper, upperOK := new(big.Int).SetString(expected.upper, 10)
		if !ok || !lowerOK || !upperOK {
			check.Violations = append(check.Violations, "world integer or IR range cannot be decoded")
			return concreteKeyPart{}, false
		}
		if integer.Cmp(lower) < 0 || integer.Cmp(upper) > 0 {
			check.Violations = append(check.Violations, "world integer is outside its declared IR range")
		}
		return concreteKeyPart{kind: intValue, integer: integer}, true
	case textValue:
		if actual.Kind != "text" {
			check.Violations = append(check.Violations, "world value kind differs from IR Text")
			return concreteKeyPart{}, false
		}
		return concreteKeyPart{kind: textValue, text: actual.Text}, true
	case enumValue:
		if actual.Kind != "enum" || actual.EnumType != expected.enumType {
			check.Violations = append(check.Violations, "world enum type differs from its declared IR enum")
			return concreteKeyPart{}, false
		}
		enum, exists := document.enums[expected.enumType]
		if !exists {
			check.Anchored = false
			check.Violations = append(check.Violations, "world enum declaration is unavailable")
			return concreteKeyPart{}, false
		}
		order, exists := enum.memberOrder[actual.EnumMember]
		if !exists {
			check.Violations = append(check.Violations, "world enum member is not declared by its IR enum")
			return concreteKeyPart{}, false
		}
		return concreteKeyPart{kind: enumValue, enumOrder: order}, true
	default:
		check.Anchored = false
		check.Violations = append(check.Violations, "world field type is outside the retained concrete-data profile")
		return concreteKeyPart{}, false
	}
}

func parseConcreteInteger(raw string) (*big.Int, bool) {
	if raw == "" || raw == "-0" || raw[0] == '+' || len(raw) > 1 && raw[0] == '0' {
		return nil, false
	}
	start := 0
	if raw[0] == '-' {
		if len(raw) == 1 || raw[1] == '0' {
			return nil, false
		}
		start = 1
	}
	for index := start; index < len(raw); index++ {
		if raw[index] < '0' || raw[index] > '9' {
			return nil, false
		}
	}
	integer, ok := new(big.Int).SetString(raw, 10)
	return integer, ok
}

func compareConcreteKeys(left, right []concreteKeyPart) int {
	for index := range left {
		comparison := compareConcreteKeyPart(left[index], right[index])
		if comparison != 0 {
			return comparison
		}
	}
	return 0
}

func compareConcreteKeyPart(left, right concreteKeyPart) int {
	switch left.kind {
	case boolValue:
		if left.truth == right.truth {
			return 0
		}
		if !left.truth {
			return -1
		}
		return 1
	case intValue:
		return left.integer.Cmp(right.integer)
	case textValue:
		return strings.Compare(left.text, right.text)
	case enumValue:
		if left.enumOrder < right.enumOrder {
			return -1
		}
		if left.enumOrder > right.enumOrder {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func finishWorldCheck(check WorldCheck) WorldCheck {
	check.WellFormed = check.Anchored && len(check.Violations) == 0
	check.Violations = append([]string(nil), check.Violations...)
	return check
}

// WorldContainsProjection checks whether every projected table row appears in
// a distinct row of the full input world. It compares retained typed values
// exactly and does not infer omitted tables or fields.
func WorldContainsProjection(full, projection ConcreteWorld) bool {
	for _, projectedTable := range projection.Tables {
		var fullTable *ConcreteTable
		for tableIndex := range full.Tables {
			if full.Tables[tableIndex].Name == projectedTable.Name {
				fullTable = &full.Tables[tableIndex]
				break
			}
		}
		if fullTable == nil {
			return false
		}
		matched := make([]bool, len(fullTable.Rows))
		for _, projectedRow := range projectedTable.Rows {
			found := false
			for rowIndex, fullRow := range fullTable.Rows {
				if !matched[rowIndex] && equalConcreteRecord(fullRow, projectedRow) {
					matched[rowIndex] = true
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

func equalConcreteRecord(left, right ConcreteRecord) bool {
	if left.RecordType != right.RecordType || len(left.Fields) != len(right.Fields) {
		return false
	}
	for index := range left.Fields {
		if left.Fields[index].Name != right.Fields[index].Name || left.Fields[index].Value != right.Fields[index].Value {
			return false
		}
	}
	return true
}

func (p *parser) retainConcreteInterfaces() (
	map[string]protocol.Digest,
	map[string]protocol.Digest,
	[]protocol.Digest,
	error,
) {
	inputs := make(map[string]protocol.Digest, len(p.inputPorts))
	for name, nodeID := range p.inputPorts {
		node, exists := p.nodes[nodeID]
		if !exists || node.kind != "input" {
			return nil, nil, nil, rejection.New(rejection.InvalidJSON, "input interface node is unavailable during concrete-model retention")
		}
		inputs[name] = node.tableType
	}
	outputs := make(map[string]protocol.Digest, len(p.outputNames))
	for name, nodeID := range p.outputNames {
		node, exists := p.nodes[nodeID]
		if !exists {
			return nil, nil, nil, rejection.New(rejection.InvalidJSON, "output interface node is unavailable during concrete-model retention")
		}
		outputs[name] = node.tableType
	}
	assumes := make([]protocol.Digest, 0)
	for _, id := range p.contractOrder {
		contract := p.contracts[id]
		if contract.kind == "formula" && contract.role == "assume" {
			assumes = append(assumes, id)
		}
	}
	return inputs, outputs, assumes, nil
}

func cloneEnumDefinitions(source map[protocol.Digest]enumDefinition) map[protocol.Digest]enumDefinition {
	result := make(map[protocol.Digest]enumDefinition, len(source))
	for id, definition := range source {
		members := make(map[string]struct{}, len(definition.members))
		for member := range definition.members {
			members[member] = struct{}{}
		}
		orders := make(map[string]int, len(definition.memberOrder))
		for member, order := range definition.memberOrder {
			orders[member] = order
		}
		result[id] = enumDefinition{members: members, memberOrder: orders}
	}
	return result
}

func cloneRecordDefinitions(source map[protocol.Digest]recordDefinition) map[protocol.Digest]recordDefinition {
	result := make(map[protocol.Digest]recordDefinition, len(source))
	for id, definition := range source {
		fields := make(map[string]fieldDefinition, len(definition.fields))
		for name, field := range definition.fields {
			fields[name] = field
		}
		result[id] = recordDefinition{fields: fields}
	}
	return result
}

func cloneTableDefinitions(source map[protocol.Digest]tableDefinition) map[protocol.Digest]tableDefinition {
	result := make(map[protocol.Digest]tableDefinition, len(source))
	for id, definition := range source {
		definition.primaryKey = append([]string(nil), definition.primaryKey...)
		result[id] = definition
	}
	return result
}

func cloneContractDefinitions(source map[protocol.Digest]contractDefinition) map[protocol.Digest]contractDefinition {
	result := make(map[protocol.Digest]contractDefinition, len(source))
	for id, definition := range source {
		result[id] = definition
	}
	return result
}

func cloneNodeDefinitions(source map[protocol.Digest]nodeDefinition) map[protocol.Digest]nodeDefinition {
	result := make(map[protocol.Digest]nodeDefinition, len(source))
	for id, definition := range source {
		definition.predecessors = append([]protocol.Digest(nil), definition.predecessors...)
		definition.expressions = append([]nodeExpressionCheck(nil), definition.expressions...)
		definition.joinPairs = append([]joinPair(nil), definition.joinPairs...)
		definition.groupKeys = append([]groupKey(nil), definition.groupKeys...)
		definition.aggregates = append([]groupAggregate(nil), definition.aggregates...)
		result[id] = definition
	}
	return result
}

func cloneDigestMap(source map[string]protocol.Digest) map[string]protocol.Digest {
	result := make(map[string]protocol.Digest, len(source))
	for name, id := range source {
		result[name] = id
	}
	return result
}
