package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"
)

// Models lists the models the agent offers right now, newest first, followed
// by the preset suggestions. Sources, when available:
//   - Claude Code: the Anthropic API (needs ANTHROPIC_API_KEY); without it the
//     aliases (opus, sonnet, …) already resolve to the latest model.
//   - Codex: `codex debug models`.
//   - Grok: `grok models`.
//
// A failed lookup falls back to the preset list. Results are cached briefly.
func (p Preset) ListModels(ctx context.Context) []string {
	if p.listModels == nil {
		return p.Models
	}
	modelCache.Lock()
	c, ok := modelCache.m[p.ID]
	modelCache.Unlock()
	if ok && time.Since(c.at) < modelTTL {
		return c.models
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	found, err := p.listModels(ctx, p)
	if err != nil {
		return p.Models
	}
	out := slices.Clone(found)
	for _, m := range p.Models {
		if !slices.Contains(out, m) {
			out = append(out, m)
		}
	}
	modelCache.Lock()
	modelCache.m[p.ID] = cachedModels{out, time.Now()}
	modelCache.Unlock()
	return out
}

const modelTTL = 10 * time.Minute

type cachedModels struct {
	models []string
	at     time.Time
}

var modelCache = struct {
	sync.Mutex
	m map[string]cachedModels
}{m: map[string]cachedModels{}}

func anthropicModels(ctx context.Context, _ Preset) ([]string, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, errNoSource
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.anthropic.com/v1/models?limit=100", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errNoSource
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, err
	}
	var out []string
	for _, d := range body.Data { // the API returns newest first
		out = append(out, d.ID)
	}
	return valid(out)
}

func codexModels(ctx context.Context, p Preset) ([]string, error) {
	raw, err := exec.CommandContext(ctx, p.Binary, "debug", "models").Output()
	if err != nil {
		return nil, err
	}
	type model struct {
		Slug       string `json:"slug"`
		Visibility string `json:"visibility"`
		Priority   int    `json:"priority"`
	}
	var body struct {
		Models []model `json:"models"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	slices.SortStableFunc(body.Models, func(a, b model) int { return a.Priority - b.Priority })
	var out []string
	for _, m := range body.Models {
		if m.Visibility == "list" {
			out = append(out, m.Slug)
		}
	}
	return valid(out)
}

// grokModels parses the "Available models:" block of `grok models`:
//
//   - grok-4.6 (default)
//   - grok-4.5
func grokModels(ctx context.Context, p Preset) ([]string, error) {
	raw, err := exec.CommandContext(ctx, p.Binary, "models").Output()
	if err != nil {
		return nil, err
	}
	var out []string
	sc := bufio.NewScanner(strings.NewReader(string(raw)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if rest, ok := strings.CutPrefix(line, "* "); ok {
			line = rest
		} else if rest, ok := strings.CutPrefix(line, "- "); ok {
			line = rest
		} else {
			continue
		}
		if f := strings.Fields(line); len(f) > 0 {
			out = append(out, f[0])
		}
	}
	return valid(out)
}

// valid keeps names the --model flag accepts and fails on an empty list.
func valid(in []string) ([]string, error) {
	var out []string
	for _, m := range in {
		if modelName.MatchString(m) && len(m) <= 100 && !slices.Contains(out, m) {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		return nil, errNoSource
	}
	return out, nil
}

var errNoSource = errors.New("daftar model tidak tersedia")
