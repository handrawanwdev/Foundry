package work

import (
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/workflow"
)

// The pages of layout-spec reach the code only through the commands that
// list it. A detail round settles the pages of its own command — the round
// that can still change them — instead of leaving the check to the end of
// the workflow, where only the last command is being written and every
// earlier one is out of reach.

func listsLayout(c core.Command) bool {
	return slices.ContainsFunc(c.Artifacts, func(r core.ArtifactRef) bool { return r.ID == core.ArtifactLayoutSpec })
}

func layoutPages(p *core.Project) []core.LayoutPage {
	if !HasUI(p) {
		return nil
	}
	return p.UI.Layout.Pages
}

// pagesNamed are the page ids the instructions of c name.
func pagesNamed(pages []core.LayoutPage, c core.Command) []string {
	var out []string
	for _, pg := range pages {
		if mentionsPage(c.Instructions, pg.ID) {
			out = append(out, pg.ID)
		}
	}
	return out
}

// builtBy maps every page id to the written commands (other than skip) that
// list layout-spec and name it.
func builtBy(pages []core.LayoutPage, cmds []core.Command, skip string) map[string][]string {
	out := map[string][]string{}
	for _, c := range cmds {
		if c.ID == skip || strings.TrimSpace(c.Instructions) == "" || !listsLayout(c) {
			continue
		}
		for _, id := range pagesNamed(pages, c) {
			out[id] = append(out[id], c.ID)
		}
	}
	return out
}

// lastLayoutCommand reports whether no command other than id that is still
// to be written lists layout-spec: the pages nobody built are then this
// command's to build.
func lastLayoutCommand(cmds []core.Command, id string) bool {
	return !slices.ContainsFunc(cmds, func(c core.Command) bool {
		return c.ID != id && strings.TrimSpace(c.Instructions) == "" && listsLayout(c)
	})
}

// unbuiltPages are the pages no written command other than skip builds.
func unbuiltPages(pages []core.LayoutPage, cmds []core.Command, skip string) []string {
	by := builtBy(pages, cmds, skip)
	var out []string
	for _, pg := range pages {
		if len(by[pg.ID]) == 0 {
			out = append(out, pg.ID)
		}
	}
	return out
}

// settlePages lands the pages a detail round says its command builds. The
// answer names them in "pages" (or in the instructions); Foundry writes the
// ids into the instructions when they are missing, so the coding agent and
// the validator read the same pages. An explicit empty "pages" means the
// command builds no page of layout-spec: the design artifacts are taken off
// it. The last command that lists layout-spec must build every page no other
// command built. Every refusal is about this command, so the round that gets
// it can fix it.
func settlePages(p *core.Project, cmds []core.Command, c *core.Command, answer *[]string) error {
	pages := layoutPages(p)
	if len(pages) == 0 || !listsLayout(*c) {
		return nil
	}
	known := make([]string, 0, len(pages))
	for _, pg := range pages {
		known = append(known, pg.ID)
	}
	var said []string
	if answer != nil {
		for _, id := range *answer {
			id = strings.TrimSpace(id)
			if id == "" || slices.Contains(said, id) {
				continue
			}
			if !slices.Contains(known, id) {
				return fmt.Errorf("%s: pages berisi %q yang bukan id halaman layout-spec (%s)", c.ID, id, strings.Join(known, ", "))
			}
			said = append(said, id)
		}
	}
	named := pagesNamed(pages, *c)
	last := lastLayoutCommand(cmds, c.ID)
	left := unbuiltPages(pages, cmds, c.ID)
	if answer != nil && len(*answer) == 0 && len(named) == 0 {
		if last && len(left) > 0 {
			return fmt.Errorf("%s adalah command terakhir yang merujuk layout-spec, tetapi halaman %s belum dibangun command mana pun; isi pages dengan halaman yang dibangunnya",
				c.ID, strings.Join(left, ", "))
		}
		c.Artifacts = slices.DeleteFunc(c.Artifacts, func(r core.ArtifactRef) bool {
			return r.ID == core.ArtifactLayoutSpec || r.ID == core.ArtifactDesignRules
		})
		return nil
	}
	all := append(slices.Clone(named), said...)
	slices.Sort(all)
	all = slices.Compact(all)
	if len(all) == 0 {
		return fmt.Errorf("%s merujuk layout-spec tetapi tidak menyebut halaman yang dibangunnya; isi pages dengan id halaman (%s), atau \"pages\": [] bila command ini tidak membangun halaman layout-spec",
			c.ID, strings.Join(known, ", "))
	}
	if last {
		var miss []string
		for _, id := range left {
			if !slices.Contains(all, id) {
				miss = append(miss, id)
			}
		}
		if len(miss) > 0 {
			return fmt.Errorf("%s adalah command terakhir yang merujuk layout-spec, tetapi halaman %s belum dibangun command mana pun; tambahkan ke pages dan instruksinya",
				c.ID, strings.Join(miss, ", "))
		}
	}
	var add []string
	for _, id := range said {
		if !slices.Contains(named, id) {
			add = append(add, "`"+id+"`")
		}
	}
	if len(add) > 0 {
		c.Instructions = strings.TrimRight(c.Instructions, "\n") + "\n\nHalaman layout-spec yang dibangun: " + strings.Join(add, ", ") + "."
	}
	return nil
}

