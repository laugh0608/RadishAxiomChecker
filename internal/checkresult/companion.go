package checkresult

import (
	"crypto/sha256"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

const (
	checkerName        = "radishaxiom-independent-checker-go"
	checkerToolchain   = "go1.26.7"
	resultDigestDomain = "axiom-independent-check-v0.1:result"
	resultVersion      = "0.1"
)

// RuntimeTCBComponent binds a source-level TCB declaration to the actual
// implementation artifact used by one checker invocation. Artifact is never
// inferred from checker.source.
type RuntimeTCBComponent struct {
	Artifact protocol.Digest
	Category string
	Version  string
}

// RuntimeIdentity contains the actual binary identities required before an
// in-memory Result can become a canonical companion document.
type RuntimeIdentity struct {
	CheckerArtifact protocol.Digest
	TCB             []RuntimeTCBComponent
}

// ValidateRuntimeIdentity verifies the source/runtime identity pair before a
// bundle is inspected. Evidence and request identities are populated by the
// invocation itself and are deliberately outside this preflight boundary.
func ValidateRuntimeIdentity(boundary IdentityBoundary, runtimeIdentity RuntimeIdentity) error {
	probe := boundary
	probe.Evidence = DocumentIdentity{
		ContentDigest:   boundary.Checker.Source,
		DomainDigest:    boundary.Checker.Source,
		DomainAvailable: true,
	}
	probe.Request = probe.Evidence
	normalizedBoundary, err := normalizeBoundary(probe)
	if err != nil {
		return err
	}
	_, err = normalizeRuntimeIdentity(normalizedBoundary, runtimeIdentity)
	return err
}

type CompanionChecker struct {
	Artifact  protocol.Digest
	Name      string
	Source    protocol.Digest
	Toolchain string
	Version   string
}

// CompanionDocument is the closed abstract value of
// axiom-independent-check-result v0.1. Remaining trust is represented by IDs
// because the public companion format does not repeat Evidence trust categories.
type CompanionDocument struct {
	Checker          CompanionChecker
	Checks           []Check
	Evidence         DocumentIdentity
	MissingArtifacts []protocol.Digest
	RemainingTrust   []protocol.Digest
	Request          DocumentIdentity
	Outcome          Outcome
	TCB              []RuntimeTCBComponent
	DomainDigest     protocol.Digest
	raw              []byte
}

// Bytes returns a copy of the unique canonical companion bytes.
func (document CompanionDocument) Bytes() []byte {
	return append([]byte(nil), document.raw...)
}

// VerifyDomainDigest checks an out-of-band identity without trusting a digest
// supplied by the producer or embedded in the result itself.
func (document CompanionDocument) VerifyDomainDigest(expected protocol.Digest) error {
	if zeroDigest(expected) || document.DomainDigest != expected {
		return fmt.Errorf("result document domain digest mismatch")
	}
	return nil
}

// VerifyIdentity binds a parsed companion back to the invocation's independent
// source boundary and registered runtime artifacts.
func (document CompanionDocument) VerifyIdentity(boundary IdentityBoundary, runtimeIdentity RuntimeIdentity) error {
	normalizedBoundary, err := normalizeBoundary(boundary)
	if err != nil {
		return err
	}
	runtimeTCB, err := normalizeRuntimeIdentity(normalizedBoundary, runtimeIdentity)
	if err != nil {
		return err
	}
	checker := normalizedBoundary.Checker
	if document.Checker.Artifact != runtimeIdentity.CheckerArtifact ||
		document.Checker.Source != checker.Source ||
		document.Checker.Toolchain != checker.Toolchain ||
		document.Checker.Version != checker.Version {
		return fmt.Errorf("companion checker identity does not match invocation")
	}
	if document.Evidence != normalizedBoundary.Evidence || document.Request != normalizedBoundary.Request {
		return fmt.Errorf("companion document identities do not match invocation")
	}
	if len(document.TCB) != len(runtimeTCB) {
		return fmt.Errorf("companion TCB does not match invocation")
	}
	for index := range document.TCB {
		if document.TCB[index] != runtimeTCB[index] {
			return fmt.Errorf("companion TCB does not match invocation")
		}
	}
	return nil
}

// EncodeCompanion turns an already aggregated in-memory result into a
// canonical public companion only when an explicit checker binary and matching
// runtime TCB artifacts are supplied. It parses its own output before returning.
func EncodeCompanion(result Result, runtimeIdentity RuntimeIdentity) (CompanionDocument, error) {
	normalized, err := normalizeResultForCompanion(result)
	if err != nil {
		return CompanionDocument{}, err
	}
	runtimeTCB, err := normalizeRuntimeIdentity(normalized.Boundary, runtimeIdentity)
	if err != nil {
		return CompanionDocument{}, err
	}
	trust := make([]protocol.Digest, 0, len(normalized.RemainingTrust))
	for _, item := range normalized.RemainingTrust {
		trust = append(trust, item.ID)
	}
	document := CompanionDocument{
		Checker: CompanionChecker{
			Artifact:  runtimeIdentity.CheckerArtifact,
			Name:      checkerName,
			Source:    normalized.Boundary.Checker.Source,
			Toolchain: normalized.Boundary.Checker.Toolchain,
			Version:   normalized.Boundary.Checker.Version,
		},
		Checks:           normalized.Checks,
		Evidence:         normalized.Boundary.Evidence,
		MissingArtifacts: normalized.MissingArtifacts,
		RemainingTrust:   trust,
		Request:          normalized.Boundary.Request,
		Outcome:          normalized.Outcome,
		TCB:              runtimeTCB,
	}
	raw, err := encodeCompanionDocument(document)
	if err != nil {
		return CompanionDocument{}, err
	}
	parsed, err := ParseCompanion(raw, strictjson.Limits{
		MaxBytes: math.MaxUint64,
		MaxDepth: math.MaxUint64,
		MaxItems: math.MaxUint64,
		MaxSteps: math.MaxUint64,
	})
	if err != nil {
		return CompanionDocument{}, fmt.Errorf("encoded companion failed self-check: %w", err)
	}
	return parsed, nil
}

func normalizeResultForCompanion(input Result) (Result, error) {
	checks, _, err := normalizeChecks(input.Checks)
	if err != nil {
		return Result{}, err
	}
	missing, err := normalizeDigests(input.MissingArtifacts, "missing artifact")
	if err != nil {
		return Result{}, err
	}
	trust, err := normalizeCompanionTrust(input.RemainingTrust)
	if err != nil {
		return Result{}, err
	}
	boundary, err := normalizeBoundary(input.Boundary)
	if err != nil {
		return Result{}, err
	}
	refs, err := normalizeDigests(input.Outcome.Refs, "result reference")
	if err != nil {
		return Result{}, err
	}
	switch input.Outcome.Kind {
	case ResultAccepted:
		if len(refs) != 0 {
			return Result{}, fmt.Errorf("accepted result contains causal references")
		}
	case ResultAcceptedWithTrust, ResultIncomplete, ResultRejected:
		if len(refs) == 0 {
			return Result{}, fmt.Errorf("%s result has no causal references", input.Outcome.Kind)
		}
	default:
		return Result{}, fmt.Errorf("unknown result kind %q", input.Outcome.Kind)
	}
	return Result{
		Boundary:         boundary,
		Checks:           checks,
		MissingArtifacts: missing,
		RemainingTrust:   trust,
		Outcome:          Outcome{Kind: input.Outcome.Kind, Refs: refs},
	}, nil
}

func normalizeCompanionTrust(input []Trust) ([]Trust, error) {
	result := append([]Trust(nil), input...)
	sort.Slice(result, func(i, j int) bool { return result[i].ID.String() < result[j].ID.String() })
	for index, item := range result {
		if zeroDigest(item.ID) {
			return nil, fmt.Errorf("remaining trust has an invalid ID")
		}
		if _, ok := trustCategories[item.Category]; !ok {
			return nil, fmt.Errorf("remaining trust has unknown category %q", item.Category)
		}
		if index > 0 && result[index-1].ID == item.ID {
			return nil, fmt.Errorf("remaining trust contains duplicate ID %s", item.ID)
		}
	}
	return result, nil
}

func normalizeRuntimeIdentity(boundary IdentityBoundary, input RuntimeIdentity) ([]RuntimeTCBComponent, error) {
	if zeroDigest(input.CheckerArtifact) {
		return nil, fmt.Errorf("checker binary artifact is required")
	}
	if input.CheckerArtifact == boundary.Checker.Source {
		return nil, fmt.Errorf("checker source cannot substitute for checker binary artifact")
	}
	if boundary.Checker.Toolchain != checkerToolchain {
		return nil, fmt.Errorf("canonical companion requires toolchain %s", checkerToolchain)
	}
	if !validVersion(boundary.Checker.Version) {
		return nil, fmt.Errorf("checker version is not an exact non-latest value")
	}
	result := append([]RuntimeTCBComponent(nil), input.TCB...)
	sort.Slice(result, func(i, j int) bool {
		if result[i].Category != result[j].Category {
			return result[i].Category < result[j].Category
		}
		return result[i].Artifact.String() < result[j].Artifact.String()
	})
	declared := make(map[string]int, len(boundary.TCB))
	declaredSources := make(map[string]map[protocol.Digest]struct{}, len(boundary.TCB))
	for _, component := range boundary.TCB {
		key := tcbVersionKey(component.Category, component.Version)
		declared[key]++
		if declaredSources[key] == nil {
			declaredSources[key] = make(map[protocol.Digest]struct{})
		}
		declaredSources[key][component.Source] = struct{}{}
	}
	seen := make(map[string]struct{}, len(result))
	actual := make(map[string]int, len(result))
	for _, component := range result {
		if !validTCBCategory(component.Category) {
			return nil, fmt.Errorf("unknown runtime TCB category %q", component.Category)
		}
		if zeroDigest(component.Artifact) || !validVersion(component.Version) {
			return nil, fmt.Errorf("runtime TCB component %q lacks artifact or exact version", component.Category)
		}
		pair := component.Category + "\x00" + component.Artifact.String()
		if _, duplicate := seen[pair]; duplicate {
			return nil, fmt.Errorf("duplicate runtime TCB component %q", component.Category)
		}
		seen[pair] = struct{}{}
		key := tcbVersionKey(component.Category, component.Version)
		if _, sourceSubstitution := declaredSources[key][component.Artifact]; sourceSubstitution {
			return nil, fmt.Errorf("TCB source cannot substitute for runtime artifact")
		}
		actual[key]++
	}
	if len(result) != len(boundary.TCB) || len(actual) != len(declared) {
		return nil, fmt.Errorf("runtime TCB does not match the source boundary")
	}
	for key, count := range declared {
		if actual[key] != count {
			return nil, fmt.Errorf("runtime TCB does not match the source boundary")
		}
	}
	return result, nil
}

func tcbVersionKey(category, version string) string {
	return category + "\x00" + version
}

func validVersion(version string) bool {
	return version != "" && version != "latest" && utf8.ValidString(version)
}

func validTCBCategory(category string) bool {
	if category == "certificate-checker" {
		return true
	}
	_, ok := requiredTCBCategories[category]
	return ok
}

func encodeCompanionDocument(document CompanionDocument) ([]byte, error) {
	var output strings.Builder
	output.WriteString(`{"checker":{"artifact":`)
	writeCompanionString(&output, document.Checker.Artifact.String())
	output.WriteString(`,"name":`)
	writeCompanionString(&output, document.Checker.Name)
	output.WriteString(`,"source":`)
	writeCompanionString(&output, document.Checker.Source.String())
	output.WriteString(`,"toolchain":`)
	writeCompanionString(&output, document.Checker.Toolchain)
	output.WriteString(`,"version":`)
	writeCompanionString(&output, document.Checker.Version)
	output.WriteString(`},"checks":[`)
	for index, check := range document.Checks {
		if index > 0 {
			output.WriteByte(',')
		}
		output.WriteString(`{"definition":`)
		output.Write(encodeDefinition(check.Definition))
		output.WriteString(`,"id":`)
		writeCompanionString(&output, check.ID.String())
		output.WriteByte('}')
	}
	output.WriteString(`],"evidence":`)
	writeDocumentIdentity(&output, document.Evidence)
	output.WriteString(`,"missing_artifacts":`)
	writeDigestArray(&output, document.MissingArtifacts)
	output.WriteString(`,"remaining_trust":`)
	writeDigestArray(&output, document.RemainingTrust)
	output.WriteString(`,"request":`)
	writeDocumentIdentity(&output, document.Request)
	output.WriteString(`,"result":{"kind":`)
	writeCompanionString(&output, string(document.Outcome.Kind))
	if document.Outcome.Kind != ResultAccepted {
		output.WriteString(`,"refs":`)
		writeDigestArray(&output, document.Outcome.Refs)
	}
	output.WriteString(`},"result_version":"0.1","tcb":[`)
	for index, component := range document.TCB {
		if index > 0 {
			output.WriteByte(',')
		}
		output.WriteString(`{"artifact":`)
		writeCompanionString(&output, component.Artifact.String())
		output.WriteString(`,"category":`)
		writeCompanionString(&output, component.Category)
		output.WriteString(`,"version":`)
		writeCompanionString(&output, component.Version)
		output.WriteByte('}')
	}
	output.WriteString(`]}`)
	raw := []byte(output.String())
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("companion encoder produced invalid UTF-8")
	}
	return raw, nil
}

