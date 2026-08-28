package axiomir

import (
	"fmt"
	"math/big"
	"sort"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/resourcebudget"
)

// ExecutionLimits binds finite IR interpretation to explicit deterministic
// budgets. Logical bytes are an upper-accounting model, not Go heap usage.
type ExecutionLimits struct {
	MaxLogicalBytes  uint64
	MaxSemanticSteps uint64
	Ledger           *resourcebudget.Ledger
	Owner            string
}

// ExecutionFailure is an observed semantic failure of one well-formed node.
// It is data for target replay, not evidence that an obligation failed until
// the caller binds the node and failure kind to the expected obligation.
type ExecutionFailure struct {
	Node   protocol.Digest
	Kind   string
	Detail string
}

func (failure *ExecutionFailure) Error() string {
	if failure.Node == (protocol.Digest{}) {
		return fmt.Sprintf("%s: %s", failure.Kind, failure.Detail)
	}
	return fmt.Sprintf("%s at %s: %s", failure.Kind, failure.Node.String(), failure.Detail)
}

// ProgramExecution retains deterministic node tables for target replay. The
// maps are private so callers cannot mutate the checker's interpreted state.
type ProgramExecution struct {
	Outputs      ConcreteWorld
	Steps        uint64
	LogicalBytes uint64

	nodes map[protocol.Digest]ConcreteTable
}

// NodeTable returns a defensive copy of one interpreted node table.
func (execution ProgramExecution) NodeTable(id protocol.Digest) (ConcreteTable, bool) {
	table, exists := execution.nodes[id]
	if !exists {
		return ConcreteTable{}, false
	}
	return cloneConcreteTable(table), true
}

// Execute interprets the retained five-node Axiom IR DAG over one complete WF
// and Pre-satisfying input world. It never executes a production target.
func (document Document) Execute(world ConcreteWorld, limits ExecutionLimits) (ProgramExecution, error) {
	check := document.CheckCompleteInputWorld(world)
	if !check.WellFormed {
		return ProgramExecution{}, rejection.New(rejection.ConcreteCheckMismatch, "IR execution requires a complete WF input world")
	}
	pre, err := document.EvaluateAssumesWithLimits(world, limits)
	if err != nil {
		return ProgramExecution{}, err
	}
	if !pre.AllTrue {
		return ProgramExecution{}, rejection.New(rejection.ConcreteCheckMismatch, "IR execution requires a Pre-satisfying input world")
	}

	inputTables := make(map[string]ConcreteTable, len(world.Tables))
	for _, table := range world.Tables {
		inputTables[table.Name] = cloneConcreteTable(table)
	}
	order, err := document.executionOrder()
	if err != nil {
		return ProgramExecution{}, err
	}
	evaluator := concreteEvaluator{
		document: document, input: copyConcreteWorld(world),
		maxSteps: limits.MaxSemanticSteps, ledger: limits.Ledger,
	}
	nodeTables := make(map[protocol.Digest]ConcreteTable, len(order))
	logicalBytes := concreteWorldLogicalBytes(world)
	if limits.MaxLogicalBytes == 0 || logicalBytes > limits.MaxLogicalBytes {
		return ProgramExecution{}, rejection.New(rejection.ResourceLimit, "IR execution input exceeds its logical-memory limit")
	}
	owner := limits.Owner
	if owner == "" {
		owner = "axiom-ir-execution"
	}
	if limits.Ledger != nil {
		if err := limits.Ledger.AcquireLogical(owner, logicalBytes); err != nil {
			return ProgramExecution{}, err
		}
	}
	for _, id := range order {
		if err := evaluator.step(); err != nil {
			return ProgramExecution{}, err
		}
		node := document.nodes[id]
		table, err := document.executeNode(id, node, nodeTables, inputTables, &evaluator)
		if err != nil {
			if failure, ok := err.(*ExecutionFailure); ok && failure.Node == (protocol.Digest{}) {
				failure.Node = id
			}
			return ProgramExecution{}, err
		}
		if err := document.validateInterpretedTable(id, node.tableType, table); err != nil {
			return ProgramExecution{}, err
		}
		nodeTables[id] = table
		logicalBytes = saturatingAdd(logicalBytes, concreteTableLogicalBytes(table))
		if logicalBytes > limits.MaxLogicalBytes {
			return ProgramExecution{}, rejection.New(rejection.ResourceLimit, "IR execution exceeds its logical-memory limit")
		}
		if limits.Ledger != nil {
			if err := limits.Ledger.AcquireLogical(owner, logicalBytes); err != nil {
				return ProgramExecution{}, err
			}
		}
	}

	outputs := ConcreteWorld{Tables: make([]ConcreteTable, 0, len(document.outputInterfaces))}
	for _, name := range document.outputInterfaces {
		id, exists := document.outputNodes[name]
		if !exists {
			return ProgramExecution{}, rejection.New(rejection.ConcreteCheckMismatch, "IR output node binding is unavailable")
		}
		table, exists := nodeTables[id]
		if !exists {
			return ProgramExecution{}, rejection.New(rejection.ConcreteCheckMismatch, "IR output node was not interpreted")
		}
		copy := cloneConcreteTable(table)
		copy.Name = name
		outputs.Tables = append(outputs.Tables, copy)
	}
	if check := document.CheckOutputWorld(outputs); !check.WellFormed {
		return ProgramExecution{}, &ExecutionFailure{Kind: "output-wf", Detail: "interpreted output world is not WF"}
	}
	return ProgramExecution{
		Outputs:      outputs,
		Steps:        evaluator.steps,
		LogicalBytes: logicalBytes,
		nodes:        cloneConcreteTableMap(nodeTables),
	}, nil
}

