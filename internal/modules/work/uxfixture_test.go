package work

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

// TestUXFixtures replays real Foundry projects through the screen mapping
// as a design job would complete them. FOUNDRY_UX_FIXTURES names a
// directory of project folders (copies: the store is only read); the
// projects are not part of the repository, so the test is skipped without
// them. Every requirement stays traced, every user step of a use case
// lands on a page, consecutive pages stay linked and the auto-completion
// adds no design error.
func TestUXFixtures(t *testing.T) {
	root := os.Getenv("FOUNDRY_UX_FIXTURES")
	if root == "" {
		t.Skip("FOUNDRY_UX_FIXTURES not set")
	}
	paths, _ := filepath.Glob(filepath.Join(root, "*", core.StateDir, "project.json"))
	if len(paths) == 0 {
		t.Fatalf("no project under %s", root)
	}
	for _, path := range paths {
		dir := filepath.Dir(filepath.Dir(path))
		t.Run(filepath.Base(dir), func(t *testing.T) {
			st, err := core.NewStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			p, err := st.Load()
			if err != nil {
				t.Fatal(err)
			}
			if p.UI == nil || p.UI.Layout == nil || len(p.UI.Layout.Pages) == 0 {
				t.Skip("no design")
			}
			before := p.DesignGate()
			beforeErrs := len(core.Errors(p.DesignIssues(nil)))

			next := cloneDesign(p.UI)
			flows := slices.Clone(p.Flows)
			completeFlows(p, next.Layout, flows)
			after := *p
			after.UI, after.Flows = next, flows
			g := after.DesignGate()
			issues := after.DesignIssues(nil)

			if len(g.Requirements) != len(p.Requirements) {
				t.Errorf("requirements traced %d of %d", len(g.Requirements), len(p.Requirements))
			}
			if n := len(core.Errors(issues)); n > beforeErrs {
				t.Errorf("completion added design errors: %d -> %d: %s", beforeErrs, n, issueList(core.Errors(issues)))
			}
			for _, is := range issues {
				if strings.Contains(is.Message, "berlanjut ke") {
					t.Errorf("navigation still broken: %s %s", is.Where, is.Message)
				}
			}
			for _, w := range core.FlowWalks(&after, next.Layout) {
				for _, s := range w.Steps {
					if !s.System && s.Page == "" {
						t.Errorf("%s step %d %q has no page", w.ID, s.N, s.Text)
					}
				}
			}
			t.Logf("before: %s", gateLine(before))
			t.Logf("after:  %s", gateLine(g))
			for _, c := range g.Checks {
				t.Logf("  %-20s %-5s %s", c.ID, c.Status, c.Detail)
			}
		})
	}
}

func gateLine(g core.DesignGate) string {
	status := map[string]int{}
	for _, r := range g.Requirements {
		status[r.Status]++
	}
	mapped, steps, placed, branches, shown := 0, 0, 0, 0, 0
	for _, f := range g.Flows {
		if len(f.Pages) > 0 && !f.Inferred {
			mapped++
		}
		steps, placed, branches, shown = steps+f.Steps, placed+f.Placed, branches+f.Branches, shown+f.Shown
	}
	checks := map[string]int{}
	for _, c := range g.Checks {
		checks[c.Status]++
	}
	return fmt.Sprintf("ready=%v err=%d warn=%d | req %v | flows mapped %d/%d, steps placed %d/%d, branches shown %d/%d | checks %v",
		g.Ready, g.Errors, g.Warnings, status, mapped, len(g.Flows), placed, steps, shown, branches, checks)
}

func issueList(in []core.Issue) string {
	var out []string
	for _, is := range in {
		out = append(out, is.Where+": "+is.Message)
	}
	return strings.Join(out, "; ")
}
