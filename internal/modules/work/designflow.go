package work

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// flowOut is the designer's screen mapping of one use case: the page of
// each step ("" for a system step), the page that shows each alternative
// and then each failure, and the pages it passes through. The older answer
// is a plain array of page ids.
type flowOut struct {
	Pages    []string `json:"pages"`
	Steps    []string `json:"steps"`
	Branches []string `json:"branches"`
}

func (f *flowOut) UnmarshalJSON(b []byte) error {
	var pages []string
	if json.Unmarshal(b, &pages) == nil {
		*f = flowOut{Pages: pages}
		return nil
	}
	var page string
	if json.Unmarshal(b, &page) == nil {
		*f = flowOut{Pages: []string{page}}
		return nil
	}
	type plain flowOut
	var v plain
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*f = flowOut(v)
	return nil
}

// applyFlowsOut lands the designer's mapping of the use cases it returned.
// A per-step list that does not match the steps is ignored (the words
// place them instead); the pages follow the steps when only those are
// given.
func applyFlowsOut(flows []core.Flow, in map[string]flowOut) error {
	for id, fo := range in {
		i := slices.IndexFunc(flows, func(f core.Flow) bool { return f.ID == id })
		if i < 0 {
			return fmt.Errorf("alur %q tidak ada", id)
		}
		fl := &flows[i]
		norm := func(ids []string) []string {
			out := make([]string, len(ids))
			for j, pg := range ids {
				if pg = strings.TrimSpace(pg); pg != "" && pg != "-" {
					out[j] = normID(pg)
				}
			}
			return out
		}
		if len(fo.Steps) > 0 {
			fl.StepPages = nil
			if len(fo.Steps) == len(fl.Steps) {
				fl.StepPages = norm(fo.Steps)
			}
		}
		if len(fo.Branches) > 0 {
			fl.BranchPages = nil
			if len(fo.Branches) == len(fl.Alternatives)+len(fl.Failures) {
				fl.BranchPages = norm(fo.Branches)
			}
		}
		switch {
		case len(fo.Pages) > 0:
			fl.PageIDs = slices.DeleteFunc(norm(fo.Pages), func(s string) bool { return s == "" })
		case fl.StepPages != nil:
			fl.PageIDs = core.RouteOf(fl.StepPages)
		}
	}
	return nil
}

// completeFlows makes the screens of a new design follow the use cases when
// the designer left gaps: a step or scenario placed on a page that does
// not exist is placed again by its words, a flow without pages passes
// through the pages its steps name (core.InferFlowPages), consecutive
// pages of a flow are linked (the link names the step that continues
// there), a scenario that hands over to another use case links to where
// that one starts, and a list page a flow starts with a create step gets
// that step as its primary action.
func completeFlows(p *core.Project, l *core.LayoutSpec, flows []core.Flow) {
	if l == nil || len(l.Pages) == 0 {
		return
	}
	check := *p
	check.Flows = flows
	known := func(ids []string) bool {
		return !slices.ContainsFunc(ids, func(id string) bool { return id != "" && page(l, id) == nil })
	}
	for i := range flows {
		fl := &flows[i]
		if !known(fl.StepPages) {
			fl.StepPages = nil
		}
		if !known(fl.BranchPages) {
			fl.BranchPages = nil
		}
		if core.StepsDeclared(*fl) {
			fl.PageIDs = core.RouteOf(fl.StepPages)
		}
		if len(fl.PageIDs) > 0 || !check.ScreenFlow(*fl) {
			continue
		}
		fl.PageIDs = core.InferFlowPages(l, *fl)
	}
	link := func(from, to *core.LayoutPage, via string) {
		if from == nil || to == nil || from == to || from.Existing ||
			slices.ContainsFunc(from.Links, func(ln core.PageLink) bool { return ln.To == to.ID }) {
			return
		}
		from.Links = append(from.Links, core.PageLink{To: to.ID, Via: via})
	}
	for _, fl := range flows {
		if !check.ScreenFlow(fl) {
			continue
		}
		steps := check.FlowSteps(l, fl)
		for i := 1; i < len(fl.PageIDs); i++ {
			to := page(l, fl.PageIDs[i])
			if to == nil || core.ParallelHop(steps, fl.PageIDs[i-1], to.ID) {
				continue
			}
			via := "Lanjut ke " + to.Name
			if st := firstUserStep(steps, to.ID); st != "" {
				via = core.StepAction(st)
			}
			link(page(l, fl.PageIDs[i-1]), to, via)
		}
		for _, br := range core.MapFlowBranches(l, fl, steps) {
			for _, id := range br.Flows {
				other := check.FlowByID(id)
				if other == nil || br.Page == "" || len(other.PageIDs) == 0 {
					continue
				}
				link(page(l, br.Page), page(l, other.PageIDs[0]), core.StepAction(branchAction(br.Text)))
			}
		}
		if len(fl.PageIDs) == 0 {
			continue
		}
		pg := page(l, fl.PageIDs[0])
		if pg == nil || pg.Existing || pg.PrimaryAction != "" || !slices.Contains(core.ActionPatterns, pg.Pattern) {
			continue
		}
		if st := firstUserStep(steps, pg.ID); core.CreateStep(st) {
			pg.PrimaryAction = core.StepAction(st)
		}
	}
}

// branchAction is what a scenario asks the user to do: the part after its
// condition ("Belum lunas: arahkan ke pembayaran" → "arahkan ke
// pembayaran").
func branchAction(text string) string {
	text = strings.Join(strings.Fields(flowRef.ReplaceAllString(text, "")), " ")
	for _, sep := range []string{"->", "→", ":"} {
		if i := strings.LastIndex(text, sep); i >= 0 && strings.TrimSpace(text[i+len(sep):]) != "" {
			return strings.TrimSpace(text[i+len(sep):])
		}
	}
	return text
}

// flowRef is a use case reference with its brackets: "(FLOW-004 atau
// FLOW-005)", "FLOW-007".
var flowRef = regexp.MustCompile(`\([^)]*FLOW-\d+[^)]*\)|\bFLOW-\d+\b`)

func page(l *core.LayoutSpec, id string) *core.LayoutPage {
	for i := range l.Pages {
		if l.Pages[i].ID == id {
			return &l.Pages[i]
		}
	}
	return nil
}

// firstUserStep is the text of the first step the user takes on a page.
func firstUserStep(steps []core.FlowStep, pageID string) string {
	for _, st := range steps {
		if st.Page == pageID && !st.System && st.Placed {
			return st.Text
		}
	}
	return ""
}
