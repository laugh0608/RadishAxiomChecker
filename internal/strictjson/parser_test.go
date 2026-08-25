package strictjson

import (
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

var testLimits = Limits{
	MaxBytes: 1 << 20,
	MaxDepth: 128,
	MaxItems: 10_000,
	MaxSteps: 1_000_000,
}

func TestParseCanonicalAcceptsProfileValues(t *testing.T) {
	tests := [][]byte{
		[]byte(`{}`),
		[]byte(`[]`),
		[]byte(`true`),
		[]byte(`false`),
		[]byte(`"text"`),
		[]byte(`"\u0000\b\t\n\f\r\"\\"`),
		[]byte(`{"a":[],"b":{"c":"世界"}}`),
		[]byte("{\"𐀀\":false,\"\":true}"),
	}
	for _, data := range tests {
		if _, err := ParseCanonical(data, testLimits); err != nil {
			t.Fatalf("ParseCanonical(%q): %v", data, err)
		}
	}
}

func TestParseDocumentAcceptsPrettyStrictJSON(t *testing.T) {
	data := []byte(" { \n  \"z\" : \"\\u4e16\\u754c\",\n  \"a\": [true, false, \"\\/\"]\n}\n")
	value, err := ParseDocument(data, testLimits)
	if err != nil {
		t.Fatal(err)
	}
	members, ok := value.Members()
	if !ok || len(members) != 2 || members[0].Name != "z" || members[1].Name != "a" {
		t.Fatalf("unexpected relaxed document members: %+v", members)
	}
}

func TestParseDocumentRetainsStrictEnvelopeChecks(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		code rejection.Code
	}{
		{"duplicate member", []byte(`{"a": false, "a": true}`), rejection.DuplicateMember},
		{"number", []byte(`{"a": 1}`), rejection.JSONNumberOrNull},
		{"null", []byte(`{"a": null}`), rejection.JSONNumberOrNull},
		{"invalid UTF-8", []byte{'"', 0xff, '"'}, rejection.InvalidUTF8},
		{"BOM", []byte{0xef, 0xbb, 0xbf, '{', '}'}, rejection.NoncanonicalJSON},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseDocument(test.data, testLimits)
			assertCode(t, err, test.code)
		})
	}
}

func TestCanonicalBytesReplaysAcceptedInput(t *testing.T) {
	tests := [][]byte{
		[]byte(`{}`),
		[]byte(`[]`),
		[]byte(`true`),
		[]byte(`false`),
		[]byte(`"text"`),
		[]byte(`"\u0000\b\t\n\f\r\"\\"`),
		[]byte(`{"a":[],"b":{"c":"世界"}}`),
		[]byte("{\"𐀀\":false,\"\":true}"),
	}
	for _, data := range tests {
		value, err := ParseCanonical(data, testLimits)
		if err != nil {
			t.Fatalf("ParseCanonical(%q): %v", data, err)
		}
		got, err := CanonicalBytes(value)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(data) {
			t.Fatalf("CanonicalBytes() = %q, want %q", got, data)
		}
	}
}

func TestCanonicalBytesRejectsInvalidValue(t *testing.T) {
	if _, err := CanonicalBytes(Value{}); err == nil {
		t.Fatal("expected invalid zero value to be rejected")
	}
}

func TestParseCanonicalRejectsNoncanonicalOrForbiddenInput(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		code rejection.Code
	}{
		{"leading whitespace", []byte(` {}`), rejection.NoncanonicalJSON},
		{"trailing newline", []byte("{}\n"), rejection.NoncanonicalJSON},
		{"BOM", []byte{0xef, 0xbb, 0xbf, '{', '}'}, rejection.NoncanonicalJSON},
		{"duplicate member", []byte(`{"a":false,"a":true}`), rejection.DuplicateMember},
		{"member order", []byte(`{"b":false,"a":true}`), rejection.NoncanonicalOrder},
		{"UTF-16 member order", []byte("{\"\":true,\"𐀀\":false}"), rejection.NoncanonicalOrder},
		{"number", []byte(`0`), rejection.JSONNumberOrNull},
		{"null", []byte(`null`), rejection.JSONNumberOrNull},
		{"invalid UTF-8", []byte{'"', 0xff, '"'}, rejection.InvalidUTF8},
		{"escaped solidus", []byte(`"\/"`), rejection.NoncanonicalJSON},
		{"escaped printable", []byte(`"\u0061"`), rejection.NoncanonicalJSON},
		{"uppercase escape", []byte(`"\u000A"`), rejection.NoncanonicalJSON},
		{"long short escape", []byte(`"\u000a"`), rejection.NoncanonicalJSON},
		{"lone high surrogate", []byte(`"\ud800"`), rejection.InvalidUTF8},
		{"lone low surrogate", []byte(`"\udc00"`), rejection.InvalidUTF8},
		{"escaped surrogate pair", []byte(`"\ud800\udc00"`), rejection.NoncanonicalJSON},
		{"unknown literal", []byte(`tru`), rejection.InvalidJSON},
		{"trailing comma", []byte(`{"a":true,}`), rejection.InvalidJSON},
		{"raw control", []byte{'"', 0x01, '"'}, rejection.InvalidJSON},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseCanonical(test.data, testLimits)
			assertCode(t, err, test.code)
		})
	}
}

func TestParseCanonicalEnforcesBudgets(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		limits Limits
	}{
		{"bytes", []byte(`{"a":true}`), Limits{MaxBytes: 2, MaxDepth: 128, MaxItems: 100, MaxSteps: 100}},
		{"depth", []byte(`[[true]]`), Limits{MaxBytes: 100, MaxDepth: 1, MaxItems: 100, MaxSteps: 100}},
		{"items", []byte(`[true,false]`), Limits{MaxBytes: 100, MaxDepth: 10, MaxItems: 1, MaxSteps: 100}},
		{"steps", []byte(`"abcdef"`), Limits{MaxBytes: 100, MaxDepth: 10, MaxItems: 100, MaxSteps: 0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseCanonical(test.data, test.limits)
			assertCode(t, err, rejection.ResourceLimit)
		})
	}
}

func assertCode(t *testing.T, err error, want rejection.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s, got nil", want)
	}
	got, ok := rejection.CodeOf(err)
	if !ok || got != want {
		t.Fatalf("expected %s, got %v", want, err)
	}
}
