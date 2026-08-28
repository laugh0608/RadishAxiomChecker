package axiomir

import (
	"errors"
	"sort"
	"strings"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

// ReplayFailureTarget checks the dynamic fact named by one locked failed proof
// obligation. It is deliberately narrower than proof: one finite witness can
// refute a universal obligation but cannot establish a universal property.
func (document Document) ReplayFailureTarget(
	definition ObligationDefinition,
	worlds []ConcreteWorld,
	requiredFields []string,
	requiredKeys []string,
	limits ExecutionLimits,
) (bool, error) {
	if definition.Expectation != "prove" {
		return false, rejection.New(rejection.ConcreteCheckMismatch, "counterexample target is not a proof obligation")
	}
	if !document.knownWitnessFields(requiredFields) {
		return false, rejection.New(rejection.ConcreteCheckMismatch, "counterexample required field does not resolve to the IR interface profile")
	}
	if !document.witnessKeysPresent(worlds, requiredKeys) {
		return false, rejection.New(rejection.ConcreteCheckMismatch, "counterexample required key is absent from its retained worlds")
	}

	switch definition.Kind {
	case "contract-guarantee":
		if len(worlds) != 1 || definition.Subject.Kind != "contract" {
			return false, rejection.New(rejection.ConcreteCheckMismatch, "contract-guarantee target has the wrong witness shape")
		}
		limits.Owner = "counterexample:" + definition.Kind + ":" + definition.Subject.ID.String()
		if limits.Ledger != nil {
			defer limits.Ledger.ReleaseLogical(limits.Owner)
		}
		execution, err := document.Execute(worlds[0], limits)
		if err != nil {
			return false, err
		}
		truth, err := document.EvaluateGuaranteeWithLimits(definition.Subject.ID, worlds[0], execution.Outputs, limits)
		return !truth, err
	case "noninterference":
		if len(worlds) != 2 || definition.Subject.Kind != "contract" {
			return false, rejection.New(rejection.ConcreteCheckMismatch, "noninterference target has the wrong witness shape")
		}
		return document.replayNoninterference(definition.Subject.ID, worlds[0], worlds[1], limits)
	case "key-cardinality":
		if len(worlds) != 1 || definition.Subject.Kind != "node" {
			return false, rejection.New(rejection.ConcreteCheckMismatch, "key-cardinality target has the wrong witness shape")
		}
		limits.Owner = "counterexample:" + definition.Kind + ":" + definition.Subject.ID.String()
		if limits.Ledger != nil {
			defer limits.Ledger.ReleaseLogical(limits.Owner)
		}
		_, err := document.Execute(worlds[0], limits)
		var failure *ExecutionFailure
		if errors.As(err, &failure) {
			return failure.Node == definition.Subject.ID && failure.Kind == "key-cardinality", nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	case "field-origin":
		if len(worlds) != 1 || definition.Subject.Kind != "field" {
			return false, rejection.New(rejection.ConcreteCheckMismatch, "field-origin target has the wrong witness shape")
		}
		limits.Owner = "counterexample:" + definition.Kind + ":" + definition.Subject.ID.String()
		if limits.Ledger != nil {
			defer limits.Ledger.ReleaseLogical(limits.Owner)
		}
		execution, err := document.Execute(worlds[0], limits)
		if err != nil {
			return false, err
		}
		missingOrigin, err := document.outputFieldOmitsRequiredOrigin(definition.Subject, requiredFields)
		if err != nil || !missingOrigin {
			return false, err
		}
		guaranteeFailed, err := document.anyGuaranteeFalse(worlds[0], execution.Outputs, limits)
		return guaranteeFailed, err
	case "row-coverage":
		if len(worlds) != 1 || definition.Subject.Kind != "node" {
			return false, rejection.New(rejection.ConcreteCheckMismatch, "row-coverage target has the wrong witness shape")
		}
		limits.Owner = "counterexample:" + definition.Kind + ":" + definition.Subject.ID.String()
		if limits.Ledger != nil {
			defer limits.Ledger.ReleaseLogical(limits.Owner)
		}
		execution, err := document.Execute(worlds[0], limits)
		if err != nil {
			return false, err
		}
		dropped, err := document.targetNodeDroppedRows(definition.Subject.ID, execution)
		if err != nil || !dropped {
			return false, err
		}
		guaranteeFailed, err := document.anyGuaranteeFalse(worlds[0], execution.Outputs, limits)
		return guaranteeFailed, err
	case "group-conservation":
		if len(worlds) != 1 || definition.Subject.Kind != "node" {
			return false, rejection.New(rejection.ConcreteCheckMismatch, "group-conservation target has the wrong witness shape")
		}
		limits.Owner = "counterexample:" + definition.Kind + ":" + definition.Subject.ID.String()
		if limits.Ledger != nil {
			defer limits.Ledger.ReleaseLogical(limits.Owner)
		}
		execution, err := document.Execute(worlds[0], limits)
		if err != nil {
			return false, err
		}
		collapsed, err := document.targetGroupCollapsedWitness(definition.Subject.ID, worlds[0], execution, requiredFields)
		if err != nil || !collapsed {
			return false, err
		}
		guaranteeFailed, err := document.anyGuaranteeFalse(worlds[0], execution.Outputs, limits)
		return guaranteeFailed, err
	default:
		return false, rejection.New(rejection.ConcreteCheckMismatch, "failed proof target kind is outside the locked replay profile")
	}
}

func (document Document) anyGuaranteeFalse(
	input, output ConcreteWorld,
	limits ExecutionLimits,
) (bool, error) {
	ids := make([]protocol.Digest, 0)
	for id, contract := range document.contracts {
		if contract.kind == "formula" && contract.role == "guarantee" {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(left, right int) bool { return ids[left].String() < ids[right].String() })
	if len(ids) == 0 {
		return false, rejection.New(rejection.ConcreteCheckMismatch, "locked target replay has no guarantee contract")
	}
	for _, id := range ids {
		truth, err := document.EvaluateGuaranteeWithLimits(id, input, output, limits)
		if err != nil {
			return false, err
		}
		if !truth {
			return true, nil
		}
	}
	return false, nil
}

func (document Document) replayNoninterference(
	contractID protocol.Digest,
	leftInput ConcreteWorld,
	rightInput ConcreteWorld,
	limits ExecutionLimits,
) (bool, error) {
	contract, exists := document.contracts[contractID]
	if !exists || contract.kind != "noninterference" {
		return false, rejection.New(rejection.ConcreteCheckMismatch, "noninterference contract is unavailable")
	}
	fields, err := object(contract.definition, "inputs", "kind", "outputs")
	if err != nil {
		return false, err
	}
	inputs, err := namesFromValue(fields["inputs"])
	if err != nil {
		return false, err
	}
	outputs, err := namesFromValue(fields["outputs"])
	if err != nil {
		return false, err
	}
	if !document.publicInputsEqual(leftInput, rightInput, inputs) {
		return false, rejection.New(rejection.ConcreteCheckMismatch, "paired counterexample inputs are not publicly equivalent")
	}
	leftLimits := limits
	leftLimits.Owner = "noninterference:left:" + contractID.String()
	rightLimits := limits
	rightLimits.Owner = "noninterference:right:" + contractID.String()
	if limits.Ledger != nil {
		defer limits.Ledger.ReleaseLogical(leftLimits.Owner)
		defer limits.Ledger.ReleaseLogical(rightLimits.Owner)
	}
	left, leftErr := document.Execute(leftInput, leftLimits)
	right, rightErr := document.Execute(rightInput, rightLimits)
	leftFailure, err := retainedExecutionFailure(leftErr)
	if err != nil {
		return false, err
	}
	rightFailure, err := retainedExecutionFailure(rightErr)
	if err != nil {
		return false, err
	}
	if (leftFailure == nil) != (rightFailure == nil) {
		return true, nil
	}
	if leftFailure != nil && rightFailure != nil {
		return false, nil
	}
	return !document.publicOutputsEqual(left.Outputs, right.Outputs, outputs), nil
}

func retainedExecutionFailure(err error) (*ExecutionFailure, error) {
	if err == nil {
		return nil, nil
	}
	if code, ok := rejection.CodeOf(err); ok && code == rejection.ResourceLimit {
		return nil, err
	}
	var failure *ExecutionFailure
	if errors.As(err, &failure) {
		return failure, nil
	}
	return nil, err
}

func (document Document) publicInputsEqual(left, right ConcreteWorld, names []string) bool {
	for _, name := range names {
		tableID, exists := document.inputTables[name]
		if !exists || !document.publicTablesEqual(left, right, name, tableID) {
			return false
		}
	}
	return true
}

func (document Document) publicOutputsEqual(left, right ConcreteWorld, names []string) bool {
	for _, name := range names {
		tableID, exists := document.outputTables[name]
		if !exists || !document.publicTablesEqual(left, right, name, tableID) {
			return false
		}
	}
	return true
}

func (document Document) publicTablesEqual(left, right ConcreteWorld, name string, tableID protocol.Digest) bool {
	leftTable, leftExists := concreteTableByName(left, name)
	rightTable, rightExists := concreteTableByName(right, name)
	if !leftExists || !rightExists || len(leftTable.Rows) != len(rightTable.Rows) {
		return false
	}
	table, exists := document.tables[tableID]
	if !exists {
		return false
	}
	record, exists := document.records[table.recordType]
	if !exists {
		return false
	}
	for rowIndex := range leftTable.Rows {
		for name, field := range record.fields {
			if field.label != publicLabel {
				continue
			}
			leftValue, leftExists := concreteRecordField(leftTable.Rows[rowIndex], name)
			rightValue, rightExists := concreteRecordField(rightTable.Rows[rowIndex], name)
			if !leftExists || !rightExists || leftValue != rightValue {
				return false
			}
		}
	}
	return true
}

func (document Document) outputFieldOmitsRequiredOrigin(subject ObligationSubject, required []string) (bool, error) {
	nodeID, exists := document.outputNodes[subject.Interface]
	if !exists {
		return false, rejection.New(rejection.ConcreteCheckMismatch, "field-origin output interface is unavailable")
	}
	node, exists := document.nodes[nodeID]
	if !exists {
		return false, rejection.New(rejection.ConcreteCheckMismatch, "field-origin output node is unavailable")
	}
	var expression strictjson.Value
	found := false
	for _, projection := range node.expressions {
		if projection.fieldName == subject.Name {
			expression = projection.value
			found = true
			break
		}
	}
	if !found {
		return false, rejection.New(rejection.ConcreteCheckMismatch, "field-origin projection is unavailable")
	}
	dependencies := make(map[string]struct{})
	collectExpressionFields(expression, dependencies)
	for _, name := range required {
		if _, exists := dependencies[name]; exists {
			return false, nil
		}
	}
	return true, nil
}

func collectExpressionFields(value strictjson.Value, result map[string]struct{}) {
	if members, ok := value.Members(); ok {
		for _, member := range members {
			if member.Name == "field" {
				if name, ok := member.Value.Text(); ok {
					result[name] = struct{}{}
				}
			}
			collectExpressionFields(member.Value, result)
		}
		return
	}
	if items, ok := value.Items(); ok {
		for _, item := range items {
			collectExpressionFields(item, result)
		}
	}
}

func (document Document) targetNodeDroppedRows(id protocol.Digest, execution ProgramExecution) (bool, error) {
	node, exists := document.nodes[id]
	if !exists || node.kind != "filter" || len(node.predecessors) != 1 {
		return false, rejection.New(rejection.ConcreteCheckMismatch, "row-coverage target is not a retained filter node")
	}
	source, sourceExists := execution.nodes[node.predecessors[0]]
	output, outputExists := execution.nodes[id]
	if !sourceExists || !outputExists {
		return false, rejection.New(rejection.ConcreteCheckMismatch, "row-coverage target tables are unavailable")
	}
	return len(output.Rows) < len(source.Rows), nil
}

func (document Document) targetGroupCollapsedWitness(
	id protocol.Digest,
	world ConcreteWorld,
	execution ProgramExecution,
	requiredFields []string,
) (bool, error) {
	node, exists := document.nodes[id]
	if !exists || node.kind != "group" {
		return false, rejection.New(rejection.ConcreteCheckMismatch, "group-conservation target is not a retained group node")
	}
	output, exists := execution.nodes[id]
	if !exists {
		return false, rejection.New(rejection.ConcreteCheckMismatch, "group-conservation output table is unavailable")
	}
	for _, field := range requiredFields {
		inputValues := make(map[ConcreteValue]struct{})
		for _, table := range world.Tables {
			for _, row := range table.Rows {
				if value, exists := concreteRecordField(row, field); exists {
					inputValues[value] = struct{}{}
				}
			}
		}
		outputValues := make(map[ConcreteValue]struct{})
		for _, row := range output.Rows {
			if value, exists := concreteRecordField(row, field); exists {
				outputValues[value] = struct{}{}
			}
		}
		if len(inputValues) > len(outputValues) && len(inputValues) > 1 {
			return true, nil
		}
	}
	return false, nil
}

func (document Document) knownWitnessFields(names []string) bool {
	for _, name := range names {
		found := false
		for _, tableID := range document.inputTables {
			table := document.tables[tableID]
			if _, exists := document.records[table.recordType].fields[name]; exists {
				found = true
				break
			}
		}
		if !found {
			for _, tableID := range document.outputTables {
				table := document.tables[tableID]
				if _, exists := document.records[table.recordType].fields[name]; exists {
					found = true
					break
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (document Document) witnessKeysPresent(worlds []ConcreteWorld, keys []string) bool {
	for _, required := range keys {
		interfaceName, spelling, ok := strings.Cut(required, ":")
		if !ok || interfaceName == "" || spelling == "" {
			return false
		}
		tableID, exists := document.inputTables[interfaceName]
		if !exists {
			return false
		}
		definition := document.tables[tableID]
		if len(definition.primaryKey) != 1 {
			return false
		}
		found := false
		for _, world := range worlds {
			table, exists := concreteTableByName(world, interfaceName)
			if !exists {
				continue
			}
			for _, row := range table.Rows {
				value, exists := concreteRecordField(row, definition.primaryKey[0])
				if exists && concreteKeySpelling(value) == spelling {
					found = true
					break
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func concreteKeySpelling(value ConcreteValue) string {
	switch value.Kind {
	case "bool":
		if value.Bool {
			return "true"
		}
		return "false"
	case "int":
		return value.Integer
	case "text":
		return value.Text
	case "enum":
		return value.EnumMember
	default:
		return ""
	}
}

func concreteTableByName(world ConcreteWorld, name string) (ConcreteTable, bool) {
	for _, table := range world.Tables {
		if table.Name == name {
			return table, true
		}
	}
	return ConcreteTable{}, false
}

func namesFromValue(value strictjson.Value) ([]string, error) {
	items, err := array(value)
	if err != nil {
		return nil, err
	}
	result := make([]string, len(items))
	for index, item := range items {
		result[index], err = name(item)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
