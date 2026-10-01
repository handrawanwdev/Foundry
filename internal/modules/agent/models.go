package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Models lists the models the agent offers right now, newest first, followed
// by the preset suggestions. Sources, when available:
//   - Claude Code: the Anthropic API when ANTHROPIC_API_KEY is set, otherwise
//     the model IDs the installed CLI knows (see claudeCLIModels).
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

func claudeModels(ctx context.Context, p Preset) ([]string, error) {
	if out, err := anthropicModels(ctx); err == nil {
		return out, nil
	}
	return claudeCLIModels(ctx, p)
}

// claudeID matches a versioned Claude model ID such as claude-opus-5-5. The
// trailing group rejects longer strings (dated, "-v1" Bedrock or partial IDs).
var claudeID = regexp.MustCompile(`claude-(fable|opus|sonnet|haiku)-(\d{1,2})-(\d)([^0-9A-Za-z-])`)

// claudeCLIModels reads the model IDs embedded in the installed Claude Code
// CLI, so a CLI update brings its new models without a Foundry release. The
// list can include retired models; connecting runs a probe that rejects them.
func claudeCLIModels(ctx context.Context, p Preset) ([]string, error) {
	path, err := exec.LookPath(p.Binary)
	if err != nil {
		return nil, err
	}
	if path, err = filepath.EvalSymlinks(path); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	type version struct{ family, major, minor string }
	seen := map[version]bool{}
	buf := make([]byte, 4<<20)
	carry := 0 // bytes kept from the previous chunk so IDs across a boundary match
	for ctx.Err() == nil {
		n, err := f.Read(buf[carry:])
		chunk := buf[:carry+n]
		for _, m := range claudeID.FindAllSubmatch(chunk, -1) {
			seen[version{string(m[1]), string(m[2]), string(m[3])}] = true
		}
		if err != nil {
			break
		}
		carry = min(64, len(chunk))
		copy(buf, chunk[len(chunk)-carry:])
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	vs := make([]version, 0, len(seen))
	for v := range seen {
		vs = append(vs, v)
	}
	family := []string{"fable", "opus", "sonnet", "haiku"}
	num := func(s string) int { n, _ := strconv.Atoi(s); return n }
	slices.SortFunc(vs, func(a, b version) int {
		if c := num(b.major) - num(a.major); c != 0 {
			return c
		}
		if c := num(b.minor) - num(a.minor); c != 0 {
			return c
		}
		return slices.Index(family, a.family) - slices.Index(family, b.family)
	})
	var out []string
	for _, v := range vs {
		out = append(out, fmt.Sprintf("claude-%s-%s-%s", v.family, v.major, v.minor))
	}
	return valid(out)
}

func anthropicModels(ctx context.Context) ([]string, error) {
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