func writeDocumentIdentity(output *strings.Builder, identity DocumentIdentity) {
	output.WriteString(`{"content_digest":`)
	writeCompanionString(output, identity.ContentDigest.String())
	output.WriteString(`,"document_digest":{"kind":`)
	if identity.DomainAvailable {
		output.WriteString(`"available","value":`)
		writeCompanionString(output, identity.DomainDigest.String())
	} else {
		output.WriteString(`"unavailable"`)
	}
	output.WriteString(`}}`)
}

func writeDigestArray(output *strings.Builder, values []protocol.Digest) {
	output.WriteByte('[')
	for index, value := range values {
		if index > 0 {
			output.WriteByte(',')
		}
		writeCompanionString(output, value.String())
	}
	output.WriteByte(']')
}

func writeCompanionString(output *strings.Builder, value string) {
	const hex = "0123456789abcdef"
	output.WriteByte('"')
	for index := 0; index < len(value); index++ {
		char := value[index]
		switch char {
		case '"', '\\':
			output.WriteByte('\\')
			output.WriteByte(char)
		case '\b':
			output.WriteString(`\b`)
		case '\t':
			output.WriteString(`\t`)
		case '\n':
			output.WriteString(`\n`)
		case '\f':
			output.WriteString(`\f`)
		case '\r':
			output.WriteString(`\r`)
		default:
			if char < 0x20 {
				output.WriteString(`\u00`)
				output.WriteByte(hex[char>>4])
				output.WriteByte(hex[char&0x0f])
			} else {
				output.WriteByte(char)
			}
		}
	}
	output.WriteByte('"')
}

func companionDomainDigest(raw []byte) protocol.Digest {
	hash := sha256.New()
	_, _ = hash.Write([]byte(resultDigestDomain))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(raw)
	var digest protocol.Digest
	copy(digest[:], hash.Sum(nil))
	return digest
}