func (document Document) executeNode(
	id protocol.Digest,
	node nodeDefinition,
	computed map[protocol.Digest]ConcreteTable,
	inputs map[string]ConcreteTable,
	evaluator *concreteEvaluator,
) (ConcreteTable, error) {
	switch node.kind {
	case "input":
		table, exists := inputs[node.port]
		if !exists {
			return ConcreteTable{}, rejection.New(rejection.ConcreteCheckMismatch, "input node cannot resolve its concrete port")
		}
		return cloneConcreteTable(table), nil
	case "filter":
		source, err := predecessorTable(computed, node.predecessors[0])
		if err != nil {
			return ConcreteTable{}, err
		}
		fields, err := object(node.definition, "kind", "predicate", "source", "table_type")
		if err != nil {
			return ConcreteTable{}, err
		}
		result := ConcreteTable{Rows: make([]ConcreteRecord, 0, len(source.Rows))}
		for rowIndex := range source.Rows {
			if err := evaluator.step(); err != nil {
				return ConcreteTable{}, err
			}
			predicate, err := evaluator.evaluate(fields["predicate"], []evaluationValue{{kind: recordValue, record: &source.Rows[rowIndex]}})
			if err != nil || predicate.kind != boolValue {
				return ConcreteTable{}, concreteBoolError(err, "filter predicate evaluation requires Bool")
			}
			if predicate.truth {
				result.Rows = append(result.Rows, cloneConcreteRecord(source.Rows[rowIndex]))
			}
		}
		return result, nil
	case "map":
		source, err := predecessorTable(computed, node.predecessors[0])
		if err != nil {
			return ConcreteTable{}, err
		}
		return document.projectRows(node, []ConcreteTable{source}, evaluator)
	case "lookup_join":
		left, err := predecessorTable(computed, node.predecessors[0])
		if err != nil {
			return ConcreteTable{}, err
		}
		right, err := predecessorTable(computed, node.predecessors[1])
		if err != nil {
			return ConcreteTable{}, err
		}
		result := ConcreteTable{Rows: make([]ConcreteRecord, 0, len(left.Rows))}
		for leftIndex := range left.Rows {
			matches := make([]int, 0, 1)
			for rightIndex := range right.Rows {
				if err := evaluator.step(); err != nil {
					return ConcreteTable{}, err
				}
				equal, err := joinRowsEqual(left.Rows[leftIndex], right.Rows[rightIndex], node.joinPairs)
				if err != nil {
					return ConcreteTable{}, err
				}
				if equal {
					matches = append(matches, rightIndex)
				}
			}
			if len(matches) == 0 {
				return ConcreteTable{}, &ExecutionFailure{Node: id, Kind: "totality", Detail: "lookup_join found no right match for a left row"}
			}
			if len(matches) != 1 {
				return ConcreteTable{}, &ExecutionFailure{Node: id, Kind: "key-cardinality", Detail: "lookup_join found multiple right matches for a left row"}
			}
			row, err := document.projectRecord(node, []evaluationValue{
				{kind: recordValue, record: &left.Rows[leftIndex]},
				{kind: recordValue, record: &right.Rows[matches[0]]},
			}, evaluator)
			if err != nil {
				return ConcreteTable{}, err
			}
			result.Rows = append(result.Rows, row)
		}
		return result, nil
	case "group":
		source, err := predecessorTable(computed, node.predecessors[0])
		if err != nil {
			return ConcreteTable{}, err
		}
		return document.groupRows(node, source, evaluator)
	default:
		return ConcreteTable{}, rejection.New(rejection.ConcreteCheckMismatch, "IR execution encountered an unknown node kind")
	}
}

