package axiomir_test

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

func TestNodeTableRelationshipsAcceptLockedKinds(t *testing.T) {
	for _, kind := range []string{"filter", "map", "lookup_join", "group"} {
		t.Run(kind, func(t *testing.T) {
			config := defaultRelationshipConfig(kind)
			if _, err := axiomir.ParseStructure(relationshipFixture(t, config), fixtureLimits()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNodeTableRelationshipsRejectIllFormedRelations(t *testing.T) {
	tests := []struct {
		name   string
		kind   string
		mutate func(*relationshipConfig)
	}{
		{
			name: "filter record differs",
			kind: "filter",
			mutate: func(config *relationshipConfig) {
				config.outputRecord = recordFields(
					fieldJSON("key", "public", textTypeJSON),
					fieldJSON("other", "public", int09TypeJSON),
				)
			},
		},
		{
			name: "filter primary key differs",
			kind: "filter",
			mutate: func(config *relationshipConfig) {
				config.leftRecord = recordFields(
					fieldJSON("key", "public", textTypeJSON),
					fieldJSON("other", "public", textTypeJSON),
					fieldJSON("value", "public", int09TypeJSON),
				)
				config.outputRecord = config.leftRecord
				config.outputPrimaryKey = []string{"other"}
			},
		},
		{
			name: "filter capacity exceeds source",
			kind: "filter",
			mutate: func(config *relationshipConfig) {
				config.outputCapacity = "3"
			},
		},
		{
			name: "map projection misses output field",
			kind: "map",
			mutate: func(config *relationshipConfig) {
				config.fields = projectionFields(projectionJSON("renamed", "key", 0))
			},
		},
		{
			name: "map projection has extra field",
			kind: "map",
			mutate: func(config *relationshipConfig) {
				config.fields = projectionFields(
					projectionJSON("renamed", "key", 0),
					projectionJSON("value", "value", 0),
					`{"expression":{"op":"literal_bool","value":true},"name":"z"}`,
				)
			},
		},
		{
			name: "map projection type differs",
			kind: "map",
			mutate: func(config *relationshipConfig) {
				config.outputRecord = recordFields(
					fieldJSON("renamed", "public", textTypeJSON),
					fieldJSON("value", "public", textTypeJSON),
				)
			},
		},
		{
			name: "map capacity differs",
			kind: "map",
			mutate: func(config *relationshipConfig) {
				config.outputCapacity = "1"
			},
		},
		{
			name: "map primary key is derived",
			kind: "map",
			mutate: func(config *relationshipConfig) {
				config.fields = projectionFields(
					`{"expression":{"op":"literal_text","value":"constant"},"name":"renamed"}`,
					projectionJSON("value", "value", 0),
				)
			},
		},
		{
			name: "map primary key reads non-key source field",
			kind: "map",
			mutate: func(config *relationshipConfig) {
				config.leftRecord = recordFields(
					fieldJSON("key", "public", textTypeJSON),
					fieldJSON("other", "public", textTypeJSON),
					fieldJSON("value", "public", int09TypeJSON),
				)
				config.fields = projectionFields(
					projectionJSON("renamed", "other", 0),
					projectionJSON("value", "value", 0),
				)
			},
		},
		{
			name: "map primary key arity differs",
			kind: "map",
			mutate: func(config *relationshipConfig) {
				config.leftRecord = recordFields(
					fieldJSON("key", "public", textTypeJSON),
					fieldJSON("other", "public", textTypeJSON),
					fieldJSON("value", "public", int09TypeJSON),
				)
				config.leftPrimaryKey = []string{"key", "other"}
			},
		},
		{
			name: "lookup_join left pair field is missing",
			kind: "lookup_join",
			mutate: func(config *relationshipConfig) {
				config.pairs = `[{"left":"missing","right":"link"}]`
			},
		},
		{
			name: "lookup_join right pair field is missing",
			kind: "lookup_join",
			mutate: func(config *relationshipConfig) {
				config.pairs = `[{"left":"link","right":"missing"}]`
			},
		},
		{
			name: "lookup_join pair types differ",
			kind: "lookup_join",
			mutate: func(config *relationshipConfig) {
				config.rightRecord = recordFields(
					fieldJSON("link", "public", int09TypeJSON),
					fieldJSON("right_value", "public", int09TypeJSON),
				)
			},
		},
		{
			name: "lookup_join capacity differs",
			kind: "lookup_join",
			mutate: func(config *relationshipConfig) {
				config.outputCapacity = "1"
			},
		},
		{
			name: "lookup_join key reads right row",
			kind: "lookup_join",
			mutate: func(config *relationshipConfig) {
				config.rightRecord = recordFields(
					fieldJSON("key", "public", textTypeJSON),
					fieldJSON("link", "public", textTypeJSON),
					fieldJSON("right_value", "public", int09TypeJSON),
				)
				config.fields = projectionFields(
					projectionJSON("key", "key", 1),
					projectionJSON("right_value", "right_value", 1),
				)
			},
		},
		{
			name: "group capacity exceeds source",
			kind: "group",
			mutate: func(config *relationshipConfig) {
				config.outputCapacity = "3"
			},
		},
		{
			name: "group source key is missing",
			kind: "group",
			mutate: func(config *relationshipConfig) {
				config.keys = `[{"name":"group","source_field":"missing"}]`
			},
		},
		{
			name: "group source key is sensitive",
			kind: "group",
			mutate: func(config *relationshipConfig) {
				config.leftRecord = recordFields(
					fieldJSON("group", "sensitive", textTypeJSON),
					fieldJSON("key", "public", textTypeJSON),
					fieldJSON("value", "public", int09TypeJSON),
				)
			},
		},
		{
			name: "group output primary key differs",
			kind: "group",
			mutate: func(config *relationshipConfig) {
				config.outputPrimaryKey = []string{"count"}
			},
		},
		{
			name: "group key order differs from output primary key",
			kind: "group",
			mutate: func(config *relationshipConfig) {
				config.leftRecord = recordFields(
					fieldJSON("group", "public", textTypeJSON),
					fieldJSON("key", "public", textTypeJSON),
					fieldJSON("other", "public", textTypeJSON),
					fieldJSON("value", "public", int09TypeJSON),
				)
				config.outputRecord = recordFields(
					fieldJSON("count", "public", int02TypeJSON),
					fieldJSON("group", "public", textTypeJSON),
					fieldJSON("other", "public", textTypeJSON),
					fieldJSON("total", "public", int018TypeJSON),
				)
				config.outputPrimaryKey = []string{"other", "group"}
				config.keys = `[{"name":"group","source_field":"group"},{"name":"other","source_field":"other"}]`
			},
		},
		{
			name: "group output key type differs",
			kind: "group",
			mutate: func(config *relationshipConfig) {
				config.outputRecord = recordFields(
					fieldJSON("count", "public", int02TypeJSON),
					fieldJSON("group", "public", boolTypeJSON),
					fieldJSON("total", "public", int018TypeJSON),
				)
			},
		},
		{
			name: "group count bounds differ",
			kind: "group",
			mutate: func(config *relationshipConfig) {
				config.outputRecord = recordFields(
					fieldJSON("count", "public", `{"kind":"int","lower":"0","upper":"3"}`),
					fieldJSON("group", "public", textTypeJSON),
					fieldJSON("total", "public", int018TypeJSON),
				)
			},
		},
		{
			name: "group sum source field is missing",
			kind: "group",
			mutate: func(config *relationshipConfig) {
				config.aggregates = `[{"kind":"count","name":"count"},{"field":"missing","kind":"sum","name":"total"}]`
			},
		},
		{
			name: "group sum source is not Int",
			kind: "group",
			mutate: func(config *relationshipConfig) {
				config.leftRecord = recordFields(
					fieldJSON("group", "public", textTypeJSON),
					fieldJSON("key", "public", textTypeJSON),
					fieldJSON("value", "public", textTypeJSON),
				)
			},
		},
		{
			name: "group sum output is not Int",
			kind: "group",
			mutate: func(config *relationshipConfig) {
				config.outputRecord = recordFields(
					fieldJSON("count", "public", int02TypeJSON),
					fieldJSON("group", "public", textTypeJSON),
					fieldJSON("total", "public", textTypeJSON),
				)
			},
		},
		{
			name: "group declarations miss output field",
			kind: "group",
			mutate: func(config *relationshipConfig) {
				config.aggregates = `[{"kind":"count","name":"count"}]`
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := defaultRelationshipConfig(test.kind)
			test.mutate(&config)
			_, err := axiomir.ParseStructure(relationshipFixture(t, config), fixtureLimits())
			assertCode(t, err, rejection.InvalidJSON)
		})
	}
}

type relationshipConfig struct {
	kind             string
	leftRecord       string
	leftCapacity     string
	leftPrimaryKey   []string
	rightRecord      string
	rightCapacity    string
	rightPrimaryKey  []string
	outputRecord     string
	outputCapacity   string
	outputPrimaryKey []string
	predicate        string
	fields           string
	pairs            string
	keys             string
	aggregates       string
}

const (
	boolTypeJSON   = `{"kind":"bool"}`
	textTypeJSON   = `{"kind":"text"}`
	int09TypeJSON  = `{"kind":"int","lower":"0","upper":"9"}`
	int02TypeJSON  = `{"kind":"int","lower":"0","upper":"2"}`
	int018TypeJSON = `{"kind":"int","lower":"0","upper":"18"}`
)

func defaultRelationshipConfig(kind string) relationshipConfig {
	config := relationshipConfig{
		kind:             kind,
		leftCapacity:     "2",
		leftPrimaryKey:   []string{"key"},
		outputCapacity:   "2",
		outputPrimaryKey: []string{"key"},
		predicate:        `{"op":"literal_bool","value":true}`,
	}
	switch kind {
	case "filter":
		config.leftRecord = recordFields(
			fieldJSON("key", "public", textTypeJSON),
			fieldJSON("value", "public", int09TypeJSON),
		)
		config.outputRecord = config.leftRecord
		config.outputCapacity = "1"
	case "map":
		config.leftRecord = recordFields(
			fieldJSON("key", "public", textTypeJSON),
			fieldJSON("value", "public", int09TypeJSON),
		)
		config.outputRecord = recordFields(
			fieldJSON("renamed", "public", textTypeJSON),
			fieldJSON("value", "public", int09TypeJSON),
		)
		config.outputPrimaryKey = []string{"renamed"}
		config.fields = projectionFields(
			projectionJSON("renamed", "key", 0),
			projectionJSON("value", "value", 0),
		)
	case "lookup_join":
		config.leftRecord = recordFields(
			fieldJSON("key", "public", textTypeJSON),
			fieldJSON("link", "public", textTypeJSON),
		)
		config.rightRecord = recordFields(
			fieldJSON("link", "public", textTypeJSON),
			fieldJSON("right_value", "public", int09TypeJSON),
		)
		config.rightCapacity = "2"
		config.rightPrimaryKey = []string{"link"}
		config.outputRecord = recordFields(
			fieldJSON("key", "public", textTypeJSON),
			fieldJSON("right_value", "public", int09TypeJSON),
		)
		config.fields = projectionFields(
			projectionJSON("key", "key", 0),
			projectionJSON("right_value", "right_value", 1),
		)
		config.pairs = `[{"left":"link","right":"link"}]`
	case "group":
		config.leftRecord = recordFields(
			fieldJSON("group", "public", textTypeJSON),
			fieldJSON("key", "public", textTypeJSON),
			fieldJSON("value", "public", int09TypeJSON),
		)
		config.outputRecord = recordFields(
			fieldJSON("count", "public", int02TypeJSON),
			fieldJSON("group", "public", textTypeJSON),
			fieldJSON("total", "public", int018TypeJSON),
		)
		config.outputPrimaryKey = []string{"group"}
		config.keys = `[{"name":"group","source_field":"group"}]`
		config.aggregates = `[{"kind":"count","name":"count"},{"field":"value","kind":"sum","name":"total"}]`
	default:
		panic("unknown relationship fixture kind")
	}
	return config
}

func relationshipFixture(t *testing.T, config relationshipConfig) []byte {
	t.Helper()
	var records []identifiedFixtureEntry
	leftRecordEntry, leftRecordID := declarationEntry("axiom-ir-v0.1:record-type", config.leftRecord)
	appendUniqueFixtureEntry(&records, identifiedFixtureEntry{id: leftRecordID, entry: leftRecordEntry})
	outputRecordEntry, outputRecordID := declarationEntry("axiom-ir-v0.1:record-type", config.outputRecord)
	appendUniqueFixtureEntry(&records, identifiedFixtureEntry{id: outputRecordID, entry: outputRecordEntry})

	var tables []identifiedFixtureEntry
	leftTableDefinition := tableJSON(config.leftCapacity, config.leftPrimaryKey, leftRecordID)
	leftTableEntry, leftTableID := declarationEntry("axiom-ir-v0.1:table-type", leftTableDefinition)
	appendUniqueFixtureEntry(&tables, identifiedFixtureEntry{id: leftTableID, entry: leftTableEntry})
	outputTableDefinition := tableJSON(config.outputCapacity, config.outputPrimaryKey, outputRecordID)
	outputTableEntry, outputTableID := declarationEntry("axiom-ir-v0.1:table-type", outputTableDefinition)
	appendUniqueFixtureEntry(&tables, identifiedFixtureEntry{id: outputTableID, entry: outputTableEntry})

	leftNodeDefinition := fmt.Sprintf(`{"kind":"input","port":"left","table_type":"%s"}`, leftTableID)
	leftNodeEntry, leftNodeID := declarationEntry("axiom-ir-v0.1:node", leftNodeDefinition)
	nodes := []identifiedFixtureEntry{{id: leftNodeID, entry: leftNodeEntry}}

	var rightNodeID string
	if config.kind == "lookup_join" {
		rightRecordEntry, rightRecordID := declarationEntry("axiom-ir-v0.1:record-type", config.rightRecord)
		appendUniqueFixtureEntry(&records, identifiedFixtureEntry{id: rightRecordID, entry: rightRecordEntry})
		rightTableDefinition := tableJSON(config.rightCapacity, config.rightPrimaryKey, rightRecordID)
		rightTableEntry, rightTableID := declarationEntry("axiom-ir-v0.1:table-type", rightTableDefinition)
		appendUniqueFixtureEntry(&tables, identifiedFixtureEntry{id: rightTableID, entry: rightTableEntry})
		rightNodeDefinition := fmt.Sprintf(`{"kind":"input","port":"right","table_type":"%s"}`, rightTableID)
		rightNodeEntry, id := declarationEntry("axiom-ir-v0.1:node", rightNodeDefinition)
		rightNodeID = id
		nodes = append(nodes, identifiedFixtureEntry{id: rightNodeID, entry: rightNodeEntry})
	}

	var transformDefinition string
	switch config.kind {
	case "filter":
		transformDefinition = fmt.Sprintf(
			`{"kind":"filter","predicate":%s,"source":"%s","table_type":"%s"}`,
			config.predicate,
			leftNodeID,
			outputTableID,
		)
	case "map":
		transformDefinition = fmt.Sprintf(
			`{"fields":%s,"kind":"map","source":"%s","table_type":"%s"}`,
			config.fields,
			leftNodeID,
			outputTableID,
		)
	case "lookup_join":
		transformDefinition = fmt.Sprintf(
			`{"fields":%s,"kind":"lookup_join","left":"%s","pairs":%s,"right":"%s","table_type":"%s"}`,
			config.fields,
			leftNodeID,
			config.pairs,
			rightNodeID,
			outputTableID,
		)
	case "group":
		transformDefinition = fmt.Sprintf(
			`{"aggregates":%s,"keys":%s,"kind":"group","source":"%s","table_type":"%s"}`,
			config.aggregates,
			config.keys,
			leftNodeID,
			outputTableID,
		)
	default:
		t.Fatalf("unknown relationship fixture kind: %q", config.kind)
	}
	transformEntry, transformID := declarationEntry("axiom-ir-v0.1:node", transformDefinition)
	nodes = append(nodes, identifiedFixtureEntry{id: transformID, entry: transformEntry})

	for _, entries := range [][]identifiedFixtureEntry{records, tables, nodes} {
		sort.Slice(entries, func(i, j int) bool { return entries[i].id < entries[j].id })
	}
	return []byte(fmt.Sprintf(
		`{"contracts":[],"digest_algorithm":"sha-256","effects":[],"enum_types":[],"format":"axiom-ir","ir_version":"0.1","nodes":[%s],"outputs":[{"name":"out","node":"%s"}],"record_types":[%s],"semantics":{"name":"keyed-finite-table-semantics","sha256":"6b18d65eefa439956db8eebe1f4ce90e08b4def4abf7c718c2605e7528598d0d"},"table_types":[%s]}`,
		joinFixtureEntries(nodes),
		transformID,
		joinFixtureEntries(records),
		joinFixtureEntries(tables),
	))
}

func appendUniqueFixtureEntry(entries *[]identifiedFixtureEntry, candidate identifiedFixtureEntry) {
	for _, entry := range *entries {
		if entry.id == candidate.id {
			return
		}
	}
	*entries = append(*entries, candidate)
}

func tableJSON(capacity string, primaryKey []string, recordID string) string {
	quoted := make([]string, 0, len(primaryKey))
	for _, fieldName := range primaryKey {
		quoted = append(quoted, strconv.Quote(fieldName))
	}
	return fmt.Sprintf(
		`{"capacity":"%s","primary_key":[%s],"record_type":"%s"}`,
		capacity,
		strings.Join(quoted, ","),
		recordID,
	)
}

func recordFields(fields ...string) string {
	return fmt.Sprintf(`{"fields":[%s]}`, strings.Join(fields, ","))
}

func fieldJSON(fieldName, label, fieldType string) string {
	return fmt.Sprintf(`{"label":"%s","name":"%s","type":%s}`, label, fieldName, fieldType)
}

func projectionFields(fields ...string) string {
	return "[" + strings.Join(fields, ",") + "]"
}

func projectionJSON(outputName, sourceName string, boundIndex uint64) string {
	return fmt.Sprintf(
		`{"expression":{"field":"%s","op":"field","record":{"index":"%d","op":"bound"}},"name":"%s"}`,
		sourceName,
		boundIndex,
		outputName,
	)
}
