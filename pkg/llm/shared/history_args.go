package shared

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode"
)

// ParseActionArgs turns a recorded activity's argument one-liner back
// into the object a replayed tool call carries. The digest writes
// `key="value"` for strings (Go %q) and `key=value` for everything else,
// in sorted key order; this is the inverse. Numbers, booleans and null
// come back typed, so a replayed call matches the tool's schema. A line
// that does not parse, or one the digest truncated (it ends in the
// truncation marker), is kept whole under "args" so the model still sees
// what was passed.
func ParseActionArgs(line string) map[string]any {
	line = strings.TrimSpace(line)
	if line == "" {
		return map[string]any{}
	}
	if strings.HasSuffix(line, truncationMarker) {
		return map[string]any{"args": line}
	}
	args, ok := parseActionPairs(line)
	if !ok {
		return map[string]any{"args": line}
	}
	return args
}

// ActionArguments is ParseActionArgs encoded as the compact JSON object
// the OpenAI-style wire formats take for tool call arguments.
func ActionArguments(line string) json.RawMessage {
	encoded, err := json.Marshal(ParseActionArgs(line))
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}

// truncationMarker is what the digest appends when it cuts a field.
const truncationMarker = "…"

func parseActionPairs(line string) (map[string]any, bool) {
	args := map[string]any{}
	rest := line
	for rest != "" {
		eq := strings.IndexByte(rest, '=')
		if eq <= 0 || !isIdentifier(rest[:eq]) {
			return nil, false
		}
		key := rest[:eq]
		rest = rest[eq+1:]
		if rest == "" {
			return nil, false
		}
		var value any
		if rest[0] == '"' {
			end := closingQuote(rest)
			if end < 0 {
				return nil, false
			}
			unquoted, err := strconv.Unquote(rest[:end+1])
			if err != nil {
				return nil, false
			}
			value = unquoted
			rest = rest[end+1:]
		} else {
			// Non-string values print with %v and may contain spaces
			// ([a b], map[k:v]); the value runs until the next ` key=`.
			// One holding its own '=' ([FOO=bar]) cannot be split
			// reliably, so the whole line falls back to raw.
			end := nextPairStart(rest)
			bare := strings.TrimSpace(rest[:end])
			if strings.Contains(bare, "=") {
				return nil, false
			}
			value = scalarValue(bare)
			rest = rest[end:]
		}
		args[key] = value
		if rest != "" {
			if rest[0] != ' ' {
				return nil, false
			}
			rest = rest[1:]
		}
	}
	return args, true
}

// scalarValue types a bare %v token: JSON numbers, true and false decode,
// %v's <nil> is null; anything else ([a b], map[k:v]) stays a string.
func scalarValue(token string) any {
	if token == "<nil>" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(token), &v); err == nil {
		switch v.(type) {
		case float64, bool, nil:
			return v
		}
	}
	return token
}

// closingQuote returns the index of the quote that closes the Go-quoted
// string starting at s[0], or -1.
func closingQuote(s string) int {
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '"':
			return i
		}
	}
	return -1
}

// nextPairStart returns where the next ` key=` begins in s, or len(s).
func nextPairStart(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' {
			continue
		}
		rest := s[i+1:]
		eq := strings.IndexByte(rest, '=')
		if eq > 0 && isIdentifier(rest[:eq]) {
			return i
		}
	}
	return len(s)
}

func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' && r != '.' {
			return false
		}
	}
	return true
}
