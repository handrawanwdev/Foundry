package work

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/handrawanw/foundry/internal/core"
)

const (
	maxMemoryFiles = 200
	maxMemoryDeps  = 8
	// A summary only pays off against source worth reading: below
	// minMemorySource the agent reads the files directly, and the summary
	// must be at most 1/memoryRatio of that source. The block is capped.
	minMemorySource = 3000
	memoryRatio     = 4
	maxMemoryBlock  = 1600
)

var memorySkip = map[string]bool{"node_modules": true, ".git": true, core.StateDir: true, "dist": true, "vendor": true}

// fingerprint hashes the content of every owned file (directories walked,
// capped) so any edit invalidates a memory.
func fingerprint(dir string, files []string) (map[string]string, int) {
	fp := map[string]string{}
	size := 0
	add := func(rel string) {
		if len(fp) >= maxMemoryFiles {
			return
		}
		if data, err := os.ReadFile(filepath.Join(dir, rel)); err == nil {
			sum := sha256.Sum256(data)
			fp[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:8])
			size += len(data)
		}
	}
	for _, f := range files {
		root := filepath.Join(dir, filepath.FromSlash(f))
		info, err := os.Stat(root)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			add(f)
			continue
		}
		filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if path != root && memorySkip[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			rel, _ := filepath.Rel(dir, path)
			add(rel)
			return nil
		})
	}
	return fp, size
}

// contractHash covers what a dependent relies on besides source: the task's
// criteria, the architecture brief, and the rule version it was built under.
func contractHash(p *core.Project, t *core.Task) string {
	return contractHashWith(p, t, rulesVersion())
}

func contractHashWith(p *core.Project, t *core.Task, rules string) string {
	sum := sha256.Sum256([]byte(strings.Join(t.AcceptanceCriteria, "\n") + "\x00" + TaskArchitectureBrief(p, t) + "\x00" + rules))
	return hex.EncodeToString(sum[:8])
}

// RememberTask records a DONE task: criteria evidence as the summary, plus
// the fingerprints that decide later whether it is still valid.
func RememberTask(p *core.Project, t *core.Task) *core.TaskMemory {
	var ev []string
	for _, c := range t.Criteria {
		if e := clip(c.Evidence, 120); e != "" {
			ev = append(ev, e)
		}
	}
	fp, size := fingerprint(p.Dir, t.Files)
	return &core.TaskMemory{
		Summary:     clip(strings.Join(ev, "; "), 400),
		Files:       fp,
		Contract:    contractHash(p, t),
		SourceBytes: size,
		At:          time.Now().UTC(),
	}
}

// MemoryStale lists why m no longer matches the tree; empty means valid.
func MemoryStale(p *core.Project, t *core.Task) []string {
	m := t.Memory
	if m == nil {
		return []string{"no memory"}
	}
	var why []string
	if m.Contract != contractHash(p, t) {
		why = append(why, "contract")
	}
	now, _ := fingerprint(p.Dir, t.Files)
	for f, h := range m.Files {
		if now[f] != h {
			why = append(why, f)
		}
	}
	for f := range now {
		if _, ok := m.Files[f]; !ok {
			why = append(why, f)
		}
	}
	sort.Strings(why)
	return why
}

// worthIncluding says a valid memory is cheaper than the source it stands for.
func worthIncluding(m *core.TaskMemory) bool {
	return m.Summary != "" && m.SourceBytes >= minMemorySource && m.SourceBytes >= memoryRatio*len(m.Summary)
}

// DependencyMemory tells a task what its finished dependencies delivered,
// from memory while valid and worth its size. A changed dependency is flagged
// so the agent reads the source instead of trusting the summary.
func DependencyMemory(p *core.Project, t *core.Task) string {
	text, _ := dependencyMemory(p, t)
	return text
}

// dependencyMemory also returns the source bytes the included summaries stand
// for, to compare cost and benefit.
func dependencyMemory(p *core.Project, t *core.Task) (string, int) {
	var b strings.Builder
	source, n := 0, 0
	for _, id := range t.DependsOn {
		d := p.Task(id)
		if d == nil || d.Status != core.TaskDone || d.Memory == nil {
			continue
		}
		var line string
		if why := MemoryStale(p, d); len(why) > 0 {
			line = fmt.Sprintf("- %s %s: changed since (%s). Read the source.\n", d.ID, d.Title, strings.Join(why[:min(len(why), 3)], ", "))
		} else if worthIncluding(d.Memory) {
			line = fmt.Sprintf("- %s %s: %s\n", d.ID, d.Title, d.Memory.Summary)
			source += d.Memory.SourceBytes
		} else {
			continue
		}
		if n++; n > maxMemoryDeps || b.Len()+len(line) > maxMemoryBlock {
			break
		}
		b.WriteString(line)
	}
	if b.Len() == 0 {
		return "", 0
	}
	return "\nDEPENDENCIES DONE (summary only; source is truth, use it, do not redo):\n" + b.String(), source
}
