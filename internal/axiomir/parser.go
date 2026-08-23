package axiomir

import (
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

const semanticsSHA256 = "6b18d65eefa439956db8eebe1f4ce90e08b4def4abf7c718c2605e7528598d0d"

type enumDefinition struct {
	members map[string]struct{}
}

type valueKind uint8

const (
	invalidValue valueKind = iota
	boolValue
	intValue
	textValue
	enumValue
	recordValue
	optionRecordValue
)

type valueType struct {
	kind       valueKind
	lower      string
	upper      string
	enumType   protocol.Digest
	recordType protocol.Digest
}

func (value valueType) equal(other valueType) bool {
	return value.kind == other.kind &&
		value.lower == other.lower &&
		value.upper == other.upper &&
		value.enumType == other.enumType &&
		value.recordType == other.recordType
}

func (value valueType) keyCompatible() bool {
	switch value.kind {
	case boolValue, intValue, textValue, enumValue:
		return true
	default:
		return false
	}
}

func (value valueType) equalityCompatible() bool {
	switch value.kind {
	case boolValue, intValue, textValue, enumValue:
		return true
	default:
		return false
	}
}

type fieldLabel uint8

const (
	invalidLabel fieldLabel = iota
	publicLabel
	sensitiveLabel
)

type fieldDefinition struct {
	label    fieldLabel
	typeInfo valueType
}

type recordDefinition struct {
	fields map[string]fieldDefinition
}

type tableDefinition struct {
	capacity   string
	primaryKey []string
	recordType protocol.Digest
}

type nodeDefinition struct {
	kind         string
	predecessors []protocol.Digest
	tableType    protocol.Digest
	expressions  []nodeExpressionCheck
}

type nodeExpressionCheck struct {
	value       strictjson.Value
	requireBool bool
}

type parser struct {
	enums       map[protocol.Digest]enumDefinition
	records     map[protocol.Digest]recordDefinition
	tables      map[protocol.Digest]tableDefinition
	nodes       map[protocol.Digest]nodeDefinition
	nodeOrder   []protocol.Digest
	inputPorts  map[string]protocol.Digest
	outputNames map[string]protocol.Digest
}

// ParseStructure parses canonical Axiom IR v0.1 bytes and verifies the closed
// structure profile documented by this package, all content-addressed entry
// IDs and references, checks declaration and primary-key well-formedness,
// expression typing, node DAG shape, and recomputes document-domain identity.
// It does not verify complete node input/output table relationships or rebuild
// obligations.
func ParseStructure(data []byte, limits strictjson.Limits) (Document, error) {
	value, err := strictjson.ParseCanonical(data, limits)
	if err != nil {
		return Document{}, err
	}
	root, err := object(value,
		"contracts", "digest_algorithm", "effects", "enum_types", "format",
		"ir_version", "nodes", "outputs", "record_types", "semantics", "table_types",
	)
	if err != nil {
		return Document{}, err
	}
	if err := requireText(root["format"], "axiom-ir", rejection.UnknownTag, "unknown Axiom IR format"); err != nil {
		return Document{}, err
	}
	if err := requireText(root["ir_version"], "0.1", rejection.UnsupportedVersion, "unsupported Axiom IR version"); err != nil {
		return Document{}, err
	}
	if err := requireText(root["digest_algorithm"], "sha-256", rejection.UnknownTag, "unsupported Axiom IR digest algorithm"); err != nil {
		return Document{}, err
	}
	if err := parseSemantics(root["semantics"]); err != nil {
		return Document{}, err
	}
	effects, err := array(root["effects"])
	if err != nil {
		return Document{}, err
	}
	if len(effects) != 0 {
		return Document{}, rejection.New(rejection.UnknownTag, "Axiom IR v0.1 effect set must be empty")
	}

	p := parser{
		enums:       make(map[protocol.Digest]enumDefinition),
		records:     make(map[protocol.Digest]recordDefinition),
		tables:      make(map[protocol.Digest]tableDefinition),
		nodes:       make(map[protocol.Digest]nodeDefinition),
		nodeOrder:   make([]protocol.Digest, 0),
		inputPorts:  make(map[string]protocol.Digest),
		outputNames: make(map[string]protocol.Digest),
	}
	enumCount, err := p.parseEntries(root["enum_types"], domainEnumType, p.parseEnumDefinition)
	if err != nil {
		return Document{}, err
	}
	recordCount, err := p.parseEntries(root["record_types"], domainRecordType, p.parseRecordDefinition)
	if err != nil {
		return Document{}, err
	}
	tableCount, err := p.parseEntries(root["table_types"], domainTableType, p.parseTableDefinition)
	if err != nil {
		return Document{}, err
	}
	nodeCount, err := p.parseEntries(root["nodes"], domainNode, p.parseNodeDefinition)
	if err != nil {
		return Document{}, err
	}
	outputCount, err := p.parseOutputs(root["outputs"])
	if err != nil {
		return Document{}, err
	}
	if err := p.validateNodeGraph(); err != nil {
		return Document{}, err
	}
	if err := p.validateNodeExpressionTypes(); err != nil {
		return Document{}, err
	}
	contractCount, err := p.parseEntries(root["contracts"], domainContract, p.parseContractDefinition)
	if err != nil {
		return Document{}, err
	}

	return Document{
		ContentDigest: contentDigest(data),
		DomainDigest:  domainDigest(domainDocument, data),
		Counts: Counts{
			Contracts:   contractCount,
			EnumTypes:   enumCount,
			Nodes:       nodeCount,
			Outputs:     outputCount,
			RecordTypes: recordCount,
			TableTypes:  tableCount,
		},
	}, nil
}

func requireText(value strictjson.Value, expected string, code rejection.Code, detail string) error {
	actual, err := text(value)
	if err != nil {
		return err
	}
	if actual != expected {
		return rejection.New(code, detail)
	}
	return nil
}

func parseSemantics(value strictjson.Value) error {
	fields, err := object(value, "name", "sha256")
	if err != nil {
		return err
	}
	if err := requireText(fields["name"], "keyed-finite-table-semantics", rejection.UnknownTag, "unsupported semantics name"); err != nil {
		return err
	}
	return requireText(fields["sha256"], semanticsSHA256, rejection.DigestMismatch, "unsupported semantics digest")
}

func (p *parser) parseEntries(
	value strictjson.Value,
	domain string,
	parseDefinition func(strictjson.Value, protocol.Digest) error,
) (int, error) {
	items, err := array(value)
	if err != nil {
		return 0, err
	}
	var previous string
	for index, item := range items {
		fields, err := object(item, "definition", "id")
		if err != nil {
			return 0, err
		}
		idText, err := text(fields["id"])
		if err != nil {
			return 0, err
		}
		id, err := protocol.ParseDigest(idText)
		if err != nil {
			return 0, err
		}
		if err := requireStrictOrder(previous, idText, index != 0, "content-addressed entries are not sorted and unique by ID"); err != nil {
			return 0, err
		}
		previous = idText
		if err := parseDefinition(fields["definition"], id); err != nil {
			return 0, err
		}
		definitionBytes, err := strictjson.CanonicalBytes(fields["definition"])
		if err != nil {
			return 0, err
		}
		actual := domainDigest(domain, definitionBytes)
		if actual != id {
			return 0, rejection.New(rejection.DigestMismatch, "Axiom IR definition domain digest mismatch")
		}
	}
	return len(items), nil
}
