package axiomevidence

import (
	"radishaxiom.dev/independent-checker-go/internal/axiomir"
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
		definition, err := p.parseObligationDefinition(fields["definition"])
		if err != nil {
			return 0, err
		}
		if err := verifyDefinitionID(domainObligation, fields["definition"], id); err != nil {
			return 0, err
		}
		p.obligations[id] = definition
		result, err := p.parseObligationResult(fields["result"])
		if err != nil {
			return 0, err
		}
		p.results[id] = result
		previous = spelling
	}
	return len(items), nil
}

func (p *parser) parseObligationDefinition(value strictjson.Value) (axiomir.ObligationDefinition, error) {
	fields, err := object(value, "expectation", "kind", "subject")
	if err != nil {
		return axiomir.ObligationDefinition{}, err
	}
	expectation, err := requireOneOf(fields["expectation"], obligationExpectations, "unsupported Axiom Evidence obligation expectation")
	if err != nil {
		return axiomir.ObligationDefinition{}, err
	}
	kind, err := requireOneOf(fields["kind"], obligationKinds, "unsupported Axiom Evidence obligation kind")
	if err != nil {
		return axiomir.ObligationDefinition{}, err
	}
	subject, err := p.parseObligationSubject(fields["subject"])
	if err != nil {
		return axiomir.ObligationDefinition{}, err
	}
	return axiomir.ObligationDefinition{Expectation: expectation, Kind: kind, Subject: subject}, nil
}

func (p *parser) parseObligationSubject(value strictjson.Value) (axiomir.ObligationSubject, error) {
	tagValue, err := member(value, "kind")
	if err != nil {
		return axiomir.ObligationSubject{}, err
	}
	tag, err := text(tagValue)
	if err != nil {
		return axiomir.ObligationSubject{}, err
	}
	switch tag {
	case "artifact":
		fields, err := object(value, "artifact", "kind")
		if err != nil {
			return axiomir.ObligationSubject{}, err
		}
		artifact, err := digest(fields["artifact"])
		if err != nil {
			return axiomir.ObligationSubject{}, err
		}
		p.artifactRefs[artifact] = struct{}{}
		return axiomir.ObligationSubject{Kind: tag, Artifact: artifact}, nil
	case "contract", "node":
		fields, err := object(value, "id", "kind")
		if err != nil {
			return axiomir.ObligationSubject{}, err
		}
		id, err := digest(fields["id"])
		if err != nil {
			return axiomir.ObligationSubject{}, err
		}
		return axiomir.ObligationSubject{Kind: tag, ID: id}, nil
	case "contract-path", "node-path":
		fields, err := object(value, "id", "kind", "path")
		if err != nil {
			return axiomir.ObligationSubject{}, err
		}
		id, err := digest(fields["id"])
		if err != nil {
			return axiomir.ObligationSubject{}, err
		}
		path, err := array(fields["path"])
		if err != nil {
			return axiomir.ObligationSubject{}, err
		}
		if len(path) == 0 {
			return axiomir.ObligationSubject{}, rejection.New(rejection.InvalidJSON, "Axiom Evidence obligation path must not be empty")
		}
		parsedPath := make([]string, 0, len(path))
		for _, element := range path {
			text, err := nonemptyText(element)
			if err != nil {
				return axiomir.ObligationSubject{}, err
			}
			parsedPath = append(parsedPath, text)
		}
		return axiomir.ObligationSubject{Kind: tag, ID: id, Path: parsedPath}, nil
	case "document", "program":
		fields, err := object(value, "ir_document_digest", "kind")
		if err != nil {
			return axiomir.ObligationSubject{}, err
		}
		document, err := digest(fields["ir_document_digest"])
		if err != nil {
			return axiomir.ObligationSubject{}, err
		}
		p.documentRefs[document] = struct{}{}
		return axiomir.ObligationSubject{Kind: tag, IRDocumentDigest: document}, nil
	case "field":
		fields, err := object(value, "direction", "interface", "kind", "name")
		if err != nil {
			return axiomir.ObligationSubject{}, err
		}
		direction, err := parseDirection(fields["direction"])
		if err != nil {
			return axiomir.ObligationSubject{}, err
		}
		interfaceName, err := nonemptyText(fields["interface"])
		if err != nil {
			return axiomir.ObligationSubject{}, err
		}
		name, err := nonemptyText(fields["name"])
		if err != nil {
			return axiomir.ObligationSubject{}, err
		}
		return axiomir.ObligationSubject{Kind: tag, Direction: direction, Interface: interfaceName, Name: name}, nil
	case "interface":
		fields, err := object(value, "direction", "kind", "name")
		if err != nil {
			return axiomir.ObligationSubject{}, err
		}
		direction, err := parseDirection(fields["direction"])
		if err != nil {
			return axiomir.ObligationSubject{}, err
		}
		name, err := nonemptyText(fields["name"])
		if err != nil {
			return axiomir.ObligationSubject{}, err
		}
		return axiomir.ObligationSubject{Kind: tag, Direction: direction, Name: name}, nil
	case "trust":
		fields, err := object(value, "category", "kind", "scope")
		if err != nil {
			return axiomir.ObligationSubject{}, err
		}
		category, err := requireOneOf(fields["category"], trustCategories, "unsupported Axiom Evidence trust category")
		if err != nil {
			return axiomir.ObligationSubject{}, err
		}
		trust, err := digest(fields["scope"])
		if err != nil {
			return axiomir.ObligationSubject{}, err
		}
		p.trustRefs[trust] = struct{}{}
		return axiomir.ObligationSubject{Kind: tag, Category: category, Scope: trust}, nil
	default:
		return axiomir.ObligationSubject{}, rejection.New(rejection.UnknownTag, "unsupported Axiom Evidence obligation subject kind")
	}
}

