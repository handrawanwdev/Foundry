package work

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
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
	// typeErr is the first object that is valid JSON but does not fit the
	// schema; it is reported when nothing else matches.
	var typeErr error
	// decode reports whether raw fits the schema, strictly or after the
	// value types models commonly get wrong are coerced.
	decode := func(raw []byte) bool {
		ptr.Elem().Set(zero)
		err := json.Unmarshal(raw, v)
		if err == nil {
			return true
		}
		var te *json.UnmarshalTypeError
		if !errors.As(err, &te) {
			return false
		}
		if fixed, ok := lenientJSON(raw, ptr.Elem().Type()); ok {
			ptr.Elem().Set(zero)
			if json.Unmarshal(fixed, v) == nil {
				return true
			}
		}
		if typeErr == nil {
			typeErr = err
		}
		return false
	}
	try := func(raw []byte) bool {
		return decode(raw) && (match == nil || match())
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
		err := dec.Decode(&raw)
		if errors.Is(err, io.ErrUnexpectedEOF) {
			// The rest of the output is one cut-off object: the objects
			// inside it are fragments of the answer, never the answer.
			break
		}
		next := i + 1
		if err == nil {
			if decode(raw) {
				if match == nil || match() {
					return nil
				}
			} else {
				// Valid JSON that does not fit the schema: its inner objects
				// are parts of that answer (a compatibility entry with a
				// "note"), so they must not be taken for the answer.
				next = i + int(dec.InputOffset())
			}
		}
		j := bytes.IndexByte(b[next:], '{')
		if j < 0 {
			break
		}
		i = next + j
	}
	if i := bytes.IndexByte(b, '{'); i >= 0 && trySrc(string(b[i:])) {
		return nil
	}
	if typeErr != nil {
		return fmt.Errorf("output AI tidak sesuai format yang diminta: %v", typeErr)
	}
	return DiagnoseJSON(out)
}

// lenientJSON rewrites the values whose JSON type does not fit t the way
// models commonly get them wrong: a number or bool as a string or the
// reverse, a fraction for an integer, a list of strings for one string, one
// value for a list. Values it cannot convert are left for Unmarshal to refuse.
func lenientJSON(raw []byte, t reflect.Type) ([]byte, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var val any
	if dec.Decode(&val) != nil {
		return nil, false
	}
	out, err := json.Marshal(coerceValue(val, t))
	return out, err == nil
}

var unmarshalerType = reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()

func coerceValue(val any, t reflect.Type) any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	// A type with its own decoder decides what it tolerates; reshaping its
	// value here (one object wrapped as a list) would hide the form it reads.
	if val == nil || t == reflect.TypeOf(json.RawMessage(nil)) || reflect.PointerTo(t).Implements(unmarshalerType) {
		return val
	}
	switch t.Kind() {
	case reflect.Struct:
		m, ok := val.(map[string]any)
		if !ok {
			return val
		}
		coerceFields(m, t)
		return m
	case reflect.Map:
		m, ok := val.(map[string]any)
		if !ok {
			return val
		}
		for k, x := range m {
			m[k] = coerceValue(x, t.Elem())
		}
		return m
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return val
		}
		list, ok := val.([]any)
		if !ok {
			if s, isStr := val.(string); isStr && strings.TrimSpace(s) == "" {
				return []any{}
			}
			list = []any{val}
		}
		for i, x := range list {
			list[i] = coerceValue(x, t.Elem())
		}
		return list
	case reflect.String:
		switch x := val.(type) {
		case json.Number:
			return x.String()
		case bool:
			return strconv.FormatBool(x)
		case []any:
			parts := make([]string, 0, len(x))
			for _, e := range x {
				s, ok := e.(string)
				if !ok {
					return val
				}
				if s = strings.TrimSpace(s); s != "" {
					parts = append(parts, s)
				}
			}
			return strings.Join(parts, "; ")
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var f float64
		var err error
		switch x := val.(type) {
		case json.Number:
			f, err = x.Float64()
		case string:
			f, err = strconv.ParseFloat(strings.TrimSpace(x), 64)
		default:
			return val
		}
		if err != nil {
			return val
		}
		return json.Number(strconv.FormatInt(int64(math.Round(f)), 10))
	case reflect.Float32, reflect.Float64:
		if s, ok := val.(string); ok {
			if _, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
				return json.Number(strings.TrimSpace(s))
			}
		}
	case reflect.Bool:
		if s, ok := val.(string); ok {
			if b, err := strconv.ParseBool(strings.TrimSpace(s)); err == nil {
				return b
			}
		}
	}
	return val
}

// coerceFields coerces the members of m that name a field of struct t,
// matching keys the way encoding/json does (exact first, then any case).
func coerceFields(m map[string]any, t reflect.Type) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if f.Anonymous && tag == "" {
			ft := f.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				coerceFields(m, ft)
				continue
			}
		}
		if f.PkgPath != "" || name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		key := name
		if _, ok := m[key]; !ok {
			key = ""
			for k := range m {
				if strings.EqualFold(k, name) {
					key = k
					break
				}
			}
			if key == "" {
				continue
			}
		}
		m[key] = coerceValue(m[key], f.Type)
	}
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
	if t := reflect.TypeOf(v); t != nil && t.Kind() == reflect.Pointer {
		if fixed, ok := lenientJSON(raw, t.Elem()); ok && json.Unmarshal(fixed, v) == nil {
			return nil
		}
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