// writeLayoutPages tells a detail round which pages its command can build:
// every page of layout-spec, who built it already, and what is still unbuilt.
func writeLayoutPages(b *strings.Builder, p *core.Project, cmds []core.Command, c core.Command) {
	pages := layoutPages(p)
	if len(pages) == 0 || !listsLayout(c) {
		return
	}
	by := builtBy(pages, cmds, c.ID)
	b.WriteString(`PAGES OF layout-spec (this command lists layout-spec: return "pages" with the exact id of every page it builds or changes; Foundry writes them into the instructions):` + "\n")
	for _, pg := range pages {
		fmt.Fprintf(b, "- %s: %s", pg.ID, oneLine(pg.Name))
		if done := by[pg.ID]; len(done) > 0 {
			fmt.Fprintf(b, " (already built by %s)", strings.Join(done, ", "))
		}
		b.WriteString("\n")
	}
	if left := unbuiltPages(pages, cmds, c.ID); len(left) > 0 {
		if lastLayoutCommand(cmds, c.ID) {
			fmt.Fprintf(b, "No other command left lists layout-spec: this command MUST build the pages no command built yet: %s.\n", strings.Join(left, ", "))
		} else {
			fmt.Fprintf(b, "Not built by any command yet: %s.\n", strings.Join(left, ", "))
		}
	}
	b.WriteString(`If this command builds no page of layout-spec, return "pages": [] and Foundry removes design-rules and layout-spec from it.` + "\n\n")
}

// reopenForUI handles a workflow whose last detail round completed it but
// whose UI check fails on commands written in earlier rounds: the final
// round cannot reach them, so refusing it would repeat forever. A command
// that lists layout-spec without naming a page, and — for a page no command
// builds — the last command that lists layout-spec, are emptied so their own
// rounds write them again; settlePages then holds them to their pages. It
// returns why, for the next round's prompt; "" when reopening cannot fix the
// problem (a design artifact without its pair is decided by the outline).
func reopenForUI(p *core.Project, w *core.Workflow, issues []workflow.Issue, current string) string {
	pages := layoutPages(p)
	reopen := map[string]bool{}
	for _, c := range w.Commands {
		layout := listsLayout(c)
		rules := slices.ContainsFunc(c.Artifacts, func(r core.ArtifactRef) bool { return r.ID == core.ArtifactDesignRules })
		if layout != rules {
			return ""
		}
		if layout && c.ID != current && len(pagesNamed(pages, c)) == 0 {
			reopen[c.ID] = true
		}
	}
	if len(unbuiltPages(pages, w.Commands, "")) > 0 {
		for i := len(w.Commands) - 1; i >= 0; i-- {
			if c := w.Commands[i]; c.ID != current && listsLayout(c) {
				reopen[c.ID] = true
				break
			}
		}
	}
	if len(reopen) == 0 {
		return ""
	}
	var ids, msgs []string
	for i := range w.Commands {
		c := &w.Commands[i]
		if reopen[c.ID] {
			ids = append(ids, c.ID)
			c.Context, c.Instructions = "", ""
			c.Inputs, c.Outputs, c.Acceptance, c.Constraints, c.Files = nil, nil, nil, nil, nil
		}
	}
	for _, is := range issues {
		msgs = append(msgs, strings.TrimSpace(is.Command+" "+is.Message))
	}
	return fmt.Sprintf("%s ditulis ulang karena cek UI workflow gagal: %s. Sebut halaman yang dibangun di \"pages\".",
		strings.Join(ids, ", "), strings.Join(msgs, "; "))
}