func parseDirection(value strictjson.Value) (string, error) {
	allowed := map[string]struct{}{"input": {}, "output": {}}
	return requireOneOf(value, allowed, "unsupported Axiom Evidence interface direction")
}

func (p *parser) parseObligationResult(value strictjson.Value) (obligationResult, error) {
	tagValue, err := member(value, "kind")
	if err != nil {
		return obligationResult{}, err
	}
	tag, err := text(tagValue)
	if err != nil {
		return obligationResult{}, err
	}
	switch tag {
	case "proved":
		fields, err := object(value, "assumptions", "kind", "support")
		if err != nil {
			return obligationResult{}, err
		}
		assumptions, err := p.recordTrustSet(fields["assumptions"], false)
		if err != nil {
			return obligationResult{}, err
		}
		support, err := p.parseSupport(fields["support"])
		return obligationResult{kind: tag, assumptions: assumptions, support: support}, err
	case "checked":
		fields, err := object(value, "artifacts", "assumptions", "execution", "kind")
		if err != nil {
			return obligationResult{}, err
		}
		artifacts, err := p.recordArtifactSet(fields["artifacts"], false)
		if err != nil {
			return obligationResult{}, err
		}
		assumptions, err := p.recordTrustSet(fields["assumptions"], false)
		if err != nil {
			return obligationResult{}, err
		}
		execution, err := p.recordExecution(fields["execution"])
		return obligationResult{kind: tag, artifacts: artifacts, assumptions: assumptions, execution: execution}, err
	case "unknown":
		fields, err := object(value, "attempts", "kind", "reason")
		if err != nil {
			return obligationResult{}, err
		}
		attempts, err := parseDigestSet(fields["attempts"], true, "Axiom Evidence unknown attempts are empty, unsorted, or duplicate")
		if err != nil {
			return obligationResult{}, err
		}
		for _, attempt := range attempts {
			p.executionRefs[attempt] = struct{}{}
		}
		allowed := map[string]struct{}{
			"backend-unavailable": {}, "incomplete-certificate": {}, "indeterminate": {},
			"operational-error": {}, "resource-exhausted": {}, "timeout": {}, "unsupported": {},
		}
		reason, err := requireOneOf(fields["reason"], allowed, "unsupported Axiom Evidence unknown reason")
		return obligationResult{kind: tag, attempts: attempts, reason: reason}, err
	case "failed":
		fields, err := object(value, "assumptions", "counterexample", "execution", "kind")
		if err != nil {
			return obligationResult{}, err
		}
		assumptions, err := p.recordTrustSet(fields["assumptions"], false)
		if err != nil {
			return obligationResult{}, err
		}
		counterexample, err := p.parseCounterexample(fields["counterexample"])
		if err != nil {
			return obligationResult{}, err
		}
		execution, err := p.recordExecution(fields["execution"])
		return obligationResult{
			kind: tag, assumptions: assumptions, execution: execution, counterexample: &counterexample,
		}, err
	case "trusted":
		fields, err := object(value, "kind", "trust")
		if err != nil {
			return obligationResult{}, err
		}
		trust, err := digest(fields["trust"])
		if err != nil {
			return obligationResult{}, err
		}
		p.trustRefs[trust] = struct{}{}
		return obligationResult{kind: tag, trust: trust}, nil
	default:
		return obligationResult{}, rejection.New(rejection.UnknownTag, "unsupported Axiom Evidence obligation result kind")
	}
}

