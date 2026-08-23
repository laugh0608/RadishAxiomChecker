package axiomir_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

func TestDeclarationIndexAcceptsLockedKeyCompatibleKinds(t *testing.T) {
	for _, kind := range []string{"bool", "int", "text", "enum"} {
		t.Run(kind, func(t *testing.T) {
			data := declarationFixture(t, declarationConfig{fieldKind: kind})
			document, err := axiomir.ParseStructure(data, fixtureLimits())
			if err != nil {
				t.Fatal(err)
			}
			if document.Counts.EnumTypes != boolInt(kind == "enum") ||
				document.Counts.RecordTypes != 1 || document.Counts.TableTypes != 1 {
				t.Fatalf("unexpected declaration counts: %+v", document.Counts)
			}
		})
	}
}

func TestDeclarationIndexRejectsIllFormedTypesAndKeys(t *testing.T) {
	tests := []struct {
		name   string
		config declarationConfig
		code   rejection.Code
	}{
		{
			name:   "missing primary key field",
			config: declarationConfig{fieldKind: "text", fieldName: "other"},
			code:   rejection.InvalidJSON,
		},
		{
			name:   "sensitive primary key field",
			config: declarationConfig{fieldKind: "text", label: "sensitive"},
			code:   rejection.InvalidJSON,
		},
		{
			name:   "duplicate primary key field",
			config: declarationConfig{fieldKind: "text", primaryKey: []string{"key", "key"}},
			code:   rejection.InvalidJSON,
		},
		{
			name:   "empty primary key",
			config: declarationConfig{fieldKind: "text", primaryKey: []string{}},
			code:   rejection.InvalidJSON,
		},
		{
			name:   "dangling enum type",
			config: declarationConfig{fieldKind: "dangling-enum"},
			code:   rejection.InvalidJSON,
		},
		{
			name:   "empty enum members",
			config: declarationConfig{fieldKind: "enum", enumDefinition: `{"members":[],"name":"Key"}`},
			code:   rejection.InvalidJSON,
		},
		{
			name:   "duplicate enum members",
			config: declarationConfig{fieldKind: "enum", enumDefinition: `{"members":["only","only"],"name":"Key"}`},
			code:   rejection.InvalidJSON,
		},
		{
			name:   "unsupported option key type",
			config: declarationConfig{fieldKind: "option"},
			code:   rejection.UnknownTag,
		},
		{
			name:   "dangling record type",
			config: declarationConfig{fieldKind: "text", danglingRecord: true},
			code:   rejection.InvalidJSON,
		},
		{
			name:   "unknown field label",
			config: declarationConfig{fieldKind: "text", label: "internal"},
			code:   rejection.UnknownTag,
		},
		{
			name:   "inverted integer bounds",
			config: declarationConfig{fieldKind: "inverted-int"},
			code:   rejection.InvalidJSON,
		},
		{
			name: "duplicate record field",
			config: declarationConfig{
				fieldKind:        "text",
				recordDefinition: `{"fields":[{"label":"public","name":"key","type":{"kind":"text"}},{"label":"public","name":"key","type":{"kind":"text"}}]}`,
			},
			code: rejection.NoncanonicalOrder,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := axiomir.ParseStructure(declarationFixture(t, test.config), fixtureLimits())
			assertCode(t, err, test.code)
		})
	}
}

type declarationConfig struct {
	danglingRecord   bool
	enumDefinition   string
	fieldKind        string
	fieldName        string
	label            string
	primaryKey       []string
	recordDefinition string
}

func declarationFixture(t *testing.T, config declarationConfig) []byte {
	t.Helper()
	if config.fieldName == "" {
		config.fieldName = "key"
	}
	if config.label == "" {
		config.label = "public"
	}
	if config.primaryKey == nil {
		config.primaryKey = []string{"key"}
	}

	enumEntries := ""
	var fieldType string
	switch config.fieldKind {
	case "bool":
		fieldType = `{"kind":"bool"}`
	case "int":
		fieldType = `{"kind":"int","lower":"0","upper":"9"}`
	case "inverted-int":
		fieldType = `{"kind":"int","lower":"9","upper":"0"}`
	case "text":
		fieldType = `{"kind":"text"}`
	case "enum":
		enumDefinition := config.enumDefinition
		if enumDefinition == "" {
			enumDefinition = `{"members":["only"],"name":"Key"}`
		}
		enumEntry, enumID := declarationEntry("axiom-ir-v0.1:enum-type", enumDefinition)
		enumEntries = enumEntry
		fieldType = fmt.Sprintf(`{"enum_type":"%s","kind":"enum"}`, enumID)
	case "dangling-enum":
		fieldType = `{"enum_type":"sha256:0000000000000000000000000000000000000000000000000000000000000000","kind":"enum"}`
	case "option":
		fieldType = `{"inner":{"kind":"text"},"kind":"option"}`
	default:
		t.Fatalf("unknown test field kind: %q", config.fieldKind)
	}

	recordDefinition := config.recordDefinition
	if recordDefinition == "" {
		recordDefinition = fmt.Sprintf(
			`{"fields":[{"label":"%s","name":"%s","type":%s}]}`,
			config.label,
			config.fieldName,
			fieldType,
		)
	}
	recordEntry, recordID := declarationEntry("axiom-ir-v0.1:record-type", recordDefinition)
	if config.danglingRecord {
		recordID = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	}
	quotedKeys := make([]string, 0, len(config.primaryKey))
	for _, key := range config.primaryKey {
		quotedKeys = append(quotedKeys, fmt.Sprintf(`"%s"`, key))
	}
	tableDefinition := fmt.Sprintf(
		`{"capacity":"1","primary_key":[%s],"record_type":"%s"}`,
		strings.Join(quotedKeys, ","),
		recordID,
	)
	tableEntry, tableID := declarationEntry("axiom-ir-v0.1:table-type", tableDefinition)
	nodeDefinition := fmt.Sprintf(`{"kind":"input","port":"in","table_type":"%s"}`, tableID)
	nodeEntry, nodeID := declarationEntry("axiom-ir-v0.1:node", nodeDefinition)

	return []byte(fmt.Sprintf(
		`{"contracts":[],"digest_algorithm":"sha-256","effects":[],"enum_types":[%s],"format":"axiom-ir","ir_version":"0.1","nodes":[%s],"outputs":[{"name":"out","node":"%s"}],"record_types":[%s],"semantics":{"name":"keyed-finite-table-semantics","sha256":"6b18d65eefa439956db8eebe1f4ce90e08b4def4abf7c718c2605e7528598d0d"},"table_types":[%s]}`,
		enumEntries,
		nodeEntry,
		nodeID,
		recordEntry,
		tableEntry,
	))
}

func declarationEntry(domain, definition string) (string, string) {
	hash := sha256.New()
	hash.Write([]byte(domain))
	hash.Write([]byte{0})
	hash.Write([]byte(definition))
	id := "sha256:" + hex.EncodeToString(hash.Sum(nil))
	return fmt.Sprintf(`{"definition":%s,"id":"%s"}`, definition, id), id
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
