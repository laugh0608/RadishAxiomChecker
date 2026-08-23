package strictjson

// CanonicalBytes encodes a value already accepted by ParseCanonical back to
// the checker's number-free, null-free JCS profile. Value does not expose
// constructors, so every reachable input has already passed the parser's
// Unicode, member-order, and closed-kind checks.
func CanonicalBytes(value Value) ([]byte, error) {
	if value.kind < Object || value.kind > Boolean {
		return nil, reject(CodeInvalidJSON, -1, "cannot encode an invalid strict JSON value")
	}
	result := make([]byte, 0, 128)
	return appendCanonical(result, value), nil
}

func appendCanonical(dst []byte, value Value) []byte {
	switch value.kind {
	case Object:
		dst = append(dst, '{')
		for i, member := range value.members {
			if i != 0 {
				dst = append(dst, ',')
			}
			dst = appendString(dst, member.Name)
			dst = append(dst, ':')
			dst = appendCanonical(dst, member.Value)
		}
		return append(dst, '}')
	case Array:
		dst = append(dst, '[')
		for i, item := range value.items {
			if i != 0 {
				dst = append(dst, ',')
			}
			dst = appendCanonical(dst, item)
		}
		return append(dst, ']')
	case String:
		return appendString(dst, value.text)
	case Boolean:
		if value.truth {
			return append(dst, "true"...)
		}
		return append(dst, "false"...)
	default:
		return dst
	}
}

func appendString(dst []byte, value string) []byte {
	const hex = "0123456789abcdef"
	dst = append(dst, '"')
	for i := 0; i < len(value); i++ {
		b := value[i]
		switch b {
		case '"', '\\':
			dst = append(dst, '\\', b)
		case '\b':
			dst = append(dst, '\\', 'b')
		case '\t':
			dst = append(dst, '\\', 't')
		case '\n':
			dst = append(dst, '\\', 'n')
		case '\f':
			dst = append(dst, '\\', 'f')
		case '\r':
			dst = append(dst, '\\', 'r')
		default:
			if b < 0x20 {
				dst = append(dst, '\\', 'u', '0', '0', hex[b>>4], hex[b&0x0f])
			} else {
				dst = append(dst, b)
			}
		}
	}
	return append(dst, '"')
}
