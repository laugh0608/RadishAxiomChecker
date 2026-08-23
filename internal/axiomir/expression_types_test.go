package axiomir_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

func TestExpressionTypesAcceptLockedIntegerAndBooleanComposition(t *testing.T) {
	intType := `{"kind":"int","lower":"0","upper":"9"}`
	literalOne := fmt.Sprintf(`{"op":"literal_int","type":%s,"value":"1"}`, intType)
	literalTwo := fmt.Sprintf(`{"op":"literal_int","type":%s,"value":"2"}`, intType)
	literalThree := fmt.Sprintf(`{"op":"literal_int","type":%s,"value":"3"}`, intType)
	addition := fmt.Sprintf(`{"op":"int_add","result_type":%s,"values":[%s,%s]}`, intType, literalOne, literalTwo)
	equality := fmt.Sprintf(`{"left":%s,"op":"eq","right":%s}`, addition, literalThree)
	predicate := fmt.Sprintf(
		`{"op":"and","values":[%s,{"op":"not","value":{"op":"literal_bool","value":false}}]}`,
		equality,
	)

	if _, err := axiomir.ParseStructure(expressionFixture(t, predicate, ""), fixtureLimits()); err != nil {
		t.Fatal(err)
	}
}

func TestExpressionTypesRejectIllTypedLockedOperations(t *testing.T) {
	int09 := `{"kind":"int","lower":"0","upper":"9"}`
	int010 := `{"kind":"int","lower":"0","upper":"10"}`
	literalInt09 := fmt.Sprintf(`{"op":"literal_int","type":%s,"value":"1"}`, int09)
	literalInt010 := fmt.Sprintf(`{"op":"literal_int","type":%s,"value":"1"}`, int010)
	literalText := `{"op":"literal_text","value":"x"}`
	literalBool := `{"op":"literal_bool","value":true}`
	inputTable := `{"kind":"input","name":"in"}`
	validLookup := fmt.Sprintf(`{"keys":[%s],"op":"lookup","table":%s}`, literalText, inputTable)

	tests := []struct {
		name      string
		predicate string
		formula   string
	}{
		{
			name:      "filter predicate is not Bool",
			predicate: literalText,
		},
		{
			name:      "field name is absent",
			predicate: `{"field":"missing","op":"field","record":{"index":"0","op":"bound"}}`,
		},
		{
			name:      "field operand is not a record",
			predicate: fmt.Sprintf(`{"field":"key","op":"field","record":%s}`, literalText),
		},
		{
			name:      "bound value has the wrong operand type",
			predicate: `{"op":"not","value":{"index":"0","op":"bound"}}`,
		},
		{
			name:      "and operand is not Bool",
			predicate: fmt.Sprintf(`{"op":"and","values":[%s,%s]}`, literalBool, literalText),
		},
		{
			name:      "eq operands differ",
			predicate: fmt.Sprintf(`{"left":%s,"op":"eq","right":%s}`, literalBool, literalText),
		},
		{
			name:      "record equality is not defined",
			predicate: `{"left":{"index":"0","op":"bound"},"op":"eq","right":{"index":"0","op":"bound"}}`,
		},
		{
			name:      "le Int types differ",
			predicate: fmt.Sprintf(`{"left":%s,"op":"le","right":%s}`, literalInt010, literalInt09),
		},
		{
			name: "int_add operand is not Int",
			predicate: fmt.Sprintf(
				`{"left":{"op":"int_add","result_type":%s,"values":[%s,%s]},"op":"eq","right":%s}`,
				int09,
				literalInt09,
				literalText,
				literalInt09,
			),
		},
		{
			name: "int_sub operand types differ",
			predicate: fmt.Sprintf(
				`{"left":{"left":%s,"op":"int_sub","result_type":%s,"right":%s},"op":"eq","right":%s}`,
				literalInt09,
				int09,
				literalInt010,
				literalInt09,
			),
		},
		{
			name: "if condition is not Bool",
			predicate: fmt.Sprintf(
				`{"condition":%s,"else":%s,"op":"if","result_type":{"kind":"bool"},"then":%s}`,
				literalText,
				literalBool,
				literalBool,
			),
		},
		{
			name: "if branch differs from result type",
			predicate: fmt.Sprintf(
				`{"condition":%s,"else":%s,"op":"if","result_type":{"kind":"bool"},"then":%s}`,
				literalBool,
				literalText,
				literalBool,
			),
		},
		{
			name:      "formula is not Bool",
			predicate: literalBool,
			formula:   literalText,
		},
		{
			name:      "match_option some branch has wrong type",
			predicate: literalBool,
			formula: fmt.Sprintf(
				`{"none":%s,"op":"match_option","result_type":{"kind":"bool"},"some":{"index":"0","op":"bound"},"subject":%s}`,
				literalBool,
				validLookup,
			),
		},
		{
			name:      "match_option subject is not optional",
			predicate: literalBool,
			formula: fmt.Sprintf(
				`{"none":%s,"op":"match_option","result_type":{"kind":"bool"},"some":%s,"subject":%s}`,
				literalBool,
				literalBool,
				literalText,
			),
		},
		{
			name:      "lookup key arity differs",
			predicate: literalBool,
			formula: fmt.Sprintf(
				`{"keys":[%s,%s],"op":"lookup","table":%s}`,
				literalText,
				literalText,
				inputTable,
			),
		},
		{
			name:      "lookup key type differs",
			predicate: literalBool,
			formula:   fmt.Sprintf(`{"keys":[%s],"op":"lookup","table":%s}`, literalBool, inputTable),
		},
		{
			name:      "forall_rows body is not Bool",
			predicate: literalBool,
			formula:   fmt.Sprintf(`{"body":%s,"op":"forall_rows","table":%s}`, literalText, inputTable),
		},
		{
			name:      "count_where predicate is not Bool",
			predicate: literalBool,
			formula: fmt.Sprintf(
				`{"op":"count_where","predicate":%s,"result_type":%s,"table":%s}`,
				literalText,
				int09,
				inputTable,
			),
		},
		{
			name:      "sum_where predicate is not Bool",
			predicate: literalBool,
			formula: fmt.Sprintf(
				`{"op":"sum_where","predicate":%s,"result_type":%s,"table":%s,"value":%s}`,
				literalText,
				int09,
				inputTable,
				literalInt09,
			),
		},
		{
			name:      "sum_where value is not Int",
			predicate: literalBool,
			formula: fmt.Sprintf(
				`{"op":"sum_where","predicate":%s,"result_type":%s,"table":%s,"value":%s}`,
				literalBool,
				int09,
				inputTable,
				literalText,
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := axiomir.ParseStructure(expressionFixture(t, test.predicate, test.formula), fixtureLimits())
			assertCode(t, err, rejection.InvalidJSON)
		})
	}
}