func (p *parser) parseSupport(value strictjson.Value) (proofSupport, error) {
	tagValue, err := member(value, "kind")
	if err != nil {
		return proofSupport{}, err
	}
	tag, err := text(tagValue)
	if err != nil {
		return proofSupport{}, err
	}
	switch tag {
	case "kernel-replay":
		fields, err := object(value, "execution", "kind")
		if err != nil {
			return proofSupport{}, err
		}
		execution, err := p.recordExecution(fields["execution"])
		return proofSupport{kind: tag, execution: execution}, err
	case "backend-attestation":
		fields, err := object(value, "execution", "kind", "query", "response", "trust")
		if err != nil {
			return proofSupport{}, err
		}
		execution, err := p.recordExecution(fields["execution"])
		if err != nil {
			return proofSupport{}, err
		}
		query, err := digest(fields["query"])
		if err != nil {
			return proofSupport{}, err
		}
		response, err := digest(fields["response"])
		if err != nil {
			return proofSupport{}, err
		}
		p.artifactRefs[query] = struct{}{}
		p.artifactRefs[response] = struct{}{}
		trust, err := digest(fields["trust"])
		if err != nil {
			return proofSupport{}, err
		}
		p.trustRefs[trust] = struct{}{}
		return proofSupport{kind: tag, execution: execution, query: query, response: response, trust: trust}, nil
	default:
		return proofSupport{}, rejection.New(rejection.UnknownTag, "unsupported Axiom Evidence proof support kind")
	}
}

func (p *parser) recordArtifactSet(value strictjson.Value, nonempty bool) ([]protocol.Digest, error) {
	refs, err := parseDigestSet(value, nonempty, "Axiom Evidence artifact refs are empty, unsorted, or duplicate")
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		p.artifactRefs[ref] = struct{}{}
	}
	return refs, nil
}

func (p *parser) recordTrustSet(value strictjson.Value, nonempty bool) ([]protocol.Digest, error) {
	refs, err := parseDigestSet(value, nonempty, "Axiom Evidence trust refs are empty, unsorted, or duplicate")
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		p.trustRefs[ref] = struct{}{}
	}
	return refs, nil
}

func (p *parser) recordExecution(value strictjson.Value) (protocol.Digest, error) {
	ref, err := digest(value)
	if err != nil {
		return protocol.Digest{}, err
	}
	p.executionRefs[ref] = struct{}{}
	return ref, nil
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
