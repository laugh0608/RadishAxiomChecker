// Package axiomir implements the independent checker's strict Axiom IR v0.1
// structure and locked obligation-reconstruction model. It does not discharge
// obligations, inspect Evidence states, or aggregate an independent result.
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

	staticObligations []ObligationDefinition
	inputInterfaces   []string
	outputInterfaces  []string
	enums             map[protocol.Digest]enumDefinition
	records           map[protocol.Digest]recordDefinition
	tables            map[protocol.Digest]tableDefinition
	inputTables       map[string]protocol.Digest
	outputTables      map[string]protocol.Digest
	assumeContracts   []protocol.Digest
}

// VerifyDomainDigest binds canonical document bytes to an externally supplied
// Axiom IR document-domain identity without interpreting its source.
func (document Document) VerifyDomainDigest(expected protocol.Digest) error {
	if document.DomainDigest != expected {
		return rejection.New(rejection.DigestMismatch, "Axiom IR document domain digest mismatch")
	}
	return nil
}

// ObligationDefinition is the semantic, result-free definition independently
// reconstructed from Axiom IR. Evidence serialization and result state are not
// part of this model.
type ObligationDefinition struct {
	Expectation string
	Kind        string
	Subject     ObligationSubject
}

// ObligationSubject is the closed anchor union exercised by the locked Axiom
// IR and Evidence v0.1 profiles. Only fields belonging to Kind are populated.
type ObligationSubject struct {
	Kind             string
	Artifact         protocol.Digest
	ID               protocol.Digest
	IRDocumentDigest protocol.Digest
	Path             []string
	Direction        string
	Interface        string
	Name             string
	Category         string
	Scope            protocol.Digest
}

// StaticObligationDefinitions returns a defensive copy of the obligations
// determined solely by the parsed IR and semantics profile.
func (document Document) StaticObligationDefinitions() []ObligationDefinition {
	result := append([]ObligationDefinition(nil), document.staticObligations...)
	for index := range result {
		result[index].Subject.Path = append([]string(nil), result[index].Subject.Path...)
	}
	return result
}

// InputInterfaces returns the canonical input port names used by the benchmark
// profile to reconstruct interface-level conformance obligations.
func (document Document) InputInterfaces() []string {
	return append([]string(nil), document.inputInterfaces...)
}

// OutputInterfaces returns the canonical output names used when the explicit
// benchmark execution boundary contains an output comparison.
func (document Document) OutputInterfaces() []string {
	return append([]string(nil), document.outputInterfaces...)
}

// AssumeContractIDs returns the canonical IR assume-contract set. It exposes
// identities only; concrete truth is checked by a later expression-replay
// slice.
func (document Document) AssumeContractIDs() []protocol.Digest {
	return append([]protocol.Digest(nil), document.assumeContracts...)
}
