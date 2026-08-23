package protocol

import (
	"strconv"

	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

func object(value strictjson.Value, required []string) (map[string]strictjson.Value, error) {
	members, ok := value.Members()
	if !ok {
		return nil, rejection.New(rejection.InvalidJSON, "expected object")
	}
	allowed := make(map[string]struct{}, len(required))
	for _, name := range required {
		allowed[name] = struct{}{}
	}
	values := make(map[string]strictjson.Value, len(members))
	for _, member := range members {
		if _, ok := allowed[member.Name]; !ok {
			return nil, rejection.New(rejection.UnknownMember, "object contains an unknown member")
		}
		values[member.Name] = member.Value
	}
	for _, name := range required {
		if _, ok := values[name]; !ok {
			return nil, rejection.New(rejection.MissingRequiredMember, "object is missing a required member")
		}
	}
	return values, nil
}

func text(value strictjson.Value) (string, error) {
	result, ok := value.Text()
	if !ok {
		return "", rejection.New(rejection.InvalidJSON, "expected string")
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

func decimal(value strictjson.Value, allowZero bool) (uint64, error) {
	raw, err := text(value)
	if err != nil {
		return 0, err
	}
	if raw == "" || (len(raw) > 1 && raw[0] == '0') {
		return 0, rejection.New(rejection.InvalidJSON, "quantity is not a canonical decimal string")
	}
	for i := range raw {
		if raw[i] < '0' || raw[i] > '9' {
			return 0, rejection.New(rejection.InvalidJSON, "quantity is not a canonical decimal string")
		}
	}
	number, parseErr := strconv.ParseUint(raw, 10, 64)
	if parseErr != nil || (!allowZero && number == 0) {
		return 0, rejection.New(rejection.InvalidJSON, "quantity is outside the supported unsigned range")
	}
	return number, nil
}

func strings(value strictjson.Value) ([]string, error) {
	items, err := array(value)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		s, err := text(item)
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, nil
}

func strictlySorted(values []string) bool {
	for i := 1; i < len(values); i++ {
		if values[i-1] >= values[i] {
			return false
		}
	}
	return true
}
