package pipeline

import (
	"regexp"
	"strings"
	"time"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/agent"
)

// measured runs one agent call and stores its size and duration on the task.
func (s *Service) measured(id, prompt string, run func() (agent.Result, error)) (string, error) {
	start := time.Now()
	res, err := run()
	out := res.Text
	ms := time.Since(start).Milliseconds()
	s.Store.Update(func(p *core.Project) error {
		add := func(m *core.TaskMetrics) {
			m.Record(len(prompt), len(out), ms, err != nil, res.Usage != nil)
			if u := res.Usage; u != nil {
				m.RecordUsage(u.InputTokens, u.CacheWriteTokens, u.CachedTokens, u.OutputTokens, u.CostUSD)
			}
		}
		if t := p.Task(id); t != nil {
			if t.Metrics == nil {
				t.Metrics = &core.TaskMetrics{}
			}
			add(t.Metrics)
		}
		if p.BuildMetrics == nil {
			p.BuildMetrics = &core.TaskMetrics{}
		}
		add(p.BuildMetrics)
		return nil
	})
	return out, err
}

var volatile = regexp.MustCompile(`[0-9]+(\.[0-9]+)?(ms|s)?|0x[0-9a-f]+|\s+`)

// failureKey makes two failure notes comparable: same text up to numbers
// (timings, line counts, ids) and whitespace.
func failureKey(note string) string {
	note = strings.ToLower(strings.TrimSpace(note))
	if len(note) > 600 {
		note = note[:600]
	}
	return volatile.ReplaceAllString(note, " ")
}

// deterministic are the failure classes whose cause is the code itself: the
// build/test run Foundry owns, and criteria the agent did not meet. Agent
// crashes, timeouts, limits, and "nothing changed" are never in this set.
var deterministic = []string{"Verifikasi gagal", "Acceptance criteria belum"}

// transient marks output that points at the environment, not the code: a
// flaky test or busy port can pass on the same input, so it is retried.
var transient = regexp.MustCompile(`(?i)(timeout|timed out|deadline exceeded|connection (refused|reset)|econn(refused|reset)|eaddrinuse|address already in use|temporar|unavailable|too many requests|rate.?limit|\b50[234]\b|no space left|out of memory|killed|flaky)`)

// RepeatedFailure reports whether note repeats the latest failure and
// another identical attempt would only burn tokens. Only deterministic
// build/test and criteria failures count, and not when the output reads
// like an environment problem (timeout, busy port, 5xx, OOM): those can
// pass on retry or need a different recovery, so they keep their attempts.
// A repeat sends the task to BLOCKED, where the repair prompt demands a
// root-cause fix.
func RepeatedFailure(prev []string, note string) bool {
	if len(prev) == 0 || transient.MatchString(note) || !isDeterministic(note) {
		return false
	}
	k := failureKey(note)
	return k != "" && k == failureKey(prev[len(prev)-1])
}

func isDeterministic(note string) bool {
	for _, p := range deterministic {
		if strings.HasPrefix(strings.TrimSpace(note), p) {
			return true
		}
	}
	return false
}
