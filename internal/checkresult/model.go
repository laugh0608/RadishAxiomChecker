package checkresult

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
)

const checkDigestDomain = "axiom-independent-check-v0.1:check"

type CheckKind string

const (
	CheckConclusion           CheckKind = "conclusion-recompute"
	CheckConcreteReplay       CheckKind = "concrete-check-replay"
	CheckCounterexampleReplay CheckKind = "counterexample-replay"
	CheckIdentity             CheckKind = "identity"
	CheckIsolation            CheckKind = "isolation-report"
	CheckObligation           CheckKind = "obligation-reconstruction"
	CheckProofSupport         CheckKind = "proof-support"
	CheckStateSupport         CheckKind = "state-support"
	CheckStrictParse          CheckKind = "strict-parse"
	CheckSubject              CheckKind = "subject"
)

var requiredCheckKinds = []CheckKind{
	CheckConclusion,
	CheckConcreteReplay,
	CheckCounterexampleReplay,
	CheckIdentity,
	CheckIsolation,
	CheckObligation,
	CheckProofSupport,
	CheckStateSupport,
	CheckStrictParse,
	CheckSubject,
}

type CheckOutcome string

const (
	CheckIncomplete CheckOutcome = "incomplete"
	CheckPassed     CheckOutcome = "passed"
	CheckRejected   CheckOutcome = "rejected"
	CheckTrusted    CheckOutcome = "trusted"
)

type RefKind string

const (
	RefArtifact      RefKind = "artifact"
	RefCheck         RefKind = "check"
	RefEvidenceEntry RefKind = "evidence-entry"
	RefObligation    RefKind = "obligation"
	RefRequest       RefKind = "request"
	RefTool          RefKind = "tool"
	RefTrust         RefKind = "trust"
)

type Ref struct {
	Kind RefKind
	ID   protocol.Digest
}

type CheckDefinition struct {
	Codes   []string
	Kind    CheckKind
	Outcome CheckOutcome
	Refs    []Ref
}

type Check struct {
	Definition CheckDefinition
	ID         protocol.Digest
}

var checkCodeRegistry = map[CheckKind]map[string]struct{}{
	CheckConclusion:           codes("conclusion-mismatch", "result-aggregation"),
	CheckConcreteReplay:       codes("concrete-check-mismatch", "host-output-mismatch"),
	CheckCounterexampleReplay: codes("counterexample-invalid", "minimality-unsupported"),
	CheckIdentity: codes(
		"artifact-missing", "check-id-mismatch", "digest-mismatch", "duplicate-artifact",
		"evidence-cardinality", "length-mismatch", "manifest-coverage", "request-binding-mismatch",
	),
	CheckIsolation:  codes("checker-identity", "isolation-boundary-violation", "tcb-incomplete"),
	CheckObligation: codes("obligation-mismatch"),
	CheckProofSupport: codes(
		"attestation-not-allowed", "certificate-incomplete", "proof-support-mismatch", "proof-support-unsupported",
	),
	CheckStateSupport: codes("invalid-state-support", "trust-not-allowed"),
	CheckStrictParse: codes(
		"duplicate-member", "evidence-missing-required-members", "invalid-json", "invalid-utf8",
		"json-number-or-null", "limit-set-mismatch", "missing-required-member", "noncanonical-json",
		"noncanonical-order", "unknown-member", "unknown-tag", "unsupported-version",
	),
	CheckSubject: codes("invalid-ir", "subject-mismatch"),
}

func NewCheck(kind CheckKind, outcome CheckOutcome, rawCodes []string, rawRefs []Ref) (Check, error) {
	definition := CheckDefinition{
		Codes:   append([]string(nil), rawCodes...),
		Kind:    kind,
		Outcome: outcome,
		Refs:    append([]Ref(nil), rawRefs...),
	}
	if err := normalizeDefinition(&definition); err != nil {
		return Check{}, err
	}
	encoded := encodeDefinition(definition)
	hash := sha256.New()
	_, _ = hash.Write([]byte(checkDigestDomain))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(encoded)
	var id protocol.Digest
	copy(id[:], hash.Sum(nil))
	return Check{Definition: definition, ID: id}, nil
}