func (document Document) projectRows(
	node nodeDefinition,
	sources []ConcreteTable,
	evaluator *concreteEvaluator,
) (ConcreteTable, error) {
	result := ConcreteTable{Rows: make([]ConcreteRecord, 0, len(sources[0].Rows))}
	for rowIndex := range sources[0].Rows {
		if err := evaluator.step(); err != nil {
			return ConcreteTable{}, err
		}
		row, err := document.projectRecord(node, []evaluationValue{{kind: recordValue, record: &sources[0].Rows[rowIndex]}}, evaluator)
		if err != nil {
			return ConcreteTable{}, err
		}
		result.Rows = append(result.Rows, row)
	}
	return result, nil
}

func (document Document) projectRecord(
	node nodeDefinition,
	environment []evaluationValue,
	evaluator *concreteEvaluator,
) (ConcreteRecord, error) {
	table := document.tables[node.tableType]
	row := ConcreteRecord{RecordType: table.recordType, Fields: make([]ConcreteField, 0, len(node.expressions))}
	for _, projection := range node.expressions {
		value, err := evaluator.evaluate(projection.value, environment)
		if err != nil {
			return ConcreteRecord{}, err
		}
		concrete, err := concreteEvaluationValue(value)
		if err != nil {
			return ConcreteRecord{}, err
		}
		row.Fields = append(row.Fields, ConcreteField{Name: projection.fieldName, Value: concrete})
	}
	return row, nil
}

type concreteGroup struct {
	keyValues []ConcreteValue
	keyParts  []concreteKeyPart
	rows      []ConcreteRecord
}

func (document Document) groupRows(
	node nodeDefinition,
	source ConcreteTable,
	evaluator *concreteEvaluator,
) (ConcreteTable, error) {
	groups := make([]concreteGroup, 0)
	outputTable := document.tables[node.tableType]
	outputRecord := document.records[outputTable.recordType]
	for rowIndex := range source.Rows {
		if err := evaluator.step(); err != nil {
			return ConcreteTable{}, err
		}
		keys := make([]ConcreteValue, 0, len(node.groupKeys))
		for _, key := range node.groupKeys {
			value, exists := concreteRecordField(source.Rows[rowIndex], key.sourceField)
			if !exists {
				return ConcreteTable{}, rejection.New(rejection.ConcreteCheckMismatch, "group source row omits a retained key field")
			}
			keys = append(keys, value)
		}
		groupIndex := -1
		for index := range groups {
			if equalConcreteValues(groups[index].keyValues, keys) {
				groupIndex = index
				break
			}
		}
		if groupIndex < 0 {
			parts := make([]concreteKeyPart, len(keys))
			for index, key := range node.groupKeys {
				field := outputRecord.fields[key.name]
				check := WorldCheck{Anchored: true}
				part, valid := document.checkConcreteValue(&check, field.typeInfo, keys[index])
				if !valid || len(check.Violations) != 0 {
					return ConcreteTable{}, rejection.New(rejection.ConcreteCheckMismatch, "group key cannot be decoded against the output table")
				}
				parts[index] = part
			}
			groups = append(groups, concreteGroup{keyValues: keys, keyParts: parts})
			groupIndex = len(groups) - 1
		}
		groups[groupIndex].rows = append(groups[groupIndex].rows, source.Rows[rowIndex])
	}
	sort.Slice(groups, func(left, right int) bool {
		return compareConcreteKeys(groups[left].keyParts, groups[right].keyParts) < 0
	})
	result := ConcreteTable{Rows: make([]ConcreteRecord, 0, len(groups))}
	for _, group := range groups {
		row := ConcreteRecord{RecordType: outputTable.recordType, Fields: make([]ConcreteField, 0, len(node.groupKeys)+len(node.aggregates))}
		for index, key := range node.groupKeys {
			row.Fields = append(row.Fields, ConcreteField{Name: key.name, Value: group.keyValues[index]})
		}
		for _, aggregate := range node.aggregates {
			integer := new(big.Int)
			switch aggregate.kind {
			case "count":
				integer.SetUint64(uint64(len(group.rows)))
			case "sum":
				for _, sourceRow := range group.rows {
					if err := evaluator.step(); err != nil {
						return ConcreteTable{}, err
					}
					value, exists := concreteRecordField(sourceRow, aggregate.field)
					if !exists || value.Kind != "int" {
						return ConcreteTable{}, rejection.New(rejection.ConcreteCheckMismatch, "group sum source field is not a concrete Int")
					}
					term, ok := parseConcreteInteger(value.Integer)
					if !ok {
						return ConcreteTable{}, rejection.New(rejection.ConcreteCheckMismatch, "group sum source integer is not canonical")
					}
					integer.Add(integer, term)
				}
			default:
				return ConcreteTable{}, rejection.New(rejection.ConcreteCheckMismatch, "group execution encountered an unknown aggregate")
			}
			field := outputRecord.fields[aggregate.name]
			if field.typeInfo.kind != intValue {
				return ConcreteTable{}, rejection.New(rejection.ConcreteCheckMismatch, "group aggregate output is not declared Int")
			}
			lower, _ := new(big.Int).SetString(field.typeInfo.lower, 10)
			upper, _ := new(big.Int).SetString(field.typeInfo.upper, 10)
			if lower == nil || upper == nil || integer.Cmp(lower) < 0 || integer.Cmp(upper) > 0 {
				return ConcreteTable{}, &ExecutionFailure{Kind: "numeric-range", Detail: "group aggregate is outside its declared range"}
			}
			row.Fields = append(row.Fields, ConcreteField{Name: aggregate.name, Value: ConcreteValue{Kind: "int", Integer: integer.String()}})
		}
		sort.Slice(row.Fields, func(left, right int) bool { return row.Fields[left].Name < row.Fields[right].Name })
		result.Rows = append(result.Rows, row)
	}
	return result, nil
}

