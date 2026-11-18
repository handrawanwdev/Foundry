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

// pagesOf are the page ids c builds: the pages recorded on it, or, for a
// command written before pages were recorded, the page ids its instructions
// name. A recorded list is exact; the names in the text are a guess (a page
// id like "analisis" is also a common word).
func pagesOf(pages []core.LayoutPage, c core.Command) []string {
	if len(c.Pages) > 0 {
		return c.Pages
	}
	return pagesNamed(pages, c)
}

// builtBy maps every page id to the written commands (other than skip) that
// list layout-spec and build it.
func builtBy(pages []core.LayoutPage, cmds []core.Command, skip string) map[string][]string {
	out := map[string][]string{}
	for _, c := range cmds {
		if c.ID == skip || strings.TrimSpace(c.Instructions) == "" || !listsLayout(c) {
			continue
		}
		for _, id := range pagesOf(pages, c) {
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

// UnbuiltPages are the pages of the design no written command builds: the
// mockups the workflow does not carry to the code yet.
func UnbuiltPages(p *core.Project, cmds []core.Command) []string {
	pages := layoutPages(p)
	if len(pages) == 0 || len(cmds) == 0 {
		return nil
	}
	return unbuiltPages(pages, cmds, "")
}

// pageIDs are the ids of pages, in layout order.
func pageIDs(pages []core.LayoutPage) []string {
	out := make([]string, 0, len(pages))
	for _, pg := range pages {
		out = append(out, pg.ID)
	}
	return out
}

// checkPages validates the page ids an answer gives a command: known, once
// each, in layout order. what names the command in the refusal.
func checkPages(pages []core.LayoutPage, ids []string, what string) ([]string, error) {
	known := pageIDs(pages)
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if !slices.Contains(known, id) {
			return nil, fmt.Errorf("%s: pages berisi %q yang bukan id halaman layout-spec (%s)", what, id, strings.Join(known, ", "))
		}
		seen[id] = true
	}
	var out []string
	for _, id := range known {
		if seen[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

// withDesign makes sure a command that builds pages lists both design
// artifacts (with their current hash), the pair the UI check asks for.
func withDesign(c *core.Command, hashes map[string]string) {
	for _, id := range []string{core.ArtifactDesignRules, core.ArtifactLayoutSpec} {
		if _, ok := hashes[id]; !ok {
			continue
		}
		if !slices.ContainsFunc(c.Artifacts, func(r core.ArtifactRef) bool { return r.ID == id }) {
			c.Artifacts = append(c.Artifacts, core.ArtifactRef{ID: id, Hash: hashes[id]})
		}
	}
}

// withoutDesign takes the design artifacts off a command that builds no page.
func withoutDesign(c *core.Command) {
	c.Artifacts = slices.DeleteFunc(c.Artifacts, func(r core.ArtifactRef) bool {
		return r.ID == core.ArtifactLayoutSpec || r.ID == core.ArtifactDesignRules
	})
	c.Pages = nil
}

// syncPageFeatures links a command to the features of the pages it builds:
// a page's mockup belongs to a feature, so the command that builds it
// serves that feature too.
func syncPageFeatures(p *core.Project, c *core.Command) {
	pages := layoutPages(p)
	for _, id := range c.Pages {
		i := slices.IndexFunc(pages, func(pg core.LayoutPage) bool { return pg.ID == id })
		if i < 0 {
			continue
		}
		f := strings.TrimSpace(pages[i].FeatureID)
		if f == "" || slices.Contains(c.FeatureIDs, f) || !slices.ContainsFunc(p.Features, func(x core.Feature) bool { return x.ID == f }) {
			continue
		}
		c.FeatureIDs = append(c.FeatureIDs, f)
	}
}

// notePages writes the page ids into the instructions, so the coding agent
// reads the same pages the validator does. A page already quoted as code is
// not repeated.
func notePages(c *core.Command) {
	var add []string
	for _, id := range c.Pages {
		if !strings.Contains(c.Instructions, "`"+id+"`") {
			add = append(add, "`"+id+"`")
		}
	}
	if len(add) > 0 && strings.TrimSpace(c.Instructions) != "" {
		c.Instructions = strings.TrimRight(c.Instructions, "\n") + "\n\nHalaman layout-spec yang dibangun (ikuti mockup-nya): " + strings.Join(add, ", ") + "."
	}
}

// settlePages lands the pages a detail round says its command builds. The
// answer names them in "pages"; without it the pages the outline gave the
// command hold, and a command written before pages were recorded falls back
// to the page ids its instructions name. An explicit empty "pages" means the
// command builds no page of layout-spec: the design artifacts are taken off
// it. The last command that lists layout-spec must build every page no other
// command built. Every refusal is about this command, so the round that gets
// it can fix it.
func settlePages(p *core.Project, cmds []core.Command, c *core.Command, answer *[]string) error {
	pages := layoutPages(p)
	if len(pages) == 0 || !listsLayout(*c) {
		return nil
	}
	known := pageIDs(pages)
	src := c.Pages
	if answer != nil {
		src = *answer
	}
	said, err := checkPages(pages, src, c.ID)
	if err != nil {
		return err
	}
	last := lastLayoutCommand(cmds, c.ID)
	left := unbuiltPages(pages, cmds, c.ID)
	if answer != nil && len(said) == 0 {
		if last && len(left) > 0 {
			return fmt.Errorf("%s adalah command terakhir yang merujuk layout-spec, tetapi halaman %s belum dibangun command mana pun; isi pages dengan halaman yang dibangunnya",
				c.ID, strings.Join(left, ", "))
		}
		withoutDesign(c)
		return nil
	}
	all := said
	if len(all) == 0 {
		all = pagesNamed(pages, *c)
	}
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
	c.Pages = all
	notePages(c)
	syncPageFeatures(p, c)
	return nil
}

// settleSketchPages lands the pages the outline gives its commands, so no
// page of the design is left out of the workflow: the ids must be known; a
// screen command (one that lists layout-spec) without pages gets the pages
// of its features; a page no command took goes to the screen command of its
// feature, else to the last screen command. An outline without any screen
// command is left to the design artifacts check. Every command with pages
// lists both design artifacts and links the features of its pages.
func settleSketchPages(p *core.Project, cmds []core.Command, hashes map[string]string) error {
	pages := layoutPages(p)
	if len(pages) == 0 {
		return nil
	}
	got := map[string]bool{}
	for i := range cmds {
		c := &cmds[i]
		ids, err := checkPages(pages, c.Pages, c.ID)
		if err != nil {
			return err
		}
		c.Pages = ids
		for _, id := range ids {
			got[id] = true
		}
	}
	featurePages := func(c *core.Command, only func(string) bool) {
		for _, pg := range pages {
			if pg.FeatureID != "" && slices.Contains(c.FeatureIDs, pg.FeatureID) && only(pg.ID) && !slices.Contains(c.Pages, pg.ID) {
				c.Pages = append(c.Pages, pg.ID)
				got[pg.ID] = true
			}
		}
	}
	for i := range cmds {
		if c := &cmds[i]; listsLayout(*c) && len(c.Pages) == 0 {
			featurePages(c, func(string) bool { return true })
		}
	}
	last := -1
	for i := range cmds {
		if listsLayout(cmds[i]) || len(cmds[i].Pages) > 0 {
			last = i
		}
	}
	if last < 0 {
		return nil // no screen command: the design artifacts check refuses the outline
	}
	for _, pg := range pages {
		if got[pg.ID] {
			continue
		}
		at := slices.IndexFunc(cmds, func(c core.Command) bool {
			return (listsLayout(c) || len(c.Pages) > 0) && pg.FeatureID != "" && slices.Contains(c.FeatureIDs, pg.FeatureID)
		})
		if at < 0 {
			at = last
		}
		cmds[at].Pages = append(cmds[at].Pages, pg.ID)
		got[pg.ID] = true
	}
	for i := range cmds {
		c := &cmds[i]
		if len(c.Pages) == 0 {
			continue
		}
		c.Pages, _ = checkPages(pages, c.Pages, c.ID) // layout order
		withDesign(c, hashes)
		syncPageFeatures(p, c)
	}
	return nil
}

// settleRevisionPages lands the pages of a command a revision round
// returns: the answer's "pages", or the pages the command had.
func settleRevisionPages(p *core.Project, c *core.Command, prev *core.Command, answer *[]string, hashes map[string]string) error {
	pages := layoutPages(p)
	if len(pages) == 0 {
		return nil
	}
	switch {
	case answer != nil:
		ids, err := checkPages(pages, *answer, c.ID)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			if listsLayout(*c) && len(pagesNamed(pages, *c)) == 0 {
				withoutDesign(c)
			}
			return nil
		}
		c.Pages = ids
	case prev != nil && len(prev.Pages) > 0:
		c.Pages = slices.Clone(prev.Pages)
	}
	if len(c.Pages) == 0 {
		return nil
	}
	withDesign(c, hashes)
	notePages(c)
	syncPageFeatures(p, c)
	return nil
}

// writeLayoutPages tells a detail round which pages its command can build:
// every page of layout-spec, who built it already, what is still unbuilt,
// and the pages the outline gave this command.
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
	if len(c.Pages) > 0 {
		fmt.Fprintf(b, "The outline gives this command the pages %s: build every element of their mockups (MOCKUP SPECS below).\n", strings.Join(c.Pages, ", "))
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

// pageSpecsBudget bounds the mockup specs one prompt carries whole.
const pageSpecsBudget = 24000

// pageHeading reports whether a layout-spec heading is the section of page id
// ("### `id` Name").
func pageHeading(heading, id string) bool {
	return strings.HasPrefix(strings.TrimLeft(heading, "# "), "`"+id+"`")
}

// pageSpecs are the sections of layout-spec for the pages ids, and of the
// shells they sit in: what each page's mockup shows (sections, fields,
// actions, status changes, use case steps, states, navigation). Over the
// budget each page keeps an equal share.
func pageSpecs(p *core.Project, docs []core.Artifact, ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	body := ""
	for _, d := range docs {
		if d.ID == core.ArtifactLayoutSpec {
			body = d.Body
		}
	}
	if body == "" {
		return ""
	}
	want := slices.Clone(ids)
	for _, pg := range layoutPages(p) {
		if slices.Contains(ids, pg.ID) && pg.Shell != "" && !slices.Contains(want, pg.Shell) {
			want = append(want, pg.Shell)
		}
	}
	var parts []string
	total := 0
	for _, sec := range splitSections(body) {
		if slices.ContainsFunc(want, func(id string) bool { return pageHeading(sec.heading, id) }) {
			t := strings.TrimRight(sec.text, "\n")
			parts = append(parts, t)
			total += len(t)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	if total > pageSpecsBudget {
		share := pageSpecsBudget / len(parts)
		for i, t := range parts {
			if len(t) > share {
				cut := strings.LastIndexByte(t[:share], '\n')
				if cut < share/2 {
					cut = share
				}
				parts[i] = t[:cut] + "\n…(dipotong; baca bagian ini utuh di layout-spec)"
			}
		}
	}
	return strings.Join(parts, "\n\n") + "\n"
}

// writePageSpecs writes the mockup specs of the pages a command builds.
func writePageSpecs(b *strings.Builder, p *core.Project, docs []core.Artifact, ids []string) {
	specs := pageSpecs(p, docs, ids)
	if specs == "" {
		return
	}
	b.WriteString("MOCKUP SPECS of the pages this command builds (the approved mockup of each page, from layout-spec): the instructions name every section, field, action, dialog, status change and state below, so the coding agent builds the screen the mockup shows. Do not drop elements; follow the order of the sections.\n")
	b.WriteString(specs + "\n")
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
		if layout && c.ID != current && len(pagesOf(pages, c)) == 0 {
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

// shownPages are the pages whose mockup specs a detail round of batch
// carries whole.
func shownPages(p *core.Project, batch []core.Command) []string {
	var out []string
	if len(layoutPages(p)) == 0 {
		return nil
	}
	for _, c := range batch {
		if listsLayout(c) {
			out = append(out, c.Pages...)
		}
	}
	return out
}

// writeRevisionPages tells a revision round which command builds which
// page of the design, the pages no command builds, and (focused) the mockup
// specs of the pages the commands it reads in full build.
func writeRevisionPages(b *strings.Builder, p *core.Project, docs []core.Artifact, cmds, full []core.Command, focused bool) {
	pages := layoutPages(p)
	if len(pages) == 0 {
		return
	}
	by := builtBy(pages, cmds, "")
	b.WriteString(`PAGES OF layout-spec (a command that builds or changes screens returns "pages" with their exact ids and lists design-rules and layout-spec):` + "\n")
	for _, pg := range pages {
		fmt.Fprintf(b, "- %s: %s", pg.ID, oneLine(pg.Name))
		if done := by[pg.ID]; len(done) > 0 {
			fmt.Fprintf(b, " (built by %s)", strings.Join(done, ", "))
		} else {
			b.WriteString(" (NOT BUILT by any command: give it to a command)")
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	if !focused {
		return
	}
	var ids []string
	for _, c := range full {
		if listsLayout(c) {
			for _, id := range pagesOf(pages, c) {
				if !slices.Contains(ids, id) {
					ids = append(ids, id)
				}
			}
		}
	}
	writePageSpecs(b, p, docs, ids)
}
