package protocol

import (
	"crypto/sha256"

	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

const requestDigestDomain = "axiom-independent-check-v0.1:request"

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

var requiredLimits = []struct {
	name string
	unit string
}{
	{"artifact-bytes", "byte"},
	{"bundle-bytes", "byte"},
	{"collection-items", "item"},
	{"json-depth", "level"},
	{"semantic-steps", "step"},
	{"wall-clock", "millisecond"},
	{"working-memory", "byte"},
}

func ParseRequest(data []byte, limits strictjson.Limits) (Request, error) {
	value, err := strictjson.ParseCanonical(data, limits)
	if err != nil {
		return Request{}, err
	}
	root, err := object(value, []string{
		"assurance_policy", "bundle_manifest", "checker_profile",
		"evidence", "limits", "request_version",
	})
	if err != nil {
		return Request{}, err
	}

	version, err := text(root["request_version"])
	if err != nil {
		return Request{}, err
	}
	if version != "0.1" {
		return Request{}, rejection.New(rejection.UnsupportedVersion, "unsupported request version")
	}
	policy, err := parseAssurancePolicy(root["assurance_policy"])
	if err != nil {
		return Request{}, err
	}
	manifestText, err := text(root["bundle_manifest"])
	if err != nil {
		return Request{}, err
	}
	manifestDigest, err := ParseDigest(manifestText)
	if err != nil {
		return Request{}, err
	}
	profile, err := parseCheckerProfile(root["checker_profile"])
	if err != nil {
		return Request{}, err
	}
	evidenceText, err := text(root["evidence"])
	if err != nil {
		return Request{}, err
	}
	evidenceDigest, err := ParseDigest(evidenceText)
	if err != nil {
		return Request{}, err
	}
	requestLimits, err := parseLimits(root["limits"])
	if err != nil {
		return Request{}, err
	}
	return Request{
		AssurancePolicy: policy,
		BundleManifest:  manifestDigest,
		CheckerProfile:  profile,
		DomainDigest:    requestDomainDigest(data),
		Evidence:        evidenceDigest,
		Limits:          requestLimits,
		Version:         version,
	}, nil
}

func requestDomainDigest(data []byte) Digest {
	hash := sha256.New()
	_, _ = hash.Write([]byte(requestDigestDomain))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(data)
	var digest Digest
	copy(digest[:], hash.Sum(nil))
	return digest
}

func parseAssurancePolicy(value strictjson.Value) (AssurancePolicy, error) {
	fields, err := object(value, []string{"allowed_trust_categories", "proof_support"})
	if err != nil {
		return AssurancePolicy{}, err
	}
	categories, err := strings(fields["allowed_trust_categories"])
	if err != nil {
		return AssurancePolicy{}, err
	}
	if !strictlySorted(categories) && len(categories) > 1 {
		return AssurancePolicy{}, rejection.New(rejection.NoncanonicalOrder, "trust categories are not sorted and unique")
	}
	for _, category := range categories {
		if _, ok := trustCategories[category]; !ok {
			return AssurancePolicy{}, rejection.New(rejection.UnknownTag, "unknown trust category")
		}
	}
	proofSupport, err := text(fields["proof_support"])
	if err != nil {
		return AssurancePolicy{}, err
	}
	if proofSupport != "attestation-allowed" && proofSupport != "certificate-required" {
		return AssurancePolicy{}, rejection.New(rejection.UnknownTag, "unknown proof support policy")
	}
	return AssurancePolicy{AllowedTrustCategories: categories, ProofSupport: proofSupport}, nil
}

func parseCheckerProfile(value strictjson.Value) (CheckerProfile, error) {
	fields, err := object(value, []string{"name", "version"})
	if err != nil {
		return CheckerProfile{}, err
	}
	name, err := text(fields["name"])
	if err != nil {
		return CheckerProfile{}, err
	}
	version, err := text(fields["version"])
	if err != nil {
		return CheckerProfile{}, err
	}
	if name != "keyed-finite-table-independent-check" {
		return CheckerProfile{}, rejection.New(rejection.UnknownTag, "unknown checker profile")
	}
	if version != "0.1" {
		return CheckerProfile{}, rejection.New(rejection.UnsupportedVersion, "unsupported checker profile version")
	}
	return CheckerProfile{Name: name, Version: version}, nil
}

func parseLimits(value strictjson.Value) ([]Limit, error) {
	items, err := array(value)
	if err != nil {
		return nil, err
	}
	if len(items) != len(requiredLimits) {
		return nil, rejection.New(rejection.LimitSetMismatch, "request must contain the complete seven-item limit set")
	}
	result := make([]Limit, 0, len(items))
	for i, item := range items {
		fields, err := object(item, []string{"name", "unit", "value"})
		if err != nil {
			return nil, err
		}
		name, err := text(fields["name"])
		if err != nil {
			return nil, err
		}
		unit, err := text(fields["unit"])
		if err != nil {
			return nil, err
		}
		if name != requiredLimits[i].name || unit != requiredLimits[i].unit {
			return nil, rejection.New(rejection.LimitSetMismatch, "limit set is missing, reordered, or has a wrong unit")
		}
		number, err := decimal(fields["value"], false)
		if err != nil {
			return nil, err
		}
		result = append(result, Limit{Name: name, Unit: unit, Value: number})
	}
	return result, nil
}
