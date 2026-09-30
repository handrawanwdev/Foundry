package work

import (
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

// KindDesignQA is the visual check after a revision: the changed pages are
// rendered in a browser and the AI says pass or sends an extra patch.
const KindDesignQA = "design_qa"

// MaxQARounds: at most MaxQARounds-1 extra patches, then the last check
// only reports.
const MaxQARounds = 3

// QAPages returns the pages to check and the viewport for each: the device
// the user was looking at, otherwise the platform's default.
func QAPages(p *core.Project) (ids, devices []string) {
	rev := p.Design.Revision
	for _, c := range rev.Changes {
		if c.PageID == "" || c.Kind == "removed" {
			continue
		}
		dev := ""
		if sp := rev.Scope.Page(c.PageID); sp != nil {
			dev = sp.Device
		}
		if dev == "" {
			dev = "desktop"
			if pg := findPage(p.Design.Pages, c.PageID); pg != nil && (pg.Platform == "android" || pg.Platform == "ios") {
				dev = "mobile"
			}
		}
		ids, devices = append(ids, c.PageID), append(devices, dev)
	}
	return ids, devices
}

func findPage(pages []core.Page, id string) *core.Page {
	for i := range pages {
		if pages[i].ID == id {
			return &pages[i]
		}
	}
	return nil
}

func QAPrompt(p *core.Project) string {
	rev := p.Design.Revision
	qa := rev.QA
	var b strings.Builder
	b.WriteString(`You are the visual QA checker of Foundry's wireframe designer. A patch was just applied for the user request below.
Check the rendered result in the browser screenshots. Do not write code. Do not modify files.

REQUEST:
` + rev.Request + "\n\nPATCH APPLIED:\n- " + strings.Join(rev.Patch, "\n- ") + "\n\n")
	for _, s := range qa.Shots {
		pg := findPage(p.Design.Pages, s.PageID)
		if pg == nil {
			continue
		}
		fmt.Fprintf(&b, "PAGE %s %q on %s, density %s\nINTENT: purpose %q; hierarchy %s; primary action %s; secondary %s; responsive %q\nLAYOUT (Design IR the renderer drew):\n%s\nSCREENSHOT (open this image file): %s\nRENDERED OUTLINE ([n] type is components[n], (layout) is a layout node, (otomatis) is drawn by the frame):\n%s\nCOMPONENTS:\n",
			pg.ID, pg.Name, s.Device, pg.Density, pg.Purpose, mustJSON(pg.Hierarchy), mustJSON(pg.PrimaryAction), mustJSON(pg.SecondaryActions), pg.Responsive, mustJSON(pg.Layout), s.Image, s.Outline)
		for i, c := range pg.Components {
			fmt.Fprintf(&b, "[%d] %s\n", i, mustJSON(c))
		}
		b.WriteString("\n")
	}
	b.WriteString(`Critique the screenshot, not the JSON. It is a wireframe: judge structure, not brand colors or final styling.
Checklist: layout, alignment, spacing, typography, contrast, visual hierarchy (does the page read in the INTENT hierarchy order?),
information density, component consistency, CTA clarity (at most one primary action, no action shown twice), duplicate elements,
empty/loading/error, responsive on this device (overflow, cut-off text, touch targets, does the composition change as LAYOUT devices says), accessibility (labels), unnecessary decoration
or generic generated look (every section in a card, identical repeated cards, meaningless numbers, random icons, charts nobody asked for, giant hero, placeholder data like "Lorem ipsum" or "John Doe").
The renderer draws the LAYOUT exactly; a problem in the drawing is fixed in the design (components or layout), never by asking for another renderer.
Severity: critical = the request is not visible on screen or the page is broken; major = the patch caused a visible problem
(duplicate, overlap, element in the wrong place, overflow, a second primary action); minor = anything else worth fixing later.
"status": "fix" only for critical or major problems of this request, and the fix stays on what the request touched.
Everything else is reported as a minor issue and the status stays "pass". Never answer only "looks good".

Reply with ONLY one JSON object in a ` + "```json" + ` block:
{"status": "pass", "issues": [{"page": "PAGE-001", "severity": "minor", "category": "hierarchy", "component": "component id or empty", "problem": "what is wrong on screen", "why": "which rule it breaks", "impact": "what it does to the user", "fix": "the concrete change"}]}
"category" is one of: ` + qaCategoryList + `.
or
{"status": "fix", "notes": "what is wrong on screen", "issues": [...], "patches": [{"page": "PAGE-001", "operations": [{"op": "move", "target": 3, "after": 2}]}]}
A fix is an extra patch only, never a full page: ops add | remove | move | update ("update" with "component" replaces [n] whole).
"target"/"after" are the [n] of COMPONENTS above; "after": -1 = top, no "after" = bottom. Stay on the pages above and on what the request touched.`)
	return b.String()
}

type qaOut struct {
	Status string `json:"status"`
	Notes  string `json:"notes"`
	Issues []struct {
		Page      string `json:"page"`
		Severity  string `json:"severity"`
		Category  string `json:"category"`
		Component string `json:"component"`
		Problem   string `json:"problem"`
		Why       string `json:"why"`
		Impact    string `json:"impact"`
		Fix       string `json:"fix"`
	} `json:"issues"`
	Patches []pagePatch `json:"patches"`
}

var severities = map[string]bool{"critical": true, "major": true, "minor": true}

// qaCategories are the kinds of problem the critic reports.
var qaCategories = []string{"hierarchy", "spacing", "alignment", "consistency", "responsive", "density", "duplicate_action",
	"state", "overflow", "accessibility", "ai_slop", "other"}

var qaCategoryList = strings.Join(qaCategories, ", ")

// ApplyQA records the check. again is true when an extra patch was applied
// and the pages must be checked once more.
func ApplyQA(p *core.Project, raw string) (again bool, err error) {
	if p.Design == nil || p.Design.Revision == nil || p.Design.Revision.QA == nil {
		return false, nil // the user kept or undid the change meanwhile
	}
	var out qaOut
	if err := ExtractJSON(raw, &out); err != nil {
		return false, err
	}
	qa := p.Design.Revision.QA
	qa.Issues = nil
	for _, it := range out.Issues {
		is := core.QAIssue{Severity: strings.ToLower(strings.TrimSpace(it.Severity)), Problem: strings.TrimSpace(it.Problem),
			Why: strings.TrimSpace(it.Why), Impact: strings.TrimSpace(it.Impact), Fix: strings.TrimSpace(it.Fix)}
		if is.Problem == "" {
			continue
		}
		if !severities[is.Severity] {
			is.Severity = "minor"
		}
		is.Category = strings.ToLower(strings.TrimSpace(it.Category))
		if !slices.Contains(qaCategories, is.Category) {
			is.Category = "other"
		}
		if pg := findPage(p.Design.Pages, it.Page); pg != nil {
			is.PageID = it.Page
			if slices.ContainsFunc(pg.Components, func(c core.PageComponent) bool { return c.ID == it.Component }) {
				is.Component = it.Component
			}
		}
		qa.Issues = append(qa.Issues, is)
	}
	switch {
	case out.Status == "pass":
		if qa.Round > 1 {
			qa.Status = "fixed"
		} else {
			qa.Status = "pass"
		}
		return false, nil
	case out.Status != "fix":
		return false, fmt.Errorf("status QA %q tidak dikenal", out.Status)
	}
	qa.Notes = strings.TrimSpace(out.Notes)
	if qa.Round >= MaxQARounds {
		qa.Status = "failed"
		return false, nil
	}
	if err := applyFix(p, designOut{Patches: out.Patches}); err != nil {
		qa.Status = "failed"
		qa.Notes = strings.TrimSpace(qa.Notes + "\nPatch perbaikan ditolak: " + err.Error())
		return false, nil
	}
	qa.Round++
	qa.Status = "running"
	return true, nil
}
