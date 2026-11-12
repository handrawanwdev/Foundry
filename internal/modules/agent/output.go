package agent

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
)

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

// Usage is the token usage a CLI reported for one run. A nil field was not
// in the output; it is never filled with an estimate. The fields keep the
// provider's meaning: InputTokens is the input not read from or written to
// the prompt cache, so the cache fields are not part of it.
type Usage struct {
	Model               *string `json:"model"`
	InputTokens         *int64  `json:"input_tokens"`
	OutputTokens        *int64  `json:"output_tokens"`
	CacheCreationTokens *int64  `json:"cache_creation_input_tokens"`
	CacheReadTokens     *int64  `json:"cache_read_input_tokens"`
	// CacheWrite1h and CacheWrite5m split CacheCreationTokens by cache
	// lifetime; providers price the two differently.
	CacheWrite1h *int64  `json:"cache_creation_1h_input_tokens"`
	CacheWrite5m *int64  `json:"cache_creation_5m_input_tokens"`
	NumTurns     *int64  `json:"num_turns"`
	DurationMS   *int64  `json:"duration_ms"`
	ServiceTier  *string `json:"service_tier"`
	// CostUSD is the cost the CLI itself reports for the run, nil when it
	// reports none. Foundry never computes it.
	CostUSD *float64 `json:"total_cost_usd"`
}

// claudeUsage reads the usage of `claude -p --output-format json` (fields
// checked against Claude Code 2.1.322): usage.{input,output,
// cache_creation_input,cache_read_input}_tokens, num_turns, duration_ms and
// the models in modelUsage. nil when the output is no result object.
func claudeUsage(raw string) *Usage {
	var o struct {
		Type       string         `json:"type"`
		NumTurns   *int64         `json:"num_turns"`
		DurationMS *int64         `json:"duration_ms"`
		ModelUsage map[string]any `json:"modelUsage"`
		CostUSD    *float64       `json:"total_cost_usd"`
		Usage      *struct {
			InputTokens         *int64  `json:"input_tokens"`
			OutputTokens        *int64  `json:"output_tokens"`
			CacheCreationTokens *int64  `json:"cache_creation_input_tokens"`
			CacheReadTokens     *int64  `json:"cache_read_input_tokens"`
			ServiceTier         *string `json:"service_tier"`
			CacheCreation       *struct {
				Write1h *int64 `json:"ephemeral_1h_input_tokens"`
				Write5m *int64 `json:"ephemeral_5m_input_tokens"`
			} `json:"cache_creation"`
		} `json:"usage"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(raw)), &o) != nil || o.Type != "result" {
		return nil
	}
	u := &Usage{NumTurns: o.NumTurns, DurationMS: o.DurationMS, CostUSD: o.CostUSD}
	if o.Usage != nil {
		u.InputTokens, u.OutputTokens = o.Usage.InputTokens, o.Usage.OutputTokens
		u.CacheCreationTokens, u.CacheReadTokens = o.Usage.CacheCreationTokens, o.Usage.CacheReadTokens
		u.ServiceTier = o.Usage.ServiceTier
		if cc := o.Usage.CacheCreation; cc != nil {
			u.CacheWrite1h, u.CacheWrite5m = cc.Write1h, cc.Write5m
		}
	}
	if len(o.ModelUsage) > 0 {
		m := strings.Join(slices.Sorted(maps.Keys(o.ModelUsage)), ",")
		u.Model = &m
	}
	return u
}

// Total is the sum of the four reported token counts, nil unless all four
// were reported. It is reported usage, not a bill: providers price cache
// reads and writes differently from plain input.
func (u *Usage) Total() *int64 {
	if u == nil || u.InputTokens == nil || u.OutputTokens == nil || u.CacheCreationTokens == nil || u.CacheReadTokens == nil {
		return nil
	}
	t := *u.InputTokens + *u.OutputTokens + *u.CacheCreationTokens + *u.CacheReadTokens
	return &t
}
