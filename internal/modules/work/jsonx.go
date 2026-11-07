package work

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
		return DiagnoseJSON(out)
	}
	zero := reflect.Zero(ptr.Elem().Type())
	try := func(raw []byte) bool {
		ptr.Elem().Set(zero)
		if json.Unmarshal(raw, v) != nil {
			return false
		}
		return match == nil || match()
	}
	trySrc := func(src string) bool {
		if try([]byte(src)) {
			return true
		}
		if fixed := fixJSONSyntax(src); fixed != src && try([]byte(fixed)) {
			return true
		}
		return false
	}
	for _, fence := range fenceBodies(out) {
		if trySrc(fence) {
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
	if i := bytes.IndexByte(b, '{'); i >= 0 && trySrc(string(b[i:])) {
		return nil
	}
	return DiagnoseJSON(out)
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

// fixJSONSyntax repairs common LLM JSON mistakes (extra characters, trailing
// commas, a } that should be ]) without closing truncated input. Truncation
// stays with closeJSON so a cut-off object is not silently accepted as empty.
func fixJSONSyntax(s string) string {
	cur := s
	for n := 0; n < 24; n++ {
		if json.Valid([]byte(cur)) {
			return cur
		}
		var dummy any
		err := json.Unmarshal([]byte(cur), &dummy)
		se, ok := err.(*json.SyntaxError)
		if !ok || se == nil {
			return cur
		}
		if strings.Contains(se.Error(), "unexpected end of JSON") {
			return cur
		}
		next := fixJSONAt(cur, int(se.Offset), se.Error())
		if next == cur {
			return cur
		}
		cur = next
	}
	return cur
}

func fixJSONAt(s string, offset int, msg string) string {
	i := offset - 1
	if i < 0 {
		i = 0
	}
	if i >= len(s) {
		return s
	}
	c := s[i]
	prev, pi := lastNonSpaceByte(s, i-1)
	if (c == '}' || c == ']') && prev == ',' && pi >= 0 {
		return s[:pi] + s[i:]
	}
	if c == ',' && strings.Contains(msg, "looking for beginning") {
		return s[:i] + s[i+1:]
	}
	switch c {
	case ')':
		return s[:i] + s[i+1:]
	case '"':
		if strings.Contains(msg, "after object key:value pair") || strings.Contains(msg, "after array element") {
			return s[:i] + s[i+1:]
		}
	case '}':
		if strings.Contains(msg, "after array element") {
			return s[:i] + "]" + s[i+1:]
		}
	case ']':
		if strings.Contains(msg, "after object key") {
			return s[:i] + "}" + s[i+1:]
		}
	}
	return s
}

func lastNonSpaceByte(s string, i int) (byte, int) {
	for i >= 0 {
		if s[i] > ' ' {
			return s[i], i
		}
		i--
	}
	return 0, -1
}

// DiagnoseJSON explains why AI output is not usable JSON.
func DiagnoseJSON(out string) error {
	s := strings.TrimSpace(out)
	if s == "" {
		return errors.New("output AI kosong")
	}
	body := s
	if fences := fenceBodies(s); len(fences) > 0 {
		body = fences[0]
	} else if i := strings.IndexByte(s, '{'); i >= 0 {
		body = s[i:]
	} else {
		return errors.New("output AI tidak berisi JSON yang valid (tidak ada objek JSON)")
	}
	var dummy any
	err := json.Unmarshal([]byte(body), &dummy)
	if err == nil {
		return errors.New("output AI tidak berisi JSON yang valid (objek tidak berisi bagian yang diminta)")
	}
	se, ok := err.(*json.SyntaxError)
	if !ok {
		return fmt.Errorf("output AI tidak berisi JSON yang valid (%v)", err)
	}
	line, col := jsonLineCol(body, int(se.Offset))
	detail := "sintaks rusak"
	if strings.Contains(se.Error(), "unexpected end") {
		detail = "JSON terpotong sebelum selesai"
	}
	return fmt.Errorf("output AI tidak berisi JSON yang valid (%s di baris %d kolom %d: %s; cuplikan: %s)",
		detail, line, col, se.Error(), jsonSnippet(body, int(se.Offset), 60))
}

func jsonLineCol(s string, offset int) (line, col int) {
	i := offset - 1
	if i < 0 {
		i = 0
	}
	if i > len(s) {
		i = len(s)
	}
	line, col = 1, 1
	for _, c := range []byte(s[:i]) {
		if c == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

// unmarshalLoose fills v field-by-field so one mistyped nested value
// (a string where the schema wants an object) does not drop the rest.
func unmarshalLoose(raw []byte, v any) error {
	if json.Unmarshal(raw, v) == nil {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		if fixed := fixJSONSyntax(string(raw)); fixed != string(raw) {
			if json.Unmarshal([]byte(fixed), v) == nil {
				return nil
			}
			if err2 := json.Unmarshal([]byte(fixed), &m); err2 != nil {
				return err
			}
		} else {
			return err
		}
	}
	ptr := reflect.ValueOf(v)
	if ptr.Kind() != reflect.Pointer || ptr.IsNil() {
		return errors.New("output AI tidak berisi JSON yang valid")
	}
	el := ptr.Elem()
	if el.Kind() != reflect.Struct {
		return errors.New("output AI tidak berisi JSON yang valid")
	}
	rt := el.Type()
	filled := 0
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.PkgPath != "" {
			continue
		}
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		b, ok := m[name]
		if !ok || len(bytes.TrimSpace(b)) == 0 {
			continue
		}
		fv := el.Field(i)
		if !fv.CanSet() {
			continue
		}
		if json.Unmarshal(b, fv.Addr().Interface()) == nil {
			filled++
		}
	}
	if filled == 0 {
		return DiagnoseJSON(string(raw))
	}
	return nil
}

func jsonObjectBytes(raw string) []byte {
	for _, fence := range fenceBodies(raw) {
		if i := strings.IndexByte(fence, '{'); i >= 0 {
			src := fence[i:]
			if fixed := fixJSONSyntax(src); json.Valid([]byte(fixed)) {
				return []byte(fixed)
			}
			return []byte(src)
		}
	}
	if i := strings.IndexByte(raw, '{'); i >= 0 {
		src := raw[i:]
		if fixed := fixJSONSyntax(src); json.Valid([]byte(fixed)) {
			return []byte(fixed)
		}
		return []byte(src)
	}
	return nil
}

func jsonSnippet(s string, offset, n int) string {
	i := offset - 1
	if i < 0 {
		i = 0
	}
	if i > len(s) {
		i = len(s)
	}
	a := i - n
	if a < 0 {
		a = 0
	}
	b := i + n
	if b > len(s) {
		b = len(s)
	}
	return strings.ReplaceAll(s[a:b], "\n", "\\n")
}
