package checkresult

import (
	"fmt"
	"sort"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
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

var requiredTCBCategories = map[string]struct{}{
	"canonicalization":        {},
	"checker-core":            {},
	"cryptographic-primitive": {},
	"rule-interpreter":        {},
}

func NewSourceBoundary(source protocol.Digest, toolchain, version string) IdentityBoundary {
	components := make([]TCBComponent, 0, len(requiredTCBCategories))
	for category := range requiredTCBCategories {
		components = append(components, TCBComponent{Category: category, Source: source, Version: version})
	}
	sort.Slice(components, func(i, j int) bool { return components[i].Category < components[j].Category })
	return IdentityBoundary{
		Checker: CheckerBoundary{Source: source, Toolchain: toolchain, Version: version},
		TCB:     components,
	}
}

func Aggregate(input Input) (Result, error) {
	checks, checkByKind, err := normalizeChecks(input.Checks)
	if err != nil {
		return Result{}, err
	}
	missing, err := normalizeDigests(input.MissingArtifacts, "missing artifact")
	if err != nil {
		return Result{}, err
	}
	trust, disallowed, err := normalizeTrust(input.RemainingTrust, input.AllowedTrustCategories)
	if err != nil {
		return Result{}, err
	}
	boundary, err := normalizeBoundary(input.Boundary)
	if err != nil {
		return Result{}, err
	}

	rejected := checkIDs(checks, CheckRejected)
	incomplete := checkIDs(checks, CheckIncomplete)
	trusted := checkIDs(checks, CheckTrusted)
	var outcome Outcome
	switch {
	case len(rejected) != 0:
		outcome = Outcome{Kind: ResultRejected, Refs: rejected}
	case len(incomplete) != 0 || len(missing) != 0 || len(disallowed) != 0:
		refs := append([]protocol.Digest(nil), incomplete...)
		if len(missing) != 0 {
			refs = appendCheckRef(refs, checkByKind, CheckIdentity)
		}
		if len(disallowed) != 0 {
			refs = appendCheckRef(refs, checkByKind, CheckStateSupport)
		}
		refs, err = normalizeDigests(refs, "incomplete result reference")
		if err != nil || len(refs) == 0 {
			return Result{}, fmt.Errorf("incomplete result has no causal check reference")
		}
		outcome = Outcome{Kind: ResultIncomplete, Refs: refs}
	case len(trust) != 0:
		if len(trusted) == 0 {
			trusted = appendCheckRef(trusted, checkByKind, CheckStateSupport)
		}
		trusted, err = normalizeDigests(trusted, "trusted result reference")
		if err != nil || len(trusted) == 0 {
			return Result{}, fmt.Errorf("accepted-with-trust result has no causal check reference")
		}
		outcome = Outcome{Kind: ResultAcceptedWithTrust, Refs: trusted}
	default:
		if len(trusted) != 0 {
			return Result{}, fmt.Errorf("trusted check outcome has no remaining trust")
		}
		outcome = Outcome{Kind: ResultAccepted}
	}

	if outcome.Kind != ResultRejected && (!boundary.Evidence.DomainAvailable || !boundary.Request.DomainAvailable) {
		return Result{}, fmt.Errorf("non-rejected result requires available Evidence and request document identities")
	}
	if outcome.Kind == ResultAccepted || outcome.Kind == ResultAcceptedWithTrust {
		if !hasCompleteCheckSet(checkByKind) {
			return Result{}, fmt.Errorf("accepted result requires all ten check kinds")
		}
	}
	return Result{
		Boundary:         boundary,
		Checks:           checks,
		MissingArtifacts: missing,
		RemainingTrust:   trust,
		Outcome:          outcome,
	}, nil
}

func normalizeChecks(input []Check) ([]Check, map[CheckKind]Check, error) {
	result := make([]Check, 0, len(input))
	byKind := make(map[CheckKind]Check, len(input))
	ids := make(map[protocol.Digest]struct{}, len(input))
	for _, raw := range input {
		check, err := NewCheck(raw.Definition.Kind, raw.Definition.Outcome, raw.Definition.Codes, raw.Definition.Refs)
		if err != nil {
			return nil, nil, err
		}
		if raw.ID != check.ID {
			return nil, nil, fmt.Errorf("check %s has a mismatched domain ID", raw.Definition.Kind)
		}
		if _, duplicate := byKind[check.Definition.Kind]; duplicate {
			return nil, nil, fmt.Errorf("duplicate check kind %s", check.Definition.Kind)
		}
		if _, duplicate := ids[check.ID]; duplicate {
			return nil, nil, fmt.Errorf("duplicate check ID %s", check.ID)
		}
		byKind[check.Definition.Kind] = check
		ids[check.ID] = struct{}{}
		result = append(result, check)
	}
	if len(result) == 0 {
		return nil, nil, fmt.Errorf("result layer has no checks")
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID.String() < result[j].ID.String() })
	return result, byKind, nil
}

