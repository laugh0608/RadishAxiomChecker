package axiomevidence

import (
	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

const semanticsSHA256 = "sha256:6b18d65eefa439956db8eebe1f4ce90e08b4def4abf7c718c2605e7528598d0d"

type artifactDefinition struct {
	format        string
	formatVersion string
}

type toolDefinition struct {
	roles map[string]struct{}
}

type executionIO struct {
	artifact protocol.Digest
	role     string
}

type executionDefinition struct {
	inputs  []executionIO
	outputs []executionIO
}

type trustDefinition struct {
	category string
}

type parser struct {
	artifacts   map[protocol.Digest]artifactDefinition
	tools       map[protocol.Digest]toolDefinition
	executions  map[protocol.Digest]executionDefinition
	obligations map[protocol.Digest]axiomir.ObligationDefinition
	trust       map[protocol.Digest]trustDefinition
	uncovered   map[protocol.Digest]struct{}

	artifactRefs   map[protocol.Digest]struct{}
	toolRefs       map[protocol.Digest]struct{}
	executionRefs  map[protocol.Digest]struct{}
	obligationRefs map[protocol.Digest]struct{}
	trustRefs      map[protocol.Digest]struct{}
	documentRefs   map[protocol.Digest]struct{}
	conclusionRefs []protocol.Digest

	producer          protocol.Digest
	obligationProfile string
	irArtifact        protocol.Digest
	irDocumentDigest  protocol.Digest
}

// ParseStructure parses canonical Axiom Evidence v0.1 bytes and verifies the
// closed structure profile exercised by the locked checker bundles. It
// recomputes entry and document identities, builds closed reference indexes,
// and binds the Evidence subject identities. It does not rebuild obligations,
// judge obligation states, replay witnesses, inspect proof support, recompute a
// conclusion, or form an independent four-state result.
func ParseStructure(data []byte, limits strictjson.Limits) (Document, error) {
	value, err := strictjson.ParseCanonical(data, limits)
	if err != nil {
		return Document{}, err
	}
	root, err := object(value,
		"artifacts", "conclusion", "digest_algorithm", "evidence_version",
		"executions", "format", "obligation_profile", "obligations", "producer",
		"subject", "tools", "trust", "uncovered",
	)
	if err != nil {
		return Document{}, err
	}
	if err := requireText(root["format"], "axiom-evidence", rejection.UnknownTag, "unknown Axiom Evidence format"); err != nil {
		return Document{}, err
	}
	if err := requireText(root["evidence_version"], "0.1", rejection.UnsupportedVersion, "unsupported Axiom Evidence version"); err != nil {
		return Document{}, err
	}
	if err := requireText(root["digest_algorithm"], "sha-256", rejection.UnknownTag, "unsupported Axiom Evidence digest algorithm"); err != nil {
		return Document{}, err
	}

	p := parser{
		artifacts:      make(map[protocol.Digest]artifactDefinition),
		tools:          make(map[protocol.Digest]toolDefinition),
		executions:     make(map[protocol.Digest]executionDefinition),
		obligations:    make(map[protocol.Digest]axiomir.ObligationDefinition),
		trust:          make(map[protocol.Digest]trustDefinition),
		uncovered:      make(map[protocol.Digest]struct{}),
		artifactRefs:   make(map[protocol.Digest]struct{}),
		toolRefs:       make(map[protocol.Digest]struct{}),
		executionRefs:  make(map[protocol.Digest]struct{}),
		obligationRefs: make(map[protocol.Digest]struct{}),
		trustRefs:      make(map[protocol.Digest]struct{}),
		documentRefs:   make(map[protocol.Digest]struct{}),
	}
	artifactCount, err := p.parseArtifacts(root["artifacts"])
	if err != nil {
		return Document{}, err
	}
	if err := p.parseSubject(root["subject"]); err != nil {
		return Document{}, err
	}
	p.obligationProfile, err = parseObligationProfile(root["obligation_profile"])
	if err != nil {
		return Document{}, err
	}
	toolCount, err := p.parseTools(root["tools"])
	if err != nil {
		return Document{}, err
	}
	p.producer, err = digest(root["producer"])
	if err != nil {
		return Document{}, err
	}
	p.toolRefs[p.producer] = struct{}{}
	executionCount, err := p.parseExecutions(root["executions"])
	if err != nil {
		return Document{}, err
	}
	trustCount, err := p.parseTrust(root["trust"])
	if err != nil {
		return Document{}, err
	}
	uncoveredCount, err := p.parseUncovered(root["uncovered"])
	if err != nil {
		return Document{}, err
	}
	obligationCount, err := p.parseObligations(root["obligations"])
	if err != nil {
		return Document{}, err
	}
	if err := p.parseConclusion(root["conclusion"]); err != nil {
		return Document{}, err
	}
	if err := p.validateReferences(); err != nil {
		return Document{}, err
	}

	return Document{
		ContentDigest: contentDigest(data),
		DomainDigest:  domainDigest(domainDocument, data),
		Counts: Counts{
			Artifacts:   artifactCount,
			Executions:  executionCount,
			Obligations: obligationCount,
			Tools:       toolCount,
			Trust:       trustCount,
			Uncovered:   uncoveredCount,
		},
		irArtifact:        p.irArtifact,
		irDocumentDigest:  p.irDocumentDigest,
		obligationProfile: p.obligationProfile,
		executions:        cloneExecutions(p.executions),
		obligations:       cloneObligations(p.obligations),
		trust:             cloneTrust(p.trust),
	}, nil
}

func parseObligationProfile(value strictjson.Value) (string, error) {
	fields, err := object(value, "name", "version")
	if err != nil {
		return "", err
	}
	allowed := map[string]struct{}{
		"keyed-finite-table-benchmark":    {},
		"keyed-finite-table-verification": {},
	}
	name, err := requireOneOf(fields["name"], allowed, "unsupported Axiom Evidence obligation profile")
	if err != nil {
		return "", err
	}
	if err := requireText(fields["version"], "0.1", rejection.UnsupportedVersion, "unsupported Axiom Evidence obligation profile version"); err != nil {
		return "", err
	}
	return name, nil
}

func (p *parser) parseSubject(value strictjson.Value) error {
	tagValue, err := member(value, "kind")
	if err != nil {
		return err
	}
	if err := requireText(tagValue, "axiom-ir", rejection.UnknownTag, "unsupported Axiom Evidence subject kind"); err != nil {
		return err
	}
	fields, err := object(value, "ir_artifact", "ir_document_digest", "ir_version", "kind", "semantics")
	if err != nil {
		return err
	}
	if err := requireText(fields["ir_version"], "0.1", rejection.UnsupportedVersion, "unsupported subject Axiom IR version"); err != nil {
		return err
	}
	semantics, err := object(fields["semantics"], "name", "sha256")
	if err != nil {
		return err
	}
	if err := requireText(semantics["name"], "keyed-finite-table-semantics", rejection.UnknownTag, "unsupported subject semantics"); err != nil {
		return err
	}
	if err := requireText(semantics["sha256"], semanticsSHA256, rejection.DigestMismatch, "subject semantics digest mismatch"); err != nil {
		return err
	}
	p.irArtifact, err = digest(fields["ir_artifact"])
	if err != nil {
		return err
	}
	p.irDocumentDigest, err = digest(fields["ir_document_digest"])
	if err != nil {
		return err
	}
	p.artifactRefs[p.irArtifact] = struct{}{}
	return nil
}

func (p *parser) validateReferences() error {
	for ref := range p.artifactRefs {
		if _, ok := p.artifacts[ref]; !ok {
			return rejection.New(rejection.InvalidJSON, "Axiom Evidence contains a dangling artifact reference")
		}
	}
	for ref := range p.toolRefs {
		if _, ok := p.tools[ref]; !ok {
			return rejection.New(rejection.InvalidJSON, "Axiom Evidence contains a dangling tool reference")
		}
	}
	producer, ok := p.tools[p.producer]
	if !ok {
		return rejection.New(rejection.InvalidJSON, "Axiom Evidence producer does not resolve")
	}
	if _, ok := producer.roles["evidence-producer"]; !ok {
		return rejection.New(rejection.InvalidJSON, "Axiom Evidence producer lacks the evidence-producer role")
	}
	for ref := range p.executionRefs {
		if _, ok := p.executions[ref]; !ok {
			return rejection.New(rejection.InvalidJSON, "Axiom Evidence contains a dangling execution reference")
		}
	}
	for ref := range p.obligationRefs {
		if _, ok := p.obligations[ref]; !ok {
			return rejection.New(rejection.InvalidJSON, "Axiom Evidence contains a dangling obligation reference")
		}
	}
	for ref := range p.trustRefs {
		if _, ok := p.trust[ref]; !ok {
			return rejection.New(rejection.InvalidJSON, "Axiom Evidence contains a dangling trust reference")
		}
	}
	for ref := range p.documentRefs {
		if ref != p.irDocumentDigest {
			return rejection.New(rejection.DigestMismatch, "Axiom Evidence contains an IR document reference outside its subject")
		}
	}
	for _, ref := range p.conclusionRefs {
		_, obligation := p.obligations[ref]
		_, execution := p.executions[ref]
		if obligation == execution {
			return rejection.New(rejection.InvalidJSON, "Axiom Evidence conclusion ref is dangling or ambiguous")
		}
	}
	subjectArtifact, ok := p.artifacts[p.irArtifact]
	if !ok || subjectArtifact.format != "axiom-ir" || subjectArtifact.formatVersion != "0.1" {
		return rejection.New(rejection.InvalidJSON, "Axiom Evidence subject does not resolve to an Axiom IR v0.1 artifact")
	}
	return nil
}
