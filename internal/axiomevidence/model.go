package axiomevidence

import (
	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

// Counts reports only the top-level entry counts established by the strict
// structure parser. It does not summarize obligation outcomes.
type Counts struct {
	Artifacts   int
	Executions  int
	Obligations int
	Tools       int
	Trust       int
	Uncovered   int
}

// Document is the identity-bearing result of Axiom Evidence structural parsing.
// It retains only the metadata needed for the separate completeness comparison;
// it is not an independent-check result and makes no obligation-state claim.
type Document struct {
	ContentDigest protocol.Digest
	DomainDigest  protocol.Digest
	Counts        Counts

	irArtifact        protocol.Digest
	irDocumentDigest  protocol.Digest
	obligationProfile string
	executions        map[protocol.Digest]executionDefinition
	obligations       map[protocol.Digest]axiomir.ObligationDefinition
	trust             map[protocol.Digest]trustDefinition
}

// VerifyDomainDigest compares the independently recomputed Evidence document
// domain identity with an external binding.
func (d Document) VerifyDomainDigest(expected protocol.Digest) error {
	if d.DomainDigest != expected {
		return rejection.New(rejection.DigestMismatch, "Axiom Evidence document domain digest mismatch")
	}
	return nil
}

// VerifyIRSubject binds the parsed Evidence subject to an independently parsed
// Axiom IR document. Raw content and document-domain identities remain distinct.
func (d Document) VerifyIRSubject(contentDigest, domainDigest protocol.Digest) error {
	if d.irArtifact != contentDigest {
		return rejection.New(rejection.DigestMismatch, "Axiom Evidence subject IR content digest mismatch")
	}
	if d.irDocumentDigest != domainDigest {
		return rejection.New(rejection.DigestMismatch, "Axiom Evidence subject IR document digest mismatch")
	}
	return nil
}
