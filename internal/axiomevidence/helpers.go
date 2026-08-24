package axiomevidence

import (
	"crypto/sha256"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

const (
	domainDocument   = "axiom-evidence-v0.1:document"
	domainExecution  = "axiom-evidence-v0.1:execution"
	domainObligation = "axiom-evidence-v0.1:obligation"
	domainTool       = "axiom-evidence-v0.1:tool"
	domainTrust      = "axiom-evidence-v0.1:trust"
	domainUncovered  = "axiom-evidence-v0.1:uncovered"
)

func object(value strictjson.Value, required ...string) (map[string]strictjson.Value, error) {
	members, ok := value.Members()
	if !ok {
		return nil, rejection.New(rejection.InvalidJSON, "expected Axiom Evidence object")
	}
	allowed := make(map[string]struct{}, len(required))
	for _, name := range required {
		allowed[name] = struct{}{}
	}
	fields := make(map[string]strictjson.Value, len(members))
	for _, candidate := range members {
		if _, ok := allowed[candidate.Name]; !ok {
			return nil, rejection.New(rejection.UnknownMember, "Axiom Evidence object contains an unknown member")
		}
		fields[candidate.Name] = candidate.Value
	}
	for _, name := range required {
		if _, ok := fields[name]; !ok {
			return nil, rejection.New(rejection.MissingRequiredMember, "Axiom Evidence object is missing a required member")
		}
	}
	return fields, nil
}

func member(value strictjson.Value, name string) (strictjson.Value, error) {
	members, ok := value.Members()
	if !ok {
		return strictjson.Value{}, rejection.New(rejection.InvalidJSON, "expected tagged Axiom Evidence object")
	}
	for _, candidate := range members {
		if candidate.Name == name {
			return candidate.Value, nil
		}
	}
	return strictjson.Value{}, rejection.New(rejection.MissingRequiredMember, "tagged Axiom Evidence object is missing its tag")
}

func text(value strictjson.Value) (string, error) {
	result, ok := value.Text()
	if !ok {
		return "", rejection.New(rejection.InvalidJSON, "expected Axiom Evidence string")
	}
	return result, nil
}

func nonemptyText(value strictjson.Value) (string, error) {
	result, err := text(value)
	if err != nil {
		return "", err
	}
	if result == "" {
		return "", rejection.New(rejection.InvalidJSON, "Axiom Evidence string must not be empty")
	}
	for _, r := range result {
		if r <= 0x1f || 0x7f <= r && r <= 0x9f {
			return "", rejection.New(rejection.InvalidJSON, "Axiom Evidence string contains a control character")
		}
	}
	return result, nil
}

func array(value strictjson.Value) ([]strictjson.Value, error) {
	result, ok := value.Items()
	if !ok {
		return nil, rejection.New(rejection.InvalidJSON, "expected Axiom Evidence array")
	}
	return result, nil
}

func digest(value strictjson.Value) (protocol.Digest, error) {
	spelling, err := text(value)
	if err != nil {
		return protocol.Digest{}, err
	}
	return protocol.ParseDigest(spelling)
}

func requireText(value strictjson.Value, expected string, code rejection.Code, detail string) error {
	actual, err := text(value)
	if err != nil {
		return err
	}
	if actual != expected {
		return rejection.New(code, detail)
	}
	return nil
}

func requireOneOf(value strictjson.Value, allowed map[string]struct{}, detail string) (string, error) {
	actual, err := text(value)
	if err != nil {
		return "", err
	}
	if _, ok := allowed[actual]; !ok {
		return "", rejection.New(rejection.UnknownTag, detail)
	}
	return actual, nil
}

func requireStrictOrder(previous, current string, present bool, detail string) error {
	if present && previous >= current {
		return rejection.New(rejection.NoncanonicalOrder, detail)
	}
	return nil
}

func parseDigestSet(value strictjson.Value, nonempty bool, detail string) ([]protocol.Digest, error) {
	items, err := array(value)
	if err != nil {
		return nil, err
	}
	if nonempty && len(items) == 0 {
		return nil, rejection.New(rejection.InvalidJSON, detail)
	}
	result := make([]protocol.Digest, 0, len(items))
	var previous string
	for index, item := range items {
		spelling, err := text(item)
		if err != nil {
			return nil, err
		}
		if err := requireStrictOrder(previous, spelling, index != 0, detail); err != nil {
			return nil, err
		}
		parsed, err := protocol.ParseDigest(spelling)
		if err != nil {
			return nil, err
		}
		previous = spelling
		result = append(result, parsed)
	}
	return result, nil
}

func parseStringSet(value strictjson.Value, nonempty bool, allowed map[string]struct{}, detail string) ([]string, error) {
	items, err := array(value)
	if err != nil {
		return nil, err
	}
	if nonempty && len(items) == 0 {
		return nil, rejection.New(rejection.InvalidJSON, detail)
	}
	result := make([]string, 0, len(items))
	var previous string
	for index, item := range items {
		current, err := nonemptyText(item)
		if err != nil {
			return nil, err
		}
		if allowed != nil {
			if _, ok := allowed[current]; !ok {
				return nil, rejection.New(rejection.UnknownTag, detail)
			}
		}
		if err := requireStrictOrder(previous, current, index != 0, detail); err != nil {
			return nil, err
		}
		previous = current
		result = append(result, current)
	}
	return result, nil
}

func canonicalUnsigned(value strictjson.Value) (string, error) {
	raw, err := text(value)
	if err != nil {
		return "", err
	}
	if raw == "" || len(raw) > 1 && raw[0] == '0' {
		return "", rejection.New(rejection.InvalidJSON, "integer is not a canonical nonnegative decimal string")
	}
	for i := range raw {
		if raw[i] < '0' || raw[i] > '9' {
			return "", rejection.New(rejection.InvalidJSON, "integer is not a canonical nonnegative decimal string")
		}
	}
	return raw, nil
}

func canonicalSigned(value strictjson.Value) (string, error) {
	raw, err := text(value)
	if err != nil {
		return "", err
	}
	if raw == "" || raw == "-0" || raw[0] == '+' || len(raw) > 1 && raw[0] == '0' {
		return "", rejection.New(rejection.InvalidJSON, "integer is not a canonical signed decimal string")
	}
	start := 0
	if raw[0] == '-' {
		if len(raw) == 1 || raw[1] == '0' {
			return "", rejection.New(rejection.InvalidJSON, "integer is not a canonical signed decimal string")
		}
		start = 1
	}
	for i := start; i < len(raw); i++ {
		if raw[i] < '0' || raw[i] > '9' {
			return "", rejection.New(rejection.InvalidJSON, "integer is not a canonical signed decimal string")
		}
	}
	return raw, nil
}

func domainDigest(domain string, payload []byte) protocol.Digest {
	hash := sha256.New()
	hash.Write([]byte(domain))
	hash.Write([]byte{0})
	hash.Write(payload)
	var result protocol.Digest
	copy(result[:], hash.Sum(nil))
	return result
}

func contentDigest(payload []byte) protocol.Digest {
	return protocol.Digest(sha256.Sum256(payload))
}

func verifyDefinitionID(domain string, definition strictjson.Value, expected protocol.Digest) error {
	definitionBytes, err := strictjson.CanonicalBytes(definition)
	if err != nil {
		return err
	}
	if actual := domainDigest(domain, definitionBytes); actual != expected {
		return rejection.New(rejection.DigestMismatch, "Axiom Evidence definition domain digest mismatch")
	}
	return nil
}