func TestExpressionTypesRejectTableOperationsInNodeScope(t *testing.T) {
	predicate := `{"body":{"op":"literal_bool","value":true},"op":"forall_rows","table":{"kind":"input","name":"in"}}`
	_, err := axiomir.ParseStructure(expressionFixture(t, predicate, ""), fixtureLimits())
	assertCode(t, err, rejection.UnknownTag)
}

func expressionFixture(t *testing.T, predicate, formula string) []byte {
	t.Helper()
	recordDefinition := `{"fields":[{"label":"public","name":"key","type":{"kind":"text"}},{"label":"public","name":"value","type":{"kind":"int","lower":"0","upper":"9"}}]}`
	recordEntry, recordID := declarationEntry("axiom-ir-v0.1:record-type", recordDefinition)
	tableDefinition := fmt.Sprintf(`{"capacity":"2","primary_key":["key"],"record_type":"%s"}`, recordID)
	tableEntry, tableID := declarationEntry("axiom-ir-v0.1:table-type", tableDefinition)

	inputDefinition := fmt.Sprintf(`{"kind":"input","port":"in","table_type":"%s"}`, tableID)
	inputEntry, inputID := declarationEntry("axiom-ir-v0.1:node", inputDefinition)
	filterDefinition := fmt.Sprintf(
		`{"kind":"filter","predicate":%s,"source":"%s","table_type":"%s"}`,
		predicate,
		inputID,
		tableID,
	)
	filterEntry, filterID := declarationEntry("axiom-ir-v0.1:node", filterDefinition)
	nodes := []identifiedFixtureEntry{
		{id: inputID, entry: inputEntry},
		{id: filterID, entry: filterEntry},
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].id < nodes[j].id })

	contracts := ""
	if formula != "" {
		contractDefinition := fmt.Sprintf(`{"expression":%s,"kind":"formula","role":"guarantee"}`, formula)
		contractEntry, _ := declarationEntry("axiom-ir-v0.1:contract", contractDefinition)
		contracts = contractEntry
	}

	return []byte(fmt.Sprintf(
		`{"contracts":[%s],"digest_algorithm":"sha-256","effects":[],"enum_types":[],"format":"axiom-ir","ir_version":"0.1","nodes":[%s],"outputs":[{"name":"out","node":"%s"}],"record_types":[%s],"semantics":{"name":"keyed-finite-table-semantics","sha256":"6b18d65eefa439956db8eebe1f4ce90e08b4def4abf7c718c2605e7528598d0d"},"table_types":[%s]}`,
		contracts,
		joinFixtureEntries(nodes),
		filterID,
		recordEntry,
		tableEntry,
	))
}

type identifiedFixtureEntry struct {
	id    string
	entry string
}

func joinFixtureEntries(entries []identifiedFixtureEntry) string {
	values := make([]string, 0, len(entries))
	for _, entry := range entries {
		values = append(values, entry.entry)
	}
	return strings.Join(values, ",")
}
