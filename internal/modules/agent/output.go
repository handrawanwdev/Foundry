package agent

import "encoding/json"

// text turns raw stdout into the answer. Output the adapter cannot read is
// returned unchanged, so an unexpected format never loses the agent's answer.
func (r *Runner) text(raw string) string {
	if r.unwrap != nil {
		if text, ok := r.unwrap(raw); ok {
			return text
		}
	}
	return raw
}

// claudeResult reads `claude -p --output-format json`: one object with the
// answer in "result" (format verified against Claude Code 2.1.x).
func claudeResult(raw string) (string, bool) {
	var o struct {
		Type   string  `json:"type"`
		Result *string `json:"result"`
	}
	if json.Unmarshal([]byte(raw), &o) != nil || o.Type != "result" || o.Result == nil {
		return "", false
	}
	return *o.Result, true
}
