package strictjson

import (
	"unicode/utf16"
	"unicode/utf8"
)

// Limits bounds the strict envelope before any domain values are constructed.
type Limits struct {
	MaxBytes uint64
	MaxDepth uint64
	MaxItems uint64
	MaxSteps uint64
}

type parser struct {
	data      []byte
	pos       int
	limits    Limits
	items     uint64
	steps     uint64
	canonical bool
}

// ParseCanonical accepts only the number-free, null-free I-JSON/JCS profile.
func ParseCanonical(data []byte, limits Limits) (Value, error) {
	return parse(data, limits, true)
}

// ParseDocument accepts strict number-free, null-free JSON while treating
// insignificant whitespace and object-member order as representation details.
// It retains duplicate-member, UTF-8, depth, item, step, and byte checks. This
// is the input envelope used by pretty JSON artifacts whose abstract value is
// interpreted by a separate closed domain parser.
func ParseDocument(data []byte, limits Limits) (Value, error) {
	return parse(data, limits, false)
}

func parse(data []byte, limits Limits, canonical bool) (Value, error) {
	if uint64(len(data)) > limits.MaxBytes {
		return Value{}, reject(CodeResourceLimit, 0, "input exceeds byte limit")
	}
	if !utf8.Valid(data) {
		return Value{}, reject(CodeInvalidUTF8, 0, "input is not valid UTF-8")
	}
	if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		return Value{}, reject(CodeNoncanonicalJSON, 0, "BOM is forbidden")
	}
	p := parser{data: data, limits: limits, canonical: canonical}
	if err := p.skipWhitespace(); err != nil {
		return Value{}, err
	}
	v, err := p.value(0)
	if err != nil {
		return Value{}, err
	}
	if err := p.skipWhitespace(); err != nil {
		return Value{}, err
	}
	if p.pos != len(data) {
		if canonical {
			return Value{}, reject(CodeNoncanonicalJSON, p.pos, "trailing bytes or whitespace")
		}
		return Value{}, reject(CodeInvalidJSON, p.pos, "trailing bytes")
	}
	return v, nil
}

func (p *parser) value(openContainers uint64) (Value, error) {
	if p.pos >= len(p.data) {
		return Value{}, reject(CodeInvalidJSON, p.pos, "expected value")
	}
	switch p.data[p.pos] {
	case '{':
		if openContainers == p.limits.MaxDepth {
			return Value{}, reject(CodeResourceLimit, p.pos, "JSON depth limit exceeded")
		}
		return p.object(openContainers + 1)
	case '[':
		if openContainers == p.limits.MaxDepth {
			return Value{}, reject(CodeResourceLimit, p.pos, "JSON depth limit exceeded")
		}
		return p.array(openContainers + 1)
	case '"':
		if err := p.step(); err != nil {
			return Value{}, err
		}
		s, err := p.string()
		return Value{kind: String, text: s}, err
	case 't':
		if err := p.step(); err != nil {
			return Value{}, err
		}
		if err := p.literal("true"); err != nil {
			return Value{}, err
		}
		return Value{kind: Boolean, truth: true}, nil
	case 'f':
		if err := p.step(); err != nil {
			return Value{}, err
		}
		if err := p.literal("false"); err != nil {
			return Value{}, err
		}
		return Value{kind: Boolean, truth: false}, nil
	case 'n', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return Value{}, reject(CodeJSONNumberOrNull, p.pos, "numbers and null are forbidden")
	case ' ', '\t', '\n', '\r':
		return Value{}, reject(CodeNoncanonicalJSON, p.pos, "whitespace is forbidden")
	default:
		return Value{}, reject(CodeInvalidJSON, p.pos, "unexpected token")
	}
}