// EvaluateGuarantee evaluates one retained guarantee over independently
// interpreted input and output worlds. False is a dynamic counterexample fact,
// not a proof about all inputs.
func (document Document) EvaluateGuarantee(
	id protocol.Digest,
	input ConcreteWorld,
	output ConcreteWorld,
	maxSteps uint64,
) (bool, error) {
	return document.EvaluateGuaranteeWithLimits(
		id, input, output, ExecutionLimits{MaxSemanticSteps: maxSteps},
	)
}

func (document Document) EvaluateGuaranteeWithLimits(
	id protocol.Digest,
	input ConcreteWorld,
	output ConcreteWorld,
	limits ExecutionLimits,
) (bool, error) {
	contract, exists := document.contracts[id]
	if !exists || contract.kind != "formula" || contract.role != "guarantee" {
		return false, rejection.New(rejection.ConcreteCheckMismatch, "target guarantee contract is unavailable")
	}
	fields, err := object(contract.definition, "expression", "kind", "role")
	if err != nil {
		return false, err
	}
	evaluator := concreteEvaluator{
		document: document, input: input, output: output,
		maxSteps: limits.MaxSemanticSteps, ledger: limits.Ledger,
	}
	value, err := evaluator.evaluate(fields["expression"], nil)
	if err != nil {
		return false, err
	}
	if value.kind != boolValue {
		return false, rejection.New(rejection.ConcreteCheckMismatch, "guarantee evaluation did not produce Bool")
	}
	return value.truth, nil
}

