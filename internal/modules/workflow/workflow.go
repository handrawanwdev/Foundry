// Package workflow checks a command workflow: every reference resolves,
// the dependencies form a directed acyclic graph, and the commands whose
// blueprint artifacts changed since they were written are found. It
// orders the commands in waves: a wave only depends on earlier waves, so
// the commands of one wave can be done in parallel.
package workflow

import (
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

// Issue is one problem that blocks finalizing the workflow.
type Issue struct {
	Command string `json:"command,omitempty"`
	Message string `json:"message"`
}

// Required are the artifacts at least one command must refer to when they
// exist: the UI design reaches the code only through commands.
var Required = []string{core.ArtifactDesignRules, core.ArtifactLayoutSpec}

// Validate lists what is wrong with the commands. artifacts are the ids of
// the blueprint artifacts that exist now.
func Validate(cmds []core.Command, artifacts map[string]bool) []Issue {
	var out []Issue
	add := func(id, format string, a ...any) {
		out = append(out, Issue{Command: id, Message: fmt.Sprintf(format, a...)})
	}
	ids := map[string]bool{}
	for _, c := range cmds {
		switch {
		case strings.TrimSpace(c.ID) == "":
			add("", "command tanpa ID (%q)", c.Name)
			continue
		case ids[c.ID]:
			add(c.ID, "ID %s dipakai lebih dari satu command", c.ID)
		}
		ids[c.ID] = true
	}
	for _, c := range cmds {
		if c.ID == "" {
			continue
		}
		if strings.TrimSpace(c.Name) == "" {
			add(c.ID, "nama belum diisi")
		}
		if strings.TrimSpace(c.Objective) == "" {
			add(c.ID, "tujuan belum diisi")
		}
		if strings.TrimSpace(c.Instructions) == "" {
			add(c.ID, "instruksi belum diisi")
		}
		for _, d := range c.DependsOn {
			switch {
			case d == c.ID:
				add(c.ID, "bergantung pada dirinya sendiri")
			case !ids[d]:
				add(c.ID, "dependency %s tidak ada", d)
			}
		}
		if len(c.Artifacts) == 0 {
			add(c.ID, "belum merujuk artefak blueprint")
		}
		for _, r := range c.Artifacts {
			if !artifacts[r.ID] {
				add(c.ID, "artefak blueprint %q tidak ada", r.ID)
			}
		}
	}
	for _, id := range Required {
		if artifacts[id] && len(cmds) > 0 && !slices.ContainsFunc(cmds, func(c core.Command) bool {
			return slices.ContainsFunc(c.Artifacts, func(r core.ArtifactRef) bool { return r.ID == id })
		}) {
			add("", "tidak ada command yang merujuk artefak %s", id)
		}
	}
	if cycle := Cycle(cmds); len(cycle) > 0 {
		add(cycle[0], "dependency melingkar: %s", strings.Join(cycle, " → "))
	}
	return out
}

// Covers checks the requirements the commands cover: an unknown id is an
// issue; uncovered lists the MUST requirements no command covers.
func Covers(cmds []core.Command, reqs []core.Requirement) (issues []Issue, uncovered []string) {
	known := map[string]bool{}
	for _, r := range reqs {
		known[r.ID] = true
	}
	covered := map[string]bool{}
	for _, c := range cmds {
		for _, id := range c.Covers {
			if !known[id] {
				issues = append(issues, Issue{Command: c.ID, Message: fmt.Sprintf("requirement %s tidak ada", id)})
			}
			covered[id] = true
		}
	}
	if len(cmds) == 0 {
		return issues, nil
	}
	for _, r := range reqs {
		if r.Priority == "MUST" && !covered[r.ID] {
			uncovered = append(uncovered, r.ID)
		}
	}
	return issues, uncovered
}

// Cycle returns one dependency cycle (first id repeated at the end), or
// nil. Unknown and self dependencies are left to Validate.
func Cycle(cmds []core.Command) []string {
	deps := depMap(cmds)
	const (
		unseen = iota
		open
		done
	)
	state := map[string]int{}
	var stack []string
	var found []string
	var visit func(id string) bool
	visit = func(id string) bool {
		state[id] = open
		stack = append(stack, id)
		for _, d := range deps[id] {
			switch state[d] {
			case open:
				i := slices.Index(stack, d)
				found = append(append([]string{}, stack[i:]...), d)
				return true
			case unseen:
				if visit(d) {
					return true
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = done
		return false
	}
	for _, c := range cmds {
		if state[c.ID] == unseen && visit(c.ID) {
			return found
		}
	}
	return nil
}

// depMap keeps only dependencies on other existing commands.
func depMap(cmds []core.Command) map[string][]string {
	ids := map[string]bool{}
	for _, c := range cmds {
		ids[c.ID] = true
	}
	out := map[string][]string{}
	for _, c := range cmds {
		for _, d := range c.DependsOn {
			if d != c.ID && ids[d] && !slices.Contains(out[c.ID], d) {
				out[c.ID] = append(out[c.ID], d)
			}
		}
	}
	return out
}

// Waves orders the commands: each wave depends only on earlier waves.
// Commands keep their list order inside a wave. Nil when there is a cycle.
func Waves(cmds []core.Command) [][]string {
	deps := depMap(cmds)
	placed := map[string]bool{}
	var out [][]string
	for len(placed) < len(cmds) {
		var wave []string
		for _, c := range cmds {
			if !placed[c.ID] && !slices.ContainsFunc(deps[c.ID], func(d string) bool { return !placed[d] }) {
				wave = append(wave, c.ID)
			}
		}
		if len(wave) == 0 {
			return nil
		}
		for _, id := range wave {
			placed[id] = true
		}
		out = append(out, wave)
	}
	return out
}

// Stale maps each command to the artifacts that changed or disappeared
// since the command recorded them. hashes are the current artifact hashes.
func Stale(cmds []core.Command, hashes map[string]string) map[string][]string {
	out := map[string][]string{}
	for _, c := range cmds {
		for _, r := range c.Artifacts {
			if h, ok := hashes[r.ID]; !ok || h != r.Hash {
				out[c.ID] = append(out[c.ID], r.ID)
			}
		}
	}
	return out
}

// Dependents lists the commands that depend on id, directly.
func Dependents(cmds []core.Command, id string) []string {
	var out []string
	for _, c := range cmds {
		if slices.Contains(c.DependsOn, id) {
			out = append(out, c.ID)
		}
	}
	return out
}

// Hashes indexes artifacts by id.
func Hashes(docs []core.Artifact) map[string]string {
	out := make(map[string]string, len(docs))
	for _, d := range docs {
		out[d.ID] = d.Hash()
	}
	return out
}

// Report is the workflow as the UI and the export read it.
type Report struct {
	Issues []Issue              `json:"issues"`
	Waves  [][]string           `json:"waves"`
	Stale  map[string][]string  `json:"stale"`
	Ready  bool                 `json:"ready"`
	Review map[string]string    `json:"review"` // command → why it needs review
	Docs   map[string]DocHeader `json:"docs"`
	// Uncovered are the MUST requirements no command covers; shown, not
	// blocking.
	Uncovered []string `json:"uncovered"`
}

// DocHeader names an artifact without its body.
type DocHeader struct {
	Title    string `json:"title"`
	Category string `json:"category"`
}

// Check reports the workflow against the current artifacts and
// requirements. Ready means it can be finalized: commands exist, no issue,
// nothing to review.
func Check(w *core.Workflow, docs []core.Artifact, reqs []core.Requirement) Report {
	r := Report{Issues: []Issue{}, Waves: [][]string{}, Stale: map[string][]string{}, Review: map[string]string{}, Docs: map[string]DocHeader{},
		Uncovered: []string{}}
	exists := map[string]bool{}
	for _, d := range docs {
		exists[d.ID] = true
		r.Docs[d.ID] = DocHeader{Title: d.Title, Category: d.Category}
	}
	if w == nil {
		return r
	}
	r.Issues = append(r.Issues, Validate(w.Commands, exists)...)
	issues, uncovered := Covers(w.Commands, reqs)
	r.Issues = append(r.Issues, issues...)
	if uncovered != nil {
		r.Uncovered = uncovered
	}
	if waves := Waves(w.Commands); waves != nil {
		r.Waves = waves
	}
	r.Stale = Stale(w.Commands, Hashes(docs))
	for _, c := range w.Commands {
		switch {
		case c.Status == core.CommandNeedsReview:
			r.Review[c.ID] = c.Review
		case len(r.Stale[c.ID]) > 0:
			r.Review[c.ID] = "artefak berubah: " + strings.Join(r.Stale[c.ID], ", ")
		}
	}
	r.Ready = len(w.Commands) > 0 && len(r.Issues) == 0 && len(r.Review) == 0
	return r
}
