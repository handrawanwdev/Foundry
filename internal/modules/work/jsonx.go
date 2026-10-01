package work

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
)

// ExtractJSON decodes the first JSON object found in AI output. Agents often
// wrap JSON in prose or code fences.
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
	if i := strings.Index(out, "```json"); i >= 0 {
		rest := out[i+7:]
		if j := strings.Index(rest, "```"); j >= 0 && try([]byte(rest[:j])) {
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
