package axiomevidence

import (
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

var obligationExpectations = map[string]struct{}{
	"check": {},
	"prove": {},
	"trust": {},
}

var obligationKinds = map[string]struct{}{
	"contract-guarantee": {},
	"effect-empty":       {},
	"field-origin":       {},
	"group-conservation": {},
	"host-conformance":   {},
	"input-conformance":  {},
	"ir-structure":       {},
	"key-cardinality":    {},
	"noninterference":    {},
	"numeric-range":      {},
	"output-conformance": {},
	"row-coverage":       {},
	"totality":           {},
	"trust-boundary":     {},
}

func (p *parser) parseObligations(value strictjson.Value) (int, error) {
	items, err := array(value)
	if err != nil {
		return 0, err
	}
	var previous string
	for position, item := range items {
		fields, err := object(item, "definition", "id", "result")
		if err != nil {
			return 0, err
		}
		spelling, err := text(fields["id"])
		if err != nil {
			return 0, err
		}
		if err := requireStrictOrder(previous, spelling, position != 0, "Axiom Evidence obligations are not sorted and unique by ID"); err != nil {
			return 0, err
		}
		id, err := protocol.ParseDigest(spelling)
		if err != nil {
			return 0, err
		}
		if err := p.parseObligationDefinition(fields["definition"]); err != nil {
			return 0, err
		}
		if err := verifyDefinitionID(domainObligation, fields["definition"], id); err != nil {
			return 0, err
		}
		p.obligations[id] = struct{}{}
		if err := p.parseObligationResult(fields["result"]); err != nil {
			return 0, err
		}
		previous = spelling
	}
	return len(items), nil
}

func (p *parser) parseObligationDefinition(value strictjson.Value) error {
	fields, err := object(value, "expectation", "kind", "subject")
	if err != nil {
		return err
	}
	if _, err := requireOneOf(fields["expectation"], obligationExpectations, "unsupported Axiom Evidence obligation expectation"); err != nil {
		return err
	}
	if _, err := requireOneOf(fields["kind"], obligationKinds, "unsupported Axiom Evidence obligation kind"); err != nil {
		return err
	}
	return p.parseObligationSubject(fields["subject"])
}

func (p *parser) parseObligationSubject(value strictjson.Value) error {
	tagValue, err := member(value, "kind")
	if err != nil {
		return err
	}
	tag, err := text(tagValue)
	if err != nil {
		return err
	}
	switch tag {
	case "artifact":
		fields, err := object(value, "artifact", "kind")
		if err != nil {
			return err
		}
		artifact, err := digest(fields["artifact"])
		if err != nil {
			return err
		}
		p.artifactRefs[artifact] = struct{}{}
		return nil
	case "contract", "node":
		fields, err := object(value, "id", "kind")
		if err != nil {
			return err
		}
		_, err = digest(fields["id"])
		return err
	case "contract-path", "node-path":
		fields, err := object(value, "id", "kind", "path")
		if err != nil {
			return err
		}
		if _, err := digest(fields["id"]); err != nil {
			return err
		}
		path, err := array(fields["path"])
		if err != nil {
			return err
		}
		if len(path) == 0 {
			return rejection.New(rejection.InvalidJSON, "Axiom Evidence obligation path must not be empty")
		}
		for _, element := range path {
			if _, err := nonemptyText(element); err != nil {
				return err
			}
		}
		return nil
	case "document", "program":
		fields, err := object(value, "ir_document_digest", "kind")
		if err != nil {
			return err
		}
		document, err := digest(fields["ir_document_digest"])
		if err != nil {
			return err
		}
		p.documentRefs[document] = struct{}{}
		return nil
	case "field":
		fields, err := object(value, "direction", "interface", "kind", "name")
		if err != nil {
			return err
		}
		if err := parseDirection(fields["direction"]); err != nil {
			return err
		}
		if _, err := nonemptyText(fields["interface"]); err != nil {
			return err
		}
		_, err = nonemptyText(fields["name"])
		return err
	case "interface":
		fields, err := object(value, "direction", "kind", "name")
		if err != nil {
			return err
		}
		if err := parseDirection(fields["direction"]); err != nil {
			return err
		}
		_, err = nonemptyText(fields["name"])
		return err
	case "trust":
		fields, err := object(value, "category", "kind", "scope")
		if err != nil {
			return err
		}
		if _, err := requireOneOf(fields["category"], trustCategories, "unsupported Axiom Evidence trust category"); err != nil {
			return err
		}
		trust, err := digest(fields["scope"])
		if err != nil {
			return err
		}
		p.trustRefs[trust] = struct{}{}
		return nil
	default:
		return rejection.New(rejection.UnknownTag, "unsupported Axiom Evidence obligation subject kind")
	}
}

func parseDirection(value strictjson.Value) error {
	allowed := map[string]struct{}{"input": {}, "output": {}}
	_, err := requireOneOf(value, allowed, "unsupported Axiom Evidence interface direction")
	return err
}

func (p *parser) parseObligationResult(value strictjson.Value) error {
	tagValue, err := member(value, "kind")
	if err != nil {
		return err
	}
	tag, err := text(tagValue)
	if err != nil {
		return err
	}
	switch tag {
	case "proved":
		fields, err := object(value, "assumptions", "kind", "support")
		if err != nil {
			return err
		}
		if err := p.recordTrustSet(fields["assumptions"], false); err != nil {
			return err
		}
		return p.parseSupport(fields["support"])
	case "checked":
		fields, err := object(value, "artifacts", "assumptions", "execution", "kind")
		if err != nil {
			return err
		}
		if err := p.recordArtifactSet(fields["artifacts"], false); err != nil {
			return err
		}
		if err := p.recordTrustSet(fields["assumptions"], false); err != nil {
			return err
		}
		return p.recordExecution(fields["execution"])
	case "unknown":
		fields, err := object(value, "attempts", "kind", "reason")
		if err != nil {
			return err
		}
		attempts, err := parseDigestSet(fields["attempts"], true, "Axiom Evidence unknown attempts are empty, unsorted, or duplicate")
		if err != nil {
			return err
		}
		for _, attempt := range attempts {
			p.executionRefs[attempt] = struct{}{}
		}
		allowed := map[string]struct{}{"backend-unavailable": {}, "timeout": {}}
		_, err = requireOneOf(fields["reason"], allowed, "unsupported Axiom Evidence unknown reason")
		return err
	case "failed":
		fields, err := object(value, "assumptions", "counterexample", "execution", "kind")
		if err != nil {
			return err
		}
		if err := p.recordTrustSet(fields["assumptions"], false); err != nil {
			return err
		}
		if err := p.parseCounterexample(fields["counterexample"]); err != nil {
			return err
		}
		return p.recordExecution(fields["execution"])
	case "trusted":
		fields, err := object(value, "kind", "trust")
		if err != nil {
			return err
		}
		trust, err := digest(fields["trust"])
		if err != nil {
			return err
		}
		p.trustRefs[trust] = struct{}{}
		return nil
	default:
		return rejection.New(rejection.UnknownTag, "unsupported Axiom Evidence obligation result kind")
	}
}

func (p *parser) parseSupport(value strictjson.Value) error {
	tagValue, err := member(value, "kind")
	if err != nil {
		return err
	}
	tag, err := text(tagValue)
	if err != nil {
		return err
	}
	switch tag {
	case "kernel-replay":
		fields, err := object(value, "execution", "kind")
		if err != nil {
			return err
		}
		return p.recordExecution(fields["execution"])
	case "backend-attestation":
		fields, err := object(value, "execution", "kind", "query", "response", "trust")
		if err != nil {
			return err
		}
		if err := p.recordExecution(fields["execution"]); err != nil {
			return err
		}
		for _, name := range []string{"query", "response"} {
			artifact, err := digest(fields[name])
			if err != nil {
				return err
			}
			p.artifactRefs[artifact] = struct{}{}
		}
		trust, err := digest(fields["trust"])
		if err != nil {
			return err
		}
		p.trustRefs[trust] = struct{}{}
		return nil
	default:
		return rejection.New(rejection.UnknownTag, "unsupported Axiom Evidence proof support kind")
	}
}

func (p *parser) recordArtifactSet(value strictjson.Value, nonempty bool) error {
	refs, err := parseDigestSet(value, nonempty, "Axiom Evidence artifact refs are empty, unsorted, or duplicate")
	if err != nil {
		return err
	}
	for _, ref := range refs {
		p.artifactRefs[ref] = struct{}{}
	}
	return nil
}

func (p *parser) recordTrustSet(value strictjson.Value, nonempty bool) error {
	refs, err := parseDigestSet(value, nonempty, "Axiom Evidence trust refs are empty, unsorted, or duplicate")
	if err != nil {
		return err
	}
	for _, ref := range refs {
		p.trustRefs[ref] = struct{}{}
	}
	return nil
}

func (p *parser) recordExecution(value strictjson.Value) error {
	ref, err := digest(value)
	if err != nil {
		return err
	}
	p.executionRefs[ref] = struct{}{}
	return nil
}

func (p *parser) parseConclusion(value strictjson.Value) error {
	fields, err := object(value, "kind", "refs")
	if err != nil {
		return err
	}
	allowed := map[string]struct{}{
		"implementation_inconsistent": {},
		"inconclusive":                {},
		"input_rejected":              {},
		"satisfied":                   {},
		"violated":                    {},
	}
	if _, err := requireOneOf(fields["kind"], allowed, "unsupported Axiom Evidence conclusion kind"); err != nil {
		return err
	}
	p.conclusionRefs, err = parseDigestSet(fields["refs"], false, "Axiom Evidence conclusion refs are unsorted or duplicate")
	return err
}