func normalizeDefinition(definition *CheckDefinition) error {
	allowedCodes, ok := checkCodeRegistry[definition.Kind]
	if !ok {
		return fmt.Errorf("unknown check kind %q", definition.Kind)
	}
	switch definition.Outcome {
	case CheckIncomplete, CheckPassed, CheckRejected, CheckTrusted:
	default:
		return fmt.Errorf("unknown check outcome %q", definition.Outcome)
	}
	if len(definition.Codes) == 0 {
		return fmt.Errorf("check %s has no stable code", definition.Kind)
	}
	sort.Strings(definition.Codes)
	for index, code := range definition.Codes {
		if _, ok := allowedCodes[code]; !ok {
			return fmt.Errorf("code %q does not belong to check %s", code, definition.Kind)
		}
		if index > 0 && definition.Codes[index-1] == code {
			return fmt.Errorf("check %s contains duplicate code %q", definition.Kind, code)
		}
	}
	sort.Slice(definition.Refs, func(i, j int) bool {
		if definition.Refs[i].Kind != definition.Refs[j].Kind {
			return definition.Refs[i].Kind < definition.Refs[j].Kind
		}
		return definition.Refs[i].ID.String() < definition.Refs[j].ID.String()
	})
	for index, ref := range definition.Refs {
		if !validRefKind(ref.Kind) || zeroDigest(ref.ID) {
			return fmt.Errorf("check %s contains an invalid reference", definition.Kind)
		}
		if index > 0 && definition.Refs[index-1] == ref {
			return fmt.Errorf("check %s contains a duplicate reference", definition.Kind)
		}
	}
	return nil
}

func encodeDefinition(definition CheckDefinition) []byte {
	var output strings.Builder
	output.WriteString(`{"codes":[`)
	for index, code := range definition.Codes {
		if index > 0 {
			output.WriteByte(',')
		}
		writeJSONString(&output, code)
	}
	output.WriteString(`],"kind":`)
	writeJSONString(&output, string(definition.Kind))
	output.WriteString(`,"outcome":`)
	writeJSONString(&output, string(definition.Outcome))
	output.WriteString(`,"refs":[`)
	for index, ref := range definition.Refs {
		if index > 0 {
			output.WriteByte(',')
		}
		output.WriteString(`{"kind":`)
		writeJSONString(&output, string(ref.Kind))
		output.WriteString(`,"ref":`)
		writeJSONString(&output, ref.ID.String())
		output.WriteByte('}')
	}
	output.WriteString(`]}`)
	return []byte(output.String())
}

func writeJSONString(output *strings.Builder, value string) {
	output.WriteByte('"')
	output.WriteString(value)
	output.WriteByte('"')
}

func codes(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func validRefKind(kind RefKind) bool {
	switch kind {
	case RefArtifact, RefCheck, RefEvidenceEntry, RefObligation, RefRequest, RefTool, RefTrust:
		return true
	default:
		return false
	}
}

func zeroDigest(digest protocol.Digest) bool {
	return digest == protocol.Digest{}
}

type DocumentIdentity struct {
	ContentDigest   protocol.Digest
	DomainDigest    protocol.Digest
	DomainAvailable bool
}

type CheckerBoundary struct {
	Source    protocol.Digest
	Toolchain string
	Version   string
}

type TCBComponent struct {
	Category string
	Source   protocol.Digest
	Version  string
}

type IdentityBoundary struct {
	Checker  CheckerBoundary
	Evidence DocumentIdentity
	Request  DocumentIdentity
	TCB      []TCBComponent
}

type Trust struct {
	ID       protocol.Digest
	Category string
}

type ResultKind string

const (
	ResultAccepted          ResultKind = "accepted"
	ResultAcceptedWithTrust ResultKind = "accepted-with-trust"
	ResultIncomplete        ResultKind = "incomplete"
	ResultRejected          ResultKind = "rejected"
)

type Outcome struct {
	Kind ResultKind
	Refs []protocol.Digest
}

type Input struct {
	AllowedTrustCategories []string
	Boundary               IdentityBoundary
	Checks                 []Check
	MissingArtifacts       []protocol.Digest
	RemainingTrust         []Trust
}

type Result struct {
	Boundary         IdentityBoundary
	Checks           []Check
	MissingArtifacts []protocol.Digest
	RemainingTrust   []Trust
	Outcome          Outcome
}