func (document Document) executionOrder() ([]protocol.Digest, error) {
	order := make([]protocol.Digest, 0, len(document.nodes))
	colors := make(map[protocol.Digest]uint8, len(document.nodes))
	var visit func(protocol.Digest) error
	visit = func(id protocol.Digest) error {
		switch colors[id] {
		case 1:
			return rejection.New(rejection.ConcreteCheckMismatch, "retained IR node graph contains a cycle")
		case 2:
			return nil
		}
		node, exists := document.nodes[id]
		if !exists {
			return rejection.New(rejection.ConcreteCheckMismatch, "retained IR node is unavailable")
		}
		colors[id] = 1
		for _, predecessor := range node.predecessors {
			if err := visit(predecessor); err != nil {
				return err
			}
		}
		colors[id] = 2
		order = append(order, id)
		return nil
	}
	for _, id := range document.nodeOrder {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	return order, nil
}

func (document Document) validateInterpretedTable(
	nodeID protocol.Digest,
	tableID protocol.Digest,
	concrete ConcreteTable,
) error {
	table, exists := document.tables[tableID]
	if !exists {
		return rejection.New(rejection.ConcreteCheckMismatch, "interpreted table declaration is unavailable")
	}
	record, exists := document.records[table.recordType]
	if !exists {
		return rejection.New(rejection.ConcreteCheckMismatch, "interpreted record declaration is unavailable")
	}
	capacity, ok := new(big.Int).SetString(table.capacity, 10)
	if !ok || new(big.Int).SetUint64(uint64(len(concrete.Rows))).Cmp(capacity) > 0 {
		return &ExecutionFailure{Node: nodeID, Kind: "key-cardinality", Detail: "interpreted table exceeds its declared capacity"}
	}
	var previous []concreteKeyPart
	for index, row := range concrete.Rows {
		check := WorldCheck{Anchored: true}
		key, valid := document.checkConcreteRecord(&check, row, table, record)
		if !valid || len(check.Violations) != 0 {
			return &ExecutionFailure{Node: nodeID, Kind: "table-wf", Detail: "interpreted row violates its declared table type"}
		}
		if index != 0 && compareConcreteKeys(previous, key) >= 0 {
			return &ExecutionFailure{Node: nodeID, Kind: "key-cardinality", Detail: "interpreted table keys are duplicate or noncanonical"}
		}
		previous = key
	}
	return nil
}

func predecessorTable(computed map[protocol.Digest]ConcreteTable, id protocol.Digest) (ConcreteTable, error) {
	table, exists := computed[id]
	if !exists {
		return ConcreteTable{}, rejection.New(rejection.ConcreteCheckMismatch, "IR predecessor was not interpreted before its consumer")
	}
	return table, nil
}

func joinRowsEqual(left, right ConcreteRecord, pairs []joinPair) (bool, error) {
	for _, pair := range pairs {
		leftValue, leftExists := concreteRecordField(left, pair.left)
		rightValue, rightExists := concreteRecordField(right, pair.right)
		if !leftExists || !rightExists {
			return false, rejection.New(rejection.ConcreteCheckMismatch, "lookup_join row omits a retained pair field")
		}
		if leftValue != rightValue {
			return false, nil
		}
	}
	return true, nil
}

func concreteEvaluationValue(value evaluationValue) (ConcreteValue, error) {
	switch value.kind {
	case boolValue:
		return ConcreteValue{Kind: "bool", Bool: value.truth}, nil
	case intValue:
		if value.integer == nil {
			return ConcreteValue{}, rejection.New(rejection.ConcreteCheckMismatch, "evaluated Int has no mathematical value")
		}
		return ConcreteValue{Kind: "int", Integer: value.integer.String()}, nil
	case textValue:
		return ConcreteValue{Kind: "text", Text: value.text}, nil
	case enumValue:
		return ConcreteValue{Kind: "enum", EnumType: value.enumType, EnumMember: value.enumMember}, nil
	default:
		return ConcreteValue{}, rejection.New(rejection.ConcreteCheckMismatch, "projection expression did not produce a scalar value")
	}
}

func equalConcreteValues(left, right []ConcreteValue) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func copyConcreteWorld(world ConcreteWorld) ConcreteWorld {
	result := ConcreteWorld{Tables: make([]ConcreteTable, len(world.Tables))}
	for index, table := range world.Tables {
		result.Tables[index] = cloneConcreteTable(table)
	}
	return result
}

func cloneConcreteTable(table ConcreteTable) ConcreteTable {
	result := ConcreteTable{Name: table.Name, Rows: make([]ConcreteRecord, len(table.Rows))}
	for index, row := range table.Rows {
		result.Rows[index] = cloneConcreteRecord(row)
	}
	return result
}

func cloneConcreteRecord(record ConcreteRecord) ConcreteRecord {
	return ConcreteRecord{RecordType: record.RecordType, Fields: append([]ConcreteField(nil), record.Fields...)}
}

func cloneConcreteTableMap(source map[protocol.Digest]ConcreteTable) map[protocol.Digest]ConcreteTable {
	result := make(map[protocol.Digest]ConcreteTable, len(source))
	for id, table := range source {
		result[id] = cloneConcreteTable(table)
	}
	return result
}

func concreteWorldLogicalBytes(world ConcreteWorld) uint64 {
	total := uint64(0)
	for _, table := range world.Tables {
		total = saturatingAdd(total, concreteTableLogicalBytes(table))
	}
	return total
}

func concreteTableLogicalBytes(table ConcreteTable) uint64 {
	total := uint64(len(table.Name))
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
	return total
}
