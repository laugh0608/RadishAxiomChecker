package axiomir

import (
	"sort"
	"strings"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

// BenchmarkData is the locked axiom-benchmark-data 0.1 projection. Envelope
// role remains separate from the Evidence execution I/O role that identifies
// host input, host output, actual output, or golden output.
type BenchmarkData struct {
	BenchmarkID string
	DataVersion string
	Role        string
	World       ConcreteWorld
}

type BenchmarkInput = BenchmarkData
type BenchmarkOutput = BenchmarkData

// DecodeBenchmarkInput strictly decodes the pretty JSON benchmark envelope
// without using the production codec. JSON object order and whitespace are not
// semantic; tables and rows retain their explicitly specified array order.
func (document Document) DecodeBenchmarkInput(data []byte, limits strictjson.Limits) (BenchmarkInput, error) {
	return document.decodeBenchmarkData(data, limits, document.inputTables, map[string]struct{}{
		"input":         {},
		"invalid-input": {},
	})
}

// DecodeBenchmarkOutput uses the same strict envelope, scalar, record, and
// table decoder as inputs, but resolves tables against the IR output interface.
// The locked corpus uses golden-output as the data-envelope role for both host
// and golden artifacts; their execution roles remain distinct in Evidence.
func (document Document) DecodeBenchmarkOutput(data []byte, limits strictjson.Limits) (BenchmarkOutput, error) {
	return document.decodeBenchmarkData(data, limits, document.outputTables, map[string]struct{}{
		"golden-output": {},
	})
}

func (document Document) decodeBenchmarkData(
	data []byte,
	limits strictjson.Limits,
	interfaces map[string]protocol.Digest,
	allowedRoles map[string]struct{},
) (BenchmarkData, error) {
	value, err := strictjson.ParseDocument(data, limits)
	if err != nil {
		return BenchmarkData{}, err
	}
	root, err := dataObject(value, "benchmark_id", "data_version", "format", "role", "tables")
	if err != nil {
		return BenchmarkData{}, err
	}
	if err := requireDataText(root["format"], "axiom-benchmark-data", rejection.UnknownTag, "unknown benchmark data format"); err != nil {
		return BenchmarkData{}, err
	}
	if err := requireDataText(root["data_version"], "0.1", rejection.UnsupportedVersion, "unsupported benchmark data version"); err != nil {
		return BenchmarkData{}, err
	}
	benchmarkID, err := nonemptyDataText(root["benchmark_id"])
	if err != nil {
		return BenchmarkData{}, err
	}
	expectedBenchmark, ok := document.lockedBenchmarkID()
	if !ok || benchmarkID != expectedBenchmark {
		return BenchmarkData{}, rejection.New(rejection.ConcreteCheckMismatch, "benchmark ID does not match the locked IR interface profile")
	}
	role, err := nonemptyDataText(root["role"])
	if err != nil {
		return BenchmarkData{}, err
	}
	if _, allowed := allowedRoles[role]; !allowed {
		return BenchmarkData{}, rejection.New(rejection.UnknownTag, "benchmark data role is outside the selected interface profile")
	}
	tableValues, err := dataArray(root["tables"])
	if err != nil {
		return BenchmarkData{}, err
	}
	tables := make([]ConcreteTable, 0, len(tableValues))
	var previous string
	for index, tableValue := range tableValues {
		table, err := document.decodeBenchmarkTable(tableValue, interfaces)
		if err != nil {
			return BenchmarkData{}, err
		}
		if index != 0 && previous >= table.Name {
			return BenchmarkData{}, rejection.New(rejection.NoncanonicalOrder, "benchmark tables are not sorted and unique by interface name")
		}
		previous = table.Name
		tables = append(tables, table)
	}
	return BenchmarkData{
		BenchmarkID: benchmarkID,
		DataVersion: "0.1",
		Role:        role,
		World:       ConcreteWorld{Tables: tables},
	}, nil
}

func (document Document) decodeBenchmarkTable(
	value strictjson.Value,
	interfaces map[string]protocol.Digest,
) (ConcreteTable, error) {
	fields, err := dataObject(value, "name", "rows")
	if err != nil {
		return ConcreteTable{}, err
	}
	name, err := nonemptyDataText(fields["name"])
	if err != nil {
		return ConcreteTable{}, err
	}
	rowValues, err := dataArray(fields["rows"])
	if err != nil {
		return ConcreteTable{}, err
	}
	var recordID protocol.Digest
	var record *recordDefinition
	if tableID, exists := interfaces[name]; exists {
		if table, tableExists := document.tables[tableID]; tableExists {
			recordID = table.recordType
			if definition, recordExists := document.records[recordID]; recordExists {
				copy := definition
				record = &copy
			}
		}
	}
	rows := make([]ConcreteRecord, 0, len(rowValues))
	for _, rowValue := range rowValues {
		row, err := decodeBenchmarkRow(rowValue, recordID, record)
		if err != nil {
			return ConcreteTable{}, err
		}
		rows = append(rows, row)
	}
	return ConcreteTable{Name: name, Rows: rows}, nil
}

func decodeBenchmarkRow(
	value strictjson.Value,
	recordID protocol.Digest,
	record *recordDefinition,
) (ConcreteRecord, error) {
	members, ok := value.Members()
	if !ok {
		return ConcreteRecord{}, rejection.New(rejection.InvalidJSON, "benchmark row must be a JSON object")
	}
	fields := make([]ConcreteField, 0, len(members))
	for _, member := range members {
		if member.Name == "" {
			return ConcreteRecord{}, rejection.New(rejection.InvalidJSON, "benchmark row field name must not be empty")
		}
		var expected *valueType
		if record != nil {
			if definition, exists := record.fields[member.Name]; exists {
				copy := definition.typeInfo
				expected = &copy
			}
		}
		fields = append(fields, ConcreteField{
			Name:  member.Name,
			Value: decodeBenchmarkValue(member.Value, expected),
		})
	}
	sort.Slice(fields, func(left, right int) bool { return fields[left].Name < fields[right].Name })
	return ConcreteRecord{RecordType: recordID, Fields: fields}, nil
}

func decodeBenchmarkValue(value strictjson.Value, expected *valueType) ConcreteValue {
	if truth, ok := value.Bool(); ok {
		return ConcreteValue{Kind: "bool", Bool: truth}
	}
	spelling, ok := value.Text()
	if !ok {
		return ConcreteValue{Kind: "invalid"}
	}
	if expected == nil {
		return ConcreteValue{Kind: "untyped-string", Text: spelling}
	}
	switch expected.kind {
	case intValue:
		return ConcreteValue{Kind: "int", Integer: spelling}
	case enumValue:
		return ConcreteValue{Kind: "enum", EnumType: expected.enumType, EnumMember: spelling}
	case textValue:
		return ConcreteValue{Kind: "text", Text: spelling}
	default:
		return ConcreteValue{Kind: "text", Text: spelling}
	}
}

func (document Document) lockedBenchmarkID() (string, bool) {
	switch strings.Join(document.inputInterfaces, "\x00") {
	case "orders":
		return "AX-B01", true
	case "customers\x00orders":
		return "AX-B02", true
	case "usage_events":
		return "AX-B03", true
	case "tickets":
		return "AX-B04", true
	default:
		return "", false
	}
}

// LogicalBytes is a deterministic upper-accounting input for the checker
// working-memory boundary. It does not claim to equal Go heap allocation.
func (data BenchmarkData) LogicalBytes() uint64 {
	total := uint64(len(data.BenchmarkID) + len(data.DataVersion) + len(data.Role))
	for _, table := range data.World.Tables {
		total = saturatingAdd(total, uint64(len(table.Name)))
		for _, row := range table.Rows {
			total = saturatingAdd(total, 32)
			for _, field := range row.Fields {
				total = saturatingAdd(total, uint64(len(field.Name)+len(field.Value.Kind)))
				total = saturatingAdd(total, uint64(len(field.Value.Integer)+len(field.Value.Text)+len(field.Value.EnumMember)))
				if field.Value.Kind == "enum" {
					total = saturatingAdd(total, 32)
				}
			}
		}
	}
	return total
}

func saturatingAdd(left, right uint64) uint64 {
	result := left + right
	if result < left {
		return ^uint64(0)
	}
	return result
}

func dataObject(value strictjson.Value, required ...string) (map[string]strictjson.Value, error) {
	members, ok := value.Members()
	if !ok {
		return nil, rejection.New(rejection.InvalidJSON, "benchmark data value must be an object")
	}
	allowed := make(map[string]struct{}, len(required))
	for _, name := range required {
		allowed[name] = struct{}{}
	}
	result := make(map[string]strictjson.Value, len(members))
	for _, member := range members {
		if _, exists := allowed[member.Name]; !exists {
			return nil, rejection.New(rejection.UnknownMember, "benchmark data object contains an unknown member")
		}
		result[member.Name] = member.Value
	}
	for _, name := range required {
		if _, exists := result[name]; !exists {
			return nil, rejection.New(rejection.MissingRequiredMember, "benchmark data object is missing a required member")
		}
	}
	return result, nil
}

func dataArray(value strictjson.Value) ([]strictjson.Value, error) {
	items, ok := value.Items()
	if !ok {
		return nil, rejection.New(rejection.InvalidJSON, "benchmark data value must be an array")
	}
	return items, nil
}

func nonemptyDataText(value strictjson.Value) (string, error) {
	text, ok := value.Text()
	if !ok || text == "" {
		return "", rejection.New(rejection.InvalidJSON, "benchmark data member must be a nonempty string")
	}
	return text, nil
}

func requireDataText(value strictjson.Value, expected string, code rejection.Code, detail string) error {
	actual, ok := value.Text()
	if !ok {
		return rejection.New(rejection.InvalidJSON, "benchmark data member must be a string")
	}
	if actual != expected {
		return rejection.New(code, detail)
	}
	return nil
}
