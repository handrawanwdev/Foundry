// Package usage records the token usage of every agent run Foundry starts,
// as the CLI reported it, and sums it up. A record holds metadata only: no
// prompt, no answer, no blueprint or command text, no project name or path.
package usage

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

// File is the usage log in the project's state folder, one JSON record per
// line. MaxSize is where it is rotated to File+".1" (the older one goes).
const (
	File    = "usage.jsonl"
	MaxSize = 4 << 20
)

// Status of a run.
const (
	StatusOK         = "ok"
	StatusFailed     = "failed"
	StatusOverloaded = "overloaded"
	StatusLimit      = "limit"
)

// Telemetry says whether the token fields come from the CLI.
const (
	Reported    = "reported"
	Unavailable = "unavailable" // the CLI reported no usage: token fields are null
)

// Record is one CLI run. Token fields are pointers: null means not reported,
// never zero. The byte fields are measured by Foundry and are no token count.
type Record struct {
	RunID      string    `json:"run_id"`
	Project    string    `json:"project"` // ProjectID of the folder
	Operation  string    `json:"operation"`
	Agent      string    `json:"agent,omitempty"`
	Model      *string   `json:"model"`
	CLIVersion *string   `json:"cli_version"`
	Time       time.Time `json:"time"`

	InputTokens         *int64  `json:"input_tokens"`
	OutputTokens        *int64  `json:"output_tokens"`
	CacheCreationTokens *int64  `json:"cache_creation_input_tokens"`
	CacheReadTokens     *int64  `json:"cache_read_input_tokens"`
	CacheWrite1h        *int64  `json:"cache_creation_1h_input_tokens"`
	CacheWrite5m        *int64  `json:"cache_creation_5m_input_tokens"`
	TotalReported       *int64  `json:"total_reported_tokens"`
	NumTurns            *int64  `json:"num_turns"`
	DurationMS          *int64  `json:"duration_ms"`
	ServiceTier         *string `json:"service_tier,omitempty"`
	// CostUSD is the cost the agent CLI reported for the run (Claude Code's
	// total_cost_usd); absent when it reports none.
	CostUSD *float64 `json:"cost_usd,omitempty"`

	WallMS      int64 `json:"wall_ms"`
	PromptBytes int   `json:"prompt_bytes"`
	StaticBytes int   `json:"static_prompt_bytes"`
	OutputBytes int   `json:"output_bytes"`

	Status    string `json:"status"`
	Retry     int    `json:"retry_count"` // 0 for the first attempt
	CLICalls  int    `json:"cli_calls"`
	Telemetry string `json:"telemetry"`
}

// ProjectID names a project folder without revealing it.
func ProjectID(dir string) string {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	sum := sha256.Sum256([]byte(dir))
	return hex.EncodeToString(sum[:6])
}

// NewRunID is a random id for one run.
func NewRunID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("t%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func path(dir string) string { return filepath.Join(dir, core.StateDir, File) }

var mu sync.Mutex

// Append adds r to the usage log of the project in dir.
func Append(dir string, r Record) error {
	if dir == "" {
		return fmt.Errorf("usage: folder project kosong")
	}
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	p := path(dir)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	if fi, err := os.Stat(p); err == nil && fi.Size()+int64(len(line)) > MaxSize {
		if err := os.Rename(p, p+".1"); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(append(line, '\n'))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// Read returns the records of dir, older rotated ones first. A line that is
// no record is skipped and counted in bad; a run id seen twice keeps its
// last record.
func Read(dir string) (recs []Record, bad int, err error) {
	seen := map[string]int{}
	for _, p := range []string{path(dir) + ".1", path(dir)} {
		b, err := os.ReadFile(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, bad, err
		}
		for line := range bytes.SplitSeq(b, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			var r Record
			if json.Unmarshal(line, &r) != nil || r.RunID == "" {
				bad++
				continue
			}
			if i, ok := seen[r.RunID]; ok {
				recs[i] = r
				continue
			}
			seen[r.RunID] = len(recs)
			recs = append(recs, r)
		}
	}
	return recs, bad, nil
}

// Sum is the usage of a group of runs. Token sums only add reported values;
// Reported counts the runs that reported usage, the rest have none.
type Sum struct {
	Key                 string `json:"key"`
	Runs                int    `json:"runs"`
	Reported            int    `json:"reported_runs"`
	Failed              int    `json:"failed_runs"`
	Retries             int    `json:"retries"`
	InputTokens         int64  `json:"input_tokens"`
	OutputTokens        int64  `json:"output_tokens"`
	CacheCreationTokens int64  `json:"cache_creation_input_tokens"`
	CacheReadTokens     int64  `json:"cache_read_input_tokens"`
	TotalReported       int64  `json:"total_reported_tokens"`
	PromptBytes         int64  `json:"prompt_bytes"`
	WallMS              int64  `json:"wall_ms"`
	// CacheReadRatio is cache reads over all input the reporting runs sent
	// (plain input + cache writes + cache reads); 0 without any.
	CacheReadRatio float64 `json:"cache_read_ratio"`
}

// Groupings Summarize knows.
var Groupings = []string{"run", "operation", "project", "model", "day"}

// Summarize groups recs by one of Groupings, in key order.
func Summarize(recs []Record, by string) ([]Sum, error) {
	key := map[string]func(Record) string{
		"run":       func(r Record) string { return r.RunID },
		"operation": func(r Record) string { return r.Operation },
		"project":   func(r Record) string { return r.Project },
		"model": func(r Record) string {
			if r.Model == nil {
				return "(tidak dilaporkan)"
			}
			return *r.Model
		},
		"day": func(r Record) string { return r.Time.UTC().Format("2006-01-02") },
	}[by]
	if key == nil {
		return nil, fmt.Errorf("pengelompokan %q tidak dikenal: %s", by, strings.Join(Groupings, ", "))
	}
	sums := map[string]*Sum{}
	for _, r := range recs {
		k := key(r)
		s := sums[k]
		if s == nil {
			s = &Sum{Key: k}
			sums[k] = s
		}
		s.Runs++
		if r.Status != StatusOK {
			s.Failed++
		}
		if r.Retry > 0 {
			s.Retries++
		}
		s.PromptBytes += int64(r.PromptBytes)
		s.WallMS += r.WallMS
		if r.Telemetry != Reported {
			continue
		}
		s.Reported++
		s.InputTokens += val(r.InputTokens)
		s.OutputTokens += val(r.OutputTokens)
		s.CacheCreationTokens += val(r.CacheCreationTokens)
		s.CacheReadTokens += val(r.CacheReadTokens)
		s.TotalReported += val(r.TotalReported)
	}
	out := make([]Sum, 0, len(sums))
	for _, s := range sums {
		if in := s.InputTokens + s.CacheCreationTokens + s.CacheReadTokens; in > 0 {
			s.CacheReadRatio = float64(s.CacheReadTokens) / float64(in)
		}
		out = append(out, *s)
	}
	slices.SortFunc(out, func(a, b Sum) int { return strings.Compare(a.Key, b.Key) })
	return out, nil
}

func val(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
