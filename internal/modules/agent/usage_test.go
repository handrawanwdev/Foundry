package agent

import "testing"

// Real output of `claude -p --model haiku --output-format json` (trimmed).
const claudeSample = `{"type":"result","subtype":"success","is_error":false,"total_cost_usd":0.00383188,"usage":{"input_tokens":2,"cache_creation_input_tokens":18718,"cache_read_input_tokens":8608,"output_tokens":4},"result":"OK"}`

func TestClaudeUsageParsesReportedNumbers(t *testing.T) {
	text, u, ok := claudeUsage(claudeSample)
	if !ok || text != "OK" || u == nil {
		t.Fatalf("ok=%v text=%q u=%v", ok, text, u)
	}
	if u.InputTokens != 2 || u.CacheWriteTokens != 18718 || u.OutputTokens != 4 || u.CachedTokens != 8608 || u.CostUSD != 0.00383188 {
		t.Fatalf("usage = %+v", u)
	}
}

func TestUnwrapFallsBackOnUnknownOutput(t *testing.T) {
	r := &Runner{usage: claudeUsage}
	for _, raw := range []string{"plain answer", `{"type":"other"}`, `{"type":"result"}`, ""} {
		if res := r.unwrap(raw); res.Text != raw || res.Usage != nil {
			t.Fatalf("raw %q must pass through, got %+v", raw, res)
		}
	}
	if res := (&Runner{}).unwrap(claudeSample); res.Text != claudeSample || res.Usage != nil {
		t.Fatal("runner without adapter must not touch output")
	}
	if res := r.unwrap(claudeSample); res.Text != "OK" || res.Usage == nil {
		t.Fatalf("res = %+v", res)
	}
}

func TestLimitStillDetectedThroughJSON(t *testing.T) {
	text, _, ok := claudeUsage(`{"type":"result","is_error":true,"result":"Claude AI usage limit reached|1700000000"}`)
	if !ok || !limitHit(text) {
		t.Fatalf("limit text must survive unwrap: %q", text)
	}
}
