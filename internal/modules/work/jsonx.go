package work

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"unicode"
)

// ExtractJSON decodes the first JSON object found in AI output. Agents often
// wrap JSON in prose or code fences. A truncated object is closed and retried
// so a cut-off answer still lands.
func ExtractJSON(out string, v any) error {
	return ExtractJSONFunc(out, v, nil)
}

// ExtractJSONFunc is ExtractJSON, but skips objects until match returns true.
// match nil accepts the first object that unmarshals.
func ExtractJSONFunc(out string, v any, match func() bool) error {
	ptr := reflect.ValueOf(v)
	if ptr.Kind() != reflect.Pointer || ptr.IsNil() {
		return errors.New("output AI tidak berisi JSON yang valid")
	}
	zero := reflect.Zero(ptr.Elem().Type())
	try := func(raw []byte) bool {
		ptr.Elem().Set(zero)
		if json.Unmarshal(raw, v) != nil {
			return false
		}
		return match == nil || match()
	}
	for _, fence := range fenceBodies(out) {
		if try([]byte(fence)) {
			return nil
		}
	}
	b := []byte(out)
	for i := bytes.IndexByte(b, '{'); i >= 0; {
		var raw json.RawMessage
		dec := json.NewDecoder(bytes.NewReader(b[i:]))
		if err := dec.Decode(&raw); err == nil && try(raw) {
			return nil
		}
		next := bytes.IndexByte(b[i+1:], '{')
		if next < 0 {
			break
		}
		i += next + 1
	}
	return errors.New("output AI tidak berisi JSON yang valid")
}

func fenceBodies(out string) []string {
	var bodies []string
	s := out
	for {
		i := strings.Index(s, "```")
		if i < 0 {
			break
		}
		s = s[i+3:]
		if len(s) >= 4 && strings.EqualFold(s[:4], "json") {
			s = s[4:]
		}
		s = strings.TrimLeft(s, "\r\n ")
		j := strings.Index(s, "```")
		body := s
		if j >= 0 {
			body = s[:j]
			s = s[j+3:]
		}
		if strings.TrimSpace(body) != "" {
			bodies = append(bodies, body)
		}
		if j < 0 {
			break
		}
	}
	return bodies
}

// closeJSON appends missing quotes, brackets and braces so a truncated
// object can be unmarshaled. Incomplete keys or values may still fail.
func closeJSON(s string) string {
	s = strings.TrimRightFunc(s, unicode.IsSpace)
	if s == "" {
		return s
	}
	var stack []byte
	inStr := false
	esc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{', '[':
			stack = append(stack, c)
		case '}':
			if n := len(stack); n > 0 && stack[n-1] == '{' {
				stack = stack[:n-1]
			}
		case ']':
			if n := len(stack); n > 0 && stack[n-1] == '[' {
				stack = stack[:n-1]
			}
		}
	}
	if inStr {
		if esc {
			s += `\`
		}
		s += `"`
	}
	// Drop a trailing comma so `{"a":1,` becomes `{"a":1}`.
	trim := strings.TrimRightFunc(s, unicode.IsSpace)
	if strings.HasSuffix(trim, ",") {
		s = strings.TrimSuffix(trim, ",")
	}
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == '{' {
			s += "}"
		} else {
			s += "]"
		}
	}
	return s
}
