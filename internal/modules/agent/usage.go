package agent

import "encoding/json"

// Usage is token usage as reported by the agent CLI itself (never estimated).
// The three input kinds are disjoint: InputTokens is fresh input,
// CacheWriteTokens input written to the provider cache, CachedTokens input
// read from it. Their sum is the input processed.
type Usage struct {
	InputTokens      int     `json:"input_tokens"`
	CacheWriteTokens int     `json:"cache_write_tokens"`
	CachedTokens     int     `json:"cached_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	CostUSD          float64 `json:"cost_usd"` // as reported by the CLI; 0 = not reported
}

// Result is the answer text of one run and the usage the CLI reported.
type Result struct {
	Text  string
	Usage *Usage
}

// unwrap turns raw stdout into a Result. Output the adapter cannot read is
// returned unchanged with no usage, so an unexpected format never loses the
// agent's answer.
func (r *Runner) unwrap(raw string) Result {
	if r.usage != nil {
		if text, u, ok := r.usage(raw); ok {
			return Result{Text: text, Usage: u}
		}
	}
	return Result{Text: raw}
}

// claudeUsage reads `claude -p --output-format json`: one object with the
// answer in "result" and totals in "usage" / "total_cost_usd" (format
// verified against Claude Code 2.1.x).
func claudeUsage(raw string) (string, *Usage, bool) {
	var o struct {
		Type   string  `json:"type"`
		Result *string `json:"result"`
		Cost   float64 `json:"total_cost_usd"`
		Usage  *struct {
			Input         int `json:"input_tokens"`
			CacheCreation int `json:"cache_creation_input_tokens"`
			CacheRead     int `json:"cache_read_input_tokens"`
			Output        int `json:"output_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal([]byte(raw), &o) != nil || o.Type != "result" || o.Result == nil {
		return "", nil, false
	}
	if o.Usage == nil {
		return *o.Result, nil, true
	}
	return *o.Result, &Usage{
		InputTokens:      o.Usage.Input,
		CacheWriteTokens: o.Usage.CacheCreation,
		CachedTokens:     o.Usage.CacheRead,
		OutputTokens:     o.Usage.Output,
		CostUSD:          o.Cost,
	}, true
}
