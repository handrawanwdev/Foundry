package agent

import "testing"

// Real output of `claude -p --model haiku --output-format json` (trimmed).
const claudeSample = `{"type":"result","subtype":"success","is_error":false,"total_cost_usd":0.00383188,"usage":{"input_tokens":2,"cache_creation_input_tokens":18718,"cache_read_input_tokens":8608,"output_tokens":4},"result":"OK"}`

func TestClaudeResultUnwrapsAnswer(t *testing.T) {
	if text, ok := claudeResult(claudeSample); !ok || text != "OK" {
		t.Fatalf("ok=%v text=%q", ok, text)
	}
}

func TestTextFallsBackOnUnknownOutput(t *testing.T) {
	r := &Runner{unwrap: claudeResult}
	for _, raw := range []string{"plain answer", `{"type":"other"}`, `{"type":"result"}`, ""} {
		if got := r.text(raw); got != raw {
			t.Fatalf("raw %q must pass through, got %q", raw, got)
		}
	}
	if got := (&Runner{}).text(claudeSample); got != claudeSample {
		t.Fatal("runner without adapter must not touch output")
	}
	if got := r.text(claudeSample); got != "OK" {
		t.Fatalf("text = %q", got)
	}
}

func TestLimitStillDetectedThroughJSON(t *testing.T) {
	text, ok := claudeResult(`{"type":"result","is_error":true,"result":"Claude AI usage limit reached|1700000000"}`)
	if !ok || !limitHit(text) {
		t.Fatalf("limit text must survive unwrap: %q", text)
	}
}