func (p *parser) object(openContainers uint64) (Value, error) {
	if err := p.step(); err != nil {
		return Value{}, err
	}
	p.pos++
	if err := p.skipWhitespace(); err != nil {
		return Value{}, err
	}
	if p.consume('}') {
		if err := p.step(); err != nil {
			return Value{}, err
		}
		return Value{kind: Object}, nil
	}
	members := make([]Member, 0, 8)
	seen := make(map[string]struct{})
	var previous string
	for {
		if p.pos >= len(p.data) || p.data[p.pos] != '"' {
			return Value{}, reject(CodeInvalidJSON, p.pos, "object member name must be a string")
		}
		nameOffset := p.pos
		if err := p.step(); err != nil {
			return Value{}, err
		}
		name, err := p.string()
		if err != nil {
			return Value{}, err
		}
		if _, ok := seen[name]; ok {
			return Value{}, reject(CodeDuplicateMember, nameOffset, "duplicate object member")
		}
		if p.canonical && len(members) != 0 && compareUTF16(previous, name) >= 0 {
			return Value{}, reject(CodeNoncanonicalOrder, nameOffset, "object members are not in JCS order")
		}
		seen[name] = struct{}{}
		previous = name
		if err := p.skipWhitespace(); err != nil {
			return Value{}, err
		}
		if !p.consume(':') {
			return Value{}, reject(CodeInvalidJSON, p.pos, "expected colon")
		}
		if err := p.step(); err != nil {
			return Value{}, err
		}
		if err := p.skipWhitespace(); err != nil {
			return Value{}, err
		}
		v, err := p.value(openContainers)
		if err != nil {
			return Value{}, err
		}
		if err := p.addItem(); err != nil {
			return Value{}, err
		}
		members = append(members, Member{Name: name, Value: v})
		if err := p.skipWhitespace(); err != nil {
			return Value{}, err
		}
		if p.consume('}') {
			if err := p.step(); err != nil {
				return Value{}, err
			}
			break
		}
		if !p.consume(',') {
			return Value{}, reject(CodeInvalidJSON, p.pos, "expected comma or closing brace")
		}
		if err := p.step(); err != nil {
			return Value{}, err
		}
		if err := p.skipWhitespace(); err != nil {
			return Value{}, err
		}
	}
	return Value{kind: Object, members: members}, nil
}

func (p *parser) array(openContainers uint64) (Value, error) {
	if err := p.step(); err != nil {
		return Value{}, err
	}
	p.pos++
	if err := p.skipWhitespace(); err != nil {
		return Value{}, err
	}
	if p.consume(']') {
		if err := p.step(); err != nil {
			return Value{}, err
		}
		return Value{kind: Array}, nil
	}
	items := make([]Value, 0, 8)
	for {
		v, err := p.value(openContainers)
		if err != nil {
			return Value{}, err
		}
		if err := p.addItem(); err != nil {
			return Value{}, err
		}
		items = append(items, v)
		if err := p.skipWhitespace(); err != nil {
			return Value{}, err
		}
		if p.consume(']') {
			if err := p.step(); err != nil {
				return Value{}, err
			}
			break
		}
		if !p.consume(',') {
			return Value{}, reject(CodeInvalidJSON, p.pos, "expected comma or closing bracket")
		}
		if err := p.step(); err != nil {
			return Value{}, err
		}
		if err := p.skipWhitespace(); err != nil {
			return Value{}, err
		}
	}
	return Value{kind: Array, items: items}, nil
}

func (p *parser) string() (string, error) {
	p.pos++
	out := make([]byte, 0, 32)
	for p.pos < len(p.data) {
		b := p.data[p.pos]
		switch b {
		case '"':
			p.pos++
			return string(out), nil
		case '\\':
			decoded, err := p.escape()
			if err != nil {
				return "", err
			}
			out = append(out, decoded...)
		default:
			if b < 0x20 {
				return "", reject(CodeInvalidJSON, p.pos, "unescaped control character")
			}
			_, size := utf8.DecodeRune(p.data[p.pos:])
			out = append(out, p.data[p.pos:p.pos+size]...)
			p.pos += size
		}
	}
	return "", reject(CodeInvalidJSON, p.pos, "unterminated string")
}

