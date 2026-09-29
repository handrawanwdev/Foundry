package work

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

// ExtractJSON decodes the first JSON object found in AI output. Agents often
// wrap JSON in prose or code fences.
func ExtractJSON(out string, v any) error {
	if i := strings.Index(out, "```json"); i >= 0 {
		rest := out[i+7:]
		if j := strings.Index(rest, "```"); j >= 0 {
			if json.Unmarshal([]byte(rest[:j]), v) == nil {
				return nil
			}
		}
	}
	b := []byte(out)
	for i := bytes.IndexByte(b, '{'); i >= 0; {
		dec := json.NewDecoder(bytes.NewReader(b[i:]))
		if err := dec.Decode(v); err == nil {
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
