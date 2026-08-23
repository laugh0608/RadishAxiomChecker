package strictjson

// Kind is the closed set accepted by the checker JSON profile.
type Kind uint8

const (
	Invalid Kind = iota
	Object
	Array
	String
	Boolean
)

// Member preserves the canonical member order from the input.
type Member struct {
	Name  string
	Value Value
}

// Value is an immutable-by-convention tagged union. Accessors return copies.
type Value struct {
	kind    Kind
	members []Member
	items   []Value
	text    string
	truth   bool
}

func (v Value) Kind() Kind { return v.kind }
func (v Value) Text() (string, bool) {
	return v.text, v.kind == String
}
func (v Value) Bool() (bool, bool) {
	return v.truth, v.kind == Boolean
}
func (v Value) Members() ([]Member, bool) {
	if v.kind != Object {
		return nil, false
	}
	return append([]Member(nil), v.members...), true
}
func (v Value) Items() ([]Value, bool) {
	if v.kind != Array {
		return nil, false
	}
	return append([]Value(nil), v.items...), true
}
