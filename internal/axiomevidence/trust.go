package axiomevidence

import (
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

var trustCategories = map[string]struct{}{
	"cryptographic-primitive":    {},
	"decoder-normalizer":         {},
	"host-runtime":               {},
	"input-origin":               {},
	"production-generator":       {},
	"proof-backend":              {},
	"sensitivity-classification": {},
	"specification-intent":       {},
}

var uncoveredCategories = map[string]struct{}{
	"host-fidelity-for-all-inputs":    {},
	"legal-regulatory-compliance":     {},
	"long-term-archival-authenticity": {},
	"real-world-intent":               {},
	"resource-performance":            {},
	"source-truth-completeness":       {},
	"timing-memory-log-side-channel":  {},
}

func (p *parser) parseTrust(value strictjson.Value) (int, error) {
	return parseDefinitionEntries(value, domainTrust, p.trust, func(definition strictjson.Value, id protocol.Digest) error {
		fields, err := object(definition, "category", "claim", "mitigations", "scope")
		if err != nil {
			return err
		}
		category, err := requireOneOf(fields["category"], trustCategories, "unsupported Axiom Evidence trust category")
		if err != nil {
			return err
		}
		if _, err := nonemptyText(fields["claim"]); err != nil {
			return err
		}
		mitigations, err := array(fields["mitigations"])
		if err != nil {
			return err
		}
		if len(mitigations) != 0 {
			return rejection.New(rejection.UnknownTag, "locked Axiom Evidence profile does not contain mitigation variants")
		}
		scope, err := p.parseScope(fields["scope"])
		if err != nil {
			return err
		}
		p.trust[id] = trustDefinition{
			category:      category,
			scopeKind:     scope.kind,
			scopeTool:     scope.tool,
			scopeDocument: scope.document,
		}
		return nil
	})
}

func (p *parser) parseUncovered(value strictjson.Value) (int, error) {
	return parseDefinitionEntries(value, domainUncovered, p.uncovered, func(definition strictjson.Value, id protocol.Digest) error {
		fields, err := object(definition, "category", "scope", "statement")
		if err != nil {
			return err
		}
		if _, err := requireOneOf(fields["category"], uncoveredCategories, "unsupported Axiom Evidence uncovered category"); err != nil {
			return err
		}
		if _, err := nonemptyText(fields["statement"]); err != nil {
			return err
		}
		if _, err := p.parseScope(fields["scope"]); err != nil {
			return err
		}
		p.uncovered[id] = struct{}{}
		return nil
	})
}

type trustScope struct {
	kind     string
	tool     protocol.Digest
	document protocol.Digest
}

func (p *parser) parseScope(value strictjson.Value) (trustScope, error) {
	tagValue, err := member(value, "kind")
	if err != nil {
		return trustScope{}, err
	}
	tag, err := text(tagValue)
	if err != nil {
		return trustScope{}, err
	}
	switch tag {
	case "program":
		fields, err := object(value, "ir_document_digest", "kind")
		if err != nil {
			return trustScope{}, err
		}
		document, err := digest(fields["ir_document_digest"])
		if err != nil {
			return trustScope{}, err
		}
		p.documentRefs[document] = struct{}{}
		return trustScope{kind: tag, document: document}, nil
	case "tool":
		fields, err := object(value, "id", "kind")
		if err != nil {
			return trustScope{}, err
		}
		tool, err := digest(fields["id"])
		if err != nil {
			return trustScope{}, err
		}
		p.toolRefs[tool] = struct{}{}
		return trustScope{kind: tag, tool: tool}, nil
	default:
		return trustScope{}, rejection.New(rejection.UnknownTag, "unsupported Axiom Evidence scope kind")
	}
}
