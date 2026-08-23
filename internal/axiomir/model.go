// Package axiomir implements the independent checker's first strict Axiom IR
// v0.1 structure slice. It deliberately does not perform obligation rebuild,
// semantic verification, Evidence checking, or result aggregation.
package axiomir

import (
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

// Counts records the closed top-level collection cardinalities observed by
// the structure parser.
type Counts struct {
	Contracts   int
	EnumTypes   int
	Nodes       int
	Outputs     int
	RecordTypes int
	TableTypes  int
}

// Document is the identity-bearing result of structural parsing. Acceptance
// here is not an independent-check result and makes no semantic claim.
type Document struct {
	ContentDigest protocol.Digest
	DomainDigest  protocol.Digest
	Counts        Counts
}

// VerifyDomainDigest binds canonical document bytes to an externally supplied
// Axiom IR document-domain identity without interpreting its source.
func (document Document) VerifyDomainDigest(expected protocol.Digest) error {
	if document.DomainDigest != expected {
		return rejection.New(rejection.DigestMismatch, "Axiom IR document domain digest mismatch")
	}
	return nil
}