func normalizeTrust(input []Trust, allowedInput []string) ([]Trust, []Trust, error) {
	allowed := make(map[string]struct{}, len(allowedInput))
	for index, category := range allowedInput {
		if _, ok := trustCategories[category]; !ok {
			return nil, nil, fmt.Errorf("unknown allowed trust category %q", category)
		}
		if index > 0 && allowedInput[index-1] >= category {
			return nil, nil, fmt.Errorf("allowed trust categories are not sorted and unique")
		}
		allowed[category] = struct{}{}
	}
	result := append([]Trust(nil), input...)
	sort.Slice(result, func(i, j int) bool { return result[i].ID.String() < result[j].ID.String() })
	disallowed := make([]Trust, 0)
	for index, item := range result {
		if zeroDigest(item.ID) {
			return nil, nil, fmt.Errorf("remaining trust has an invalid ID")
		}
		if _, ok := trustCategories[item.Category]; !ok {
			return nil, nil, fmt.Errorf("remaining trust has unknown category %q", item.Category)
		}
		if index > 0 && result[index-1].ID == item.ID {
			return nil, nil, fmt.Errorf("remaining trust contains duplicate ID %s", item.ID)
		}
		if _, ok := allowed[item.Category]; !ok {
			disallowed = append(disallowed, item)
		}
	}
	return result, disallowed, nil
}

func normalizeBoundary(input IdentityBoundary) (IdentityBoundary, error) {
	if zeroDigest(input.Checker.Source) || input.Checker.Toolchain == "" || input.Checker.Version == "" {
		return IdentityBoundary{}, fmt.Errorf("checker source, toolchain, and version must be bound")
	}
	if err := validateDocumentIdentity(input.Evidence, "Evidence"); err != nil {
		return IdentityBoundary{}, err
	}
	if err := validateDocumentIdentity(input.Request, "request"); err != nil {
		return IdentityBoundary{}, err
	}
	result := input
	result.TCB = append([]TCBComponent(nil), input.TCB...)
	sort.Slice(result.TCB, func(i, j int) bool {
		if result.TCB[i].Category != result.TCB[j].Category {
			return result.TCB[i].Category < result.TCB[j].Category
		}
		return result.TCB[i].Source.String() < result.TCB[j].Source.String()
	})
	present := make(map[string]struct{}, len(result.TCB))
	for index, component := range result.TCB {
		if component.Category != "certificate-checker" {
			if _, ok := requiredTCBCategories[component.Category]; !ok {
				return IdentityBoundary{}, fmt.Errorf("unknown TCB category %q", component.Category)
			}
		}
		if zeroDigest(component.Source) || component.Version == "" {
			return IdentityBoundary{}, fmt.Errorf("TCB component %q lacks source or version", component.Category)
		}
		if index > 0 && result.TCB[index-1].Category == component.Category && result.TCB[index-1].Source == component.Source {
			return IdentityBoundary{}, fmt.Errorf("duplicate TCB component %q", component.Category)
		}
		present[component.Category] = struct{}{}
	}
	for category := range requiredTCBCategories {
		if _, ok := present[category]; !ok {
			return IdentityBoundary{}, fmt.Errorf("required TCB component %q is missing", category)
		}
	}
	return result, nil
}

func validateDocumentIdentity(identity DocumentIdentity, name string) error {
	if zeroDigest(identity.ContentDigest) {
		return fmt.Errorf("%s content digest is unavailable", name)
	}
	if identity.DomainAvailable == zeroDigest(identity.DomainDigest) {
		return fmt.Errorf("%s domain availability and digest disagree", name)
	}
	return nil
}

func normalizeDigests(input []protocol.Digest, label string) ([]protocol.Digest, error) {
	result := append([]protocol.Digest(nil), input...)
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	unique := result[:0]
	for _, digest := range result {
		if zeroDigest(digest) {
			return nil, fmt.Errorf("%s has an invalid digest", label)
		}
		if len(unique) != 0 && unique[len(unique)-1] == digest {
			continue
		}
		unique = append(unique, digest)
	}
	return unique, nil
}

func checkIDs(checks []Check, outcome CheckOutcome) []protocol.Digest {
	result := make([]protocol.Digest, 0)
	for _, check := range checks {
		if check.Definition.Outcome == outcome {
			result = append(result, check.ID)
		}
	}
	return result
}

func appendCheckRef(refs []protocol.Digest, checks map[CheckKind]Check, kind CheckKind) []protocol.Digest {
	if check, ok := checks[kind]; ok {
		return append(refs, check.ID)
	}
	return refs
}

func hasCompleteCheckSet(checks map[CheckKind]Check) bool {
	if len(checks) != len(requiredCheckKinds) {
		return false
	}
	for _, kind := range requiredCheckKinds {
		if _, ok := checks[kind]; !ok {
			return false
		}
	}
	return true
}
