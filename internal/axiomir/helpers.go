package axiomir

import (
	"bytes"
	"crypto/sha256"
	"math/big"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

const (
	domainContract   = "axiom-ir-v0.1:contract"
	domainDocument   = "axiom-ir-v0.1:document"
	domainEnumType   = "axiom-ir-v0.1:enum-type"
	domainNode       = "axiom-ir-v0.1:node"
	domainRecordType = "axiom-ir-v0.1:record-type"
	domainTableType  = "axiom-ir-v0.1:table-type"
)

func object(value strictjson.Value, required ...string) (map[string]strictjson.Value, error) {
	members, ok := value.Members()
	if !ok {
		return nil, rejection.New(rejection.InvalidJSON, "expected object")
	}
	allowed := make(map[string]struct{}, len(required))
	for _, name := range required {
		allowed[name] = struct{}{}
	}
	fields := make(map[string]strictjson.Value, len(members))
	for _, member := range members {
		if _, ok := allowed[member.Name]; !ok {
			return nil, rejection.New(rejection.UnknownMember, "Axiom IR object contains an unknown member")
		}
		fields[member.Name] = member.Value
	}
	for _, name := range required {
		if _, ok := fields[name]; !ok {
			return nil, rejection.New(rejection.MissingRequiredMember, "Axiom IR object is missing a required member")
		}
	}
	return fields, nil
}

func member(value strictjson.Value, name string) (strictjson.Value, error) {
	members, ok := value.Members()
	if !ok {
		return strictjson.Value{}, rejection.New(rejection.InvalidJSON, "expected object")
	}
	for _, candidate := range members {
		if candidate.Name == name {
			return candidate.Value, nil
		}
	}
	return strictjson.Value{}, rejection.New(rejection.MissingRequiredMember, "tagged Axiom IR object is missing its tag")
}

func text(value strictjson.Value) (string, error) {
	result, ok := value.Text()
	if !ok {
		return "", rejection.New(rejection.InvalidJSON, "expected string")
	}
	return result, nil
}

func boolean(value strictjson.Value) (bool, error) {
	result, ok := value.Bool()
	if !ok {
		return false, rejection.New(rejection.InvalidJSON, "expected boolean")
	}
	return result, nil
}

func array(value strictjson.Value) ([]strictjson.Value, error) {
	result, ok := value.Items()
	if !ok {
		return nil, rejection.New(rejection.InvalidJSON, "expected array")
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

func name(value strictjson.Value) (string, error) {
	result, err := text(value)
	if err != nil {
		return "", err
	}
	if result == "" {
		return "", rejection.New(rejection.InvalidJSON, "Axiom IR name must not be empty")
	}
	for _, r := range result {
		if r <= 0x1f || 0x7f <= r && r <= 0x9f {
			return "", rejection.New(rejection.InvalidJSON, "Axiom IR name contains a control character")
		}
	}
	return result, nil
}

func canonicalInteger(value strictjson.Value, nonnegative bool) (*big.Int, error) {
	raw, err := text(value)
	if err != nil {
		return nil, err
	}
	if raw == "" || raw == "-0" || raw[0] == '+' || len(raw) > 1 && raw[0] == '0' {
		return nil, rejection.New(rejection.InvalidJSON, "integer is not a canonical decimal string")
	}
	start := 0
	if raw[0] == '-' {
		if nonnegative || len(raw) == 1 || raw[1] == '0' {
			return nil, rejection.New(rejection.InvalidJSON, "integer is not a canonical decimal string")
		}
		start = 1
	}
	for i := start; i < len(raw); i++ {
		if raw[i] < '0' || raw[i] > '9' {
			return nil, rejection.New(rejection.InvalidJSON, "integer is not a canonical decimal string")
		}
	}
	result, ok := new(big.Int).SetString(raw, 10)
	if !ok {
		return nil, rejection.New(rejection.InvalidJSON, "integer cannot be decoded")
	}
	return result, nil
}

func requireStrictOrder(previous, current string, present bool, detail string) error {
	if present && previous >= current {
		return rejection.New(rejection.NoncanonicalOrder, detail)
	}
	return nil
}

func requireCanonicalOrder(values []strictjson.Value, strict bool, detail string) error {
	for i := 1; i < len(values); i++ {
		left, err := strictjson.CanonicalBytes(values[i-1])
		if err != nil {
			return err
		}
		right, err := strictjson.CanonicalBytes(values[i])
		if err != nil {
			return err
		}
		comparison := bytes.Compare(left, right)
		if comparison > 0 || strict && comparison == 0 {
			return rejection.New(rejection.NoncanonicalOrder, detail)
		}
	}
	return nil
}
