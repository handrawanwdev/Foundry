package main

import (
	"strings"
	"testing"

	tokenusage "github.com/handrawanwdev/foundry/internal/modules/usage"
)

func TestRunUsage(t *testing.T) {
	dir := t.TempDir()
	in := int64(7)
	tokenusage.Append(dir, tokenusage.Record{RunID: "a", Operation: "workflow", InputTokens: &in, OutputTokens: &in, Status: tokenusage.StatusOK, Telemetry: tokenusage.Reported})
	var out, errb strings.Builder
	if code := runUsage([]string{dir}, &out, &errb); code != 0 || !strings.Contains(out.String(), "workflow") {
		t.Fatalf("code=%d out=%s err=%s", code, out.String(), errb.String())
	}
	out.Reset()
	if code := runUsage([]string{"--json", "--by", "model", dir}, &out, &errb); code != 0 || !strings.Contains(out.String(), `"input_tokens": 7`) {
		t.Fatalf("json: %s", out.String())
	}
	if code := runUsage([]string{"--by", "week", dir}, &out, &errb); code != 2 {
		t.Fatalf("bad grouping code=%d", code)
	}
}