func (p *parser) escape() ([]byte, error) {
	offset := p.pos
	p.pos++
	if p.pos >= len(p.data) {
		return nil, reject(CodeInvalidJSON, p.pos, "unterminated escape")
	}
	b := p.data[p.pos]
	p.pos++
	switch b {
	case '"', '\\':
		return []byte{b}, nil
	case 'b':
		return []byte{0x08}, nil
	case 't':
		return []byte{0x09}, nil
	case 'n':
		return []byte{0x0a}, nil
	case 'f':
		return []byte{0x0c}, nil
	case 'r':
		return []byte{0x0d}, nil
	case '/':
		if p.canonical {
			return nil, reject(CodeNoncanonicalJSON, offset, "solidus must not be escaped")
		}
		return []byte{'/'}, nil
	case 'u':
		unit, err := p.hexUnit()
		if err != nil {
			return nil, err
		}
		if 0xd800 <= unit && unit <= 0xdbff {
			if p.pos+6 > len(p.data) || p.data[p.pos] != '\\' || p.data[p.pos+1] != 'u' {
				return nil, reject(CodeInvalidUTF8, offset, "unpaired high surrogate")
			}
			p.pos += 2
			low, err := p.hexUnit()
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return nil, reject(CodeInvalidUTF8, offset, "unpaired high surrogate")
			}
			if p.canonical {
				return nil, reject(CodeNoncanonicalJSON, offset, "non-control characters must use UTF-8")
			}
			r := utf16.DecodeRune(rune(unit), rune(low))
			buffer := make([]byte, utf8.RuneLen(r))
			utf8.EncodeRune(buffer, r)
			return buffer, nil
		}
		if 0xdc00 <= unit && unit <= 0xdfff {
			return nil, reject(CodeInvalidUTF8, offset, "unpaired low surrogate")
		}
		if unit >= 0x20 && p.canonical {
			return nil, reject(CodeNoncanonicalJSON, offset, "non-control characters must use UTF-8")
		}
		if p.canonical && (unit == 0x08 || unit == 0x09 || unit == 0x0a || unit == 0x0c || unit == 0x0d) {
			return nil, reject(CodeNoncanonicalJSON, offset, "control character has a short escape")
		}
		if unit < 0x80 {
			return []byte{byte(unit)}, nil
		}
		buffer := make([]byte, utf8.RuneLen(rune(unit)))
		utf8.EncodeRune(buffer, rune(unit))
		return buffer, nil
	default:
		return nil, reject(CodeInvalidJSON, offset, "unknown escape")
	}
}

func (p *parser) hexUnit() (uint16, error) {
	if p.pos+4 > len(p.data) {
		return 0, reject(CodeInvalidJSON, p.pos, "short Unicode escape")
	}
	var value uint16
	for i := 0; i < 4; i++ {
		b := p.data[p.pos+i]
		var nibble byte
		switch {
		case '0' <= b && b <= '9':
			nibble = b - '0'
		case 'a' <= b && b <= 'f':
			nibble = b - 'a' + 10
		case 'A' <= b && b <= 'F':
			if p.canonical {
				return 0, reject(CodeNoncanonicalJSON, p.pos+i, "Unicode escape hex must be lowercase")
			}
			nibble = b - 'A' + 10
		default:
			return 0, reject(CodeInvalidJSON, p.pos+i, "invalid Unicode escape")
		}
		value = value<<4 | uint16(nibble)
	}
	p.pos += 4
	return value, nil
}

func (p *parser) literal(want string) error {
	if len(p.data)-p.pos < len(want) || string(p.data[p.pos:p.pos+len(want)]) != want {
		return reject(CodeInvalidJSON, p.pos, "invalid literal")
	}
	p.pos += len(want)
	return nil
}

func (p *parser) consume(b byte) bool {
	if p.pos < len(p.data) && p.data[p.pos] == b {
		p.pos++
		return true
	}
	return false
}

func (p *parser) addItem() error {
	p.items++
	if p.items > p.limits.MaxItems {
		return reject(CodeResourceLimit, p.pos, "collection item limit exceeded")
	}
	return nil
}

func (p *parser) step() error {
	p.steps++
	if p.steps > p.limits.MaxSteps {
		return reject(CodeResourceLimit, p.pos, "parser step limit exceeded")
	}
	return nil
}

func (p *parser) skipWhitespace() error {
	if p.canonical {
		return nil
	}
	for p.pos < len(p.data) {
		switch p.data[p.pos] {
		case ' ', '\t', '\n', '\r':
			if err := p.step(); err != nil {
				return err
			}
			p.pos++
		default:
			return nil
		}
	}
	return nil
}

func compareUTF16(a, b string) int {
	aa := utf16.Encode([]rune(a))
	bb := utf16.Encode([]rune(b))
	for i := 0; i < len(aa) && i < len(bb); i++ {
		if aa[i] < bb[i] {
			return -1
		}
		if aa[i] > bb[i] {
			return 1
		}
	}
	if len(aa) < len(bb) {
		return -1
	}
	if len(aa) > len(bb) {
		return 1
	}
	return 0
}
