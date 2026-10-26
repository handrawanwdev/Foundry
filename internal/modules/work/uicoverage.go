package work

import (
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/workflow"
)

// CheckUI verifies that the UI design actually reaches the code through the
// workflow: every page is named by a command that records the layout spec,
// and every command covering a requirement with pages records both design
// artifacts. A command may implement more than one page; what matters is
// that each page is assigned to at least one command the coding agent runs.
func CheckUI(p *core.Project, cmds []core.Command) []workflow.Issue {
	if p == nil || p.UI == nil || p.UI.Layout == nil || len(p.UI.Layout.Pages) == 0 {
		return nil
	}
	var out []workflow.Issue
	add := func(id, format string, a ...any) {
		out = append(out, workflow.Issue{Command: id, Message: fmt.Sprintf(format, a...)})
	}
	refs := func(c core.Command, id string) bool {
		return slices.ContainsFunc(c.Artifacts, func(r core.ArtifactRef) bool { return r.ID == id })
	}
	pageIDs := make([]string, 0, len(p.UI.Layout.Pages))
	for _, pg := range p.UI.Layout.Pages {
		pageIDs = append(pageIDs, pg.ID)
	}
	for _, c := range cmds {
		layout := refs(c, core.ArtifactLayoutSpec)
		rules := refs(c, core.ArtifactDesignRules)
		if layout != rules {
			add(c.ID, "merujuk salah satu artefak desain tetapi tidak keduanya (%s dan %s)",
				core.ArtifactDesignRules, core.ArtifactLayoutSpec)
		}
		if !layout {
			continue
		}
		named := slices.ContainsFunc(pageIDs, func(id string) bool { return mentionsPage(c.Instructions, id) })
		if !named {
			add(c.ID, "merujuk %s tetapi instruksi tidak menyebut satu pun id halaman (%s)",
				core.ArtifactLayoutSpec, strings.Join(pageIDs, ", "))
		}
	}
	for _, pg := range p.UI.Layout.Pages {
		found := false
		for _, c := range cmds {
			if refs(c, core.ArtifactLayoutSpec) && mentionsPage(c.Instructions, pg.ID) {
				found = true
				break
			}
		}
		if !found {
			add("", "halaman %s belum diimplementasikan command mana pun yang merujuk %s dan menyebut id halaman",
				pg.ID, core.ArtifactLayoutSpec)
		}
	}
	return out
}

// mentionsPage reports whether text names the page id as a separate word.
// Page ids are lowercase letters, digits and dashes.
func mentionsPage(text, id string) bool {
	if id == "" {
		return false
	}
	for _, tok := range strings.FieldsFunc(text, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-')
	}) {
		if strings.EqualFold(tok, id) {
			return true
		}
	}
	return false
}

// UIError formats UI coverage issues as one refusal.
func UIError(issues []workflow.Issue) error {
	var msgs []string
	for i, is := range issues {
		if i == 5 {
			msgs = append(msgs, fmt.Sprintf("... %d lagi", len(issues)-5))
			break
		}
		msgs = append(msgs, strings.TrimSpace(is.Command+" "+is.Message))
	}
	return fmt.Errorf("workflow tidak valid: %s", strings.Join(msgs, "; "))
}
