package work

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

// KindDesignQA is the visual check of a design: the pages are rendered in a
// browser and the AI critic says pass or sends a Design IR patch. It runs
// on a newly generated design and after a revision.
const KindDesignQA = "design_qa"

// MaxDesignRevisions is how many fix patches the critic may apply; the
// round after the last one only reports.
const MaxDesignRevisions = 2

// MaxQARounds: the first check plus one per fix patch.
const MaxQARounds = MaxDesignRevisions + 1

// MaxInitialShots is the render budget of the first check of a new design.
const MaxInitialShots = 12

// ActiveQA returns the check in progress: a revision's, else the one of a
// newly generated design (initial).
func ActiveQA(d *core.Design) (qa *core.DesignQA, initial bool) {
	switch {
	case d == nil:
		return nil, false
	case d.Revision != nil:
		return d.Revision.QA, false
	}
	return d.QA, d.QA != nil
}

// QAPages returns the pages to render and the viewport for each. A
// revision renders its changed pages on the device the user was looking
// at. A new design renders every page on its base device, then the mobile
// view of web pages, within MaxInitialShots; later rounds render only what
// the last fix touched.
func QAPages(p *core.Project) (ids, devices []string) {
	qa, initial := ActiveQA(p.Design)
	if initial {
		if len(qa.Next) > 0 {
			for _, s := range qa.Next {
				ids, devices = append(ids, s.PageID), append(devices, s.Device)
			}
			return ids, devices
		}
		pages := p.Design.Pages
		for _, pg := range pages {
			if len(ids) < MaxInitialShots {
				ids, devices = append(ids, pg.ID), append(devices, core.BaseDevice(pg))
			}
		}
		for _, pg := range pages {
			if len(ids) < MaxInitialShots && pg.Platform == core.PlatformWeb {
				ids, devices = append(ids, pg.ID), append(devices, "mobile")
			}
		}
		return ids, devices
	}
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
	qa, initial := ActiveQA(p.Design)
	var b strings.Builder
	if initial {
		b.WriteString(`You are the visual QA checker of Foundry's wireframe designer. A new design was just generated from the requirements.
Critique the rendered pages in the browser screenshots before the user sees them. Do not write code. Do not modify files.

`)
	} else {
		rev := p.Design.Revision
		b.WriteString(`You are the visual QA checker of Foundry's wireframe designer. A patch was just applied for the user request below.
Check the rendered result in the browser screenshots. Do not write code. Do not modify files.

REQUEST:
` + rev.Request + "\n\nPATCH APPLIED:\n- " + strings.Join(rev.Patch, "\n- ") + "\n\n")
	}
	var shot []string
	for _, s := range qa.Shots {
		pg := findPage(p.Design.Pages, s.PageID)
		if pg == nil {
			continue
		}
		fmt.Fprintf(&b, "PAGE %s %q on %s, kind %s, density %s\nINTENT: role %q; purpose %q; %s; hierarchy %s; primary action %s; secondary %s; responsive %q\nPLAN: %s\nLAYOUT (Design IR the renderer drew):\n%s\nSCREENSHOT (open this image file): %s\nRENDERED OUTLINE ([n] type is components[n], (layout) is a layout node, (otomatis) is drawn by the frame):\n%s\nCOMPONENTS:\n",
			pg.ID, pg.Name, s.Device, pg.Kind, pg.Density, pg.Role, pg.Purpose, mustJSON(pg.Intent), mustJSON(pg.Hierarchy), mustJSON(pg.PrimaryAction), mustJSON(pg.SecondaryActions), pg.Responsive, mustJSON(pg.Plan), mustJSON(pg.Layout), s.Image, s.Outline)
		for i, c := range pg.Components {
			fmt.Fprintf(&b, "[%d] %s\n", i, mustJSON(c))
		}
		if !slices.Contains(shot, pg.ID) {
			shot = append(shot, pg.ID)
			if lint := LintPage(*pg, p.Design.Domain); len(lint) > 0 {
				b.WriteString("RULE FINDINGS (measured on the IR by Foundry; confirm on the screenshot):\n")
				for _, it := range lint {
					fmt.Fprintf(&b, "- %s %s [%s] %s Fix: %s\n", it.Severity, it.Category, it.Component, it.Problem, it.Fix)
				}
			}
		}
		b.WriteString("\n")
	}
	scope := "Stay on the pages above and on what the request touched."
	status := `"status": "fix" only for critical or major problems of this request, and the fix stays on what the request touched.`
	if initial {
		scope = "Stay on the pages above."
		status = `"status": "fix" only for critical or major problems; the fix may change any page above.`
	}
	b.WriteString(`Critique the screenshot, not the JSON. It is a wireframe: judge structure, not brand colors or final styling.
Checklist: does the composition follow INTENT and PLAN (primary focus dominant, secondary and supporting content lower, density fits the frequency),
layout, composition, alignment, spacing, typography, contrast, visual hierarchy (does the page read in the INTENT hierarchy order?),
information density, navigation, usability, component consistency, CTA clarity (at most one primary action, no action shown twice), duplicate elements,
empty/loading/error, responsive on this device (overflow, cut-off text, touch targets, does the composition change as LAYOUT devices says), accessibility (labels), unnecessary decoration
or generic generated look (every section in a card, identical repeated cards, meaningless numbers, random icons, charts nobody asked for, giant hero, placeholder data like "Lorem ipsum" or "John Doe").
Every finding is concrete: the component id, what is wrong on screen and the exact change. Never "make it more modern", "cleaner" or "improve the UI".
The renderer draws the LAYOUT exactly; a problem in the drawing is fixed in the design (components or layout: the Design IR), never by asking for another renderer, CSS or a renderer change.
Severity: critical = the request is not visible on screen or the page is broken; major = a visible problem
(duplicate, overlap, element in the wrong place, overflow, a second primary action, the primary task not dominant, a table shrunk on mobile); minor = anything else worth fixing later.
` + status + `
Everything else is reported as a minor issue and the status stays "pass". Never answer only "looks good".

Reply with ONLY one JSON object in a ` + "```json" + ` block:
{"status": "pass", "issues": [{"page": "PAGE-001", "device": "desktop|tablet|mobile", "severity": "minor", "category": "hierarchy", "component": "component id or empty", "problem": "what is wrong on screen", "why": "which rule it breaks", "impact": "what it does to the user", "fix": "the concrete change"}]}
"category" is one of: ` + qaCategoryList + `.
or
{"status": "fix", "notes": "what is wrong on screen", "issues": [...], "patches": [{"page": "PAGE-001", "operations": [{"op": "move", "target": 3, "after": 2}]}]}
A fix is an extra patch only, never a full page: ops add | remove | move | update ("update" with "component" replaces [n] whole; "update" with "page_fields" changes page fields, e.g. {"layout": {...}} for the composition or devices).
"target"/"after" are the [n] of COMPONENTS above; "after": -1 = top, no "after" = bottom. ` + scope)
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
		Device    string `json:"device"`
		Problem   string `json:"problem"`
		Why       string `json:"why"`
		Impact    string `json:"impact"`
		Fix       string `json:"fix"`
	} `json:"issues"`
	Patches []pagePatch `json:"patches"`
}

// severities maps the critic's severity names onto critical | major | minor.
var severities = map[string]string{"critical": "critical", "major": "major", "high": "major", "minor": "minor", "medium": "minor", "low": "minor"}

// qaCategories are the kinds of problem the critic reports.
var qaCategories = []string{"hierarchy", "spacing", "composition", "alignment", "consistency", "responsive", "density", "navigation",
	"usability", "duplicate_action", "state", "overflow", "accessibility", "ai_slop", "other"}

var vagueWords = regexp.MustCompile(`(?i)\b(modern|clean|cleaner|nicer|better|polish|polished|improve|bagus|rapi|bersih|menarik|elegan)\b`)

// vague is a finding that names no concrete problem ("make it cleaner").
func vague(s string) bool {
	return s == "" || len(strings.Fields(s)) <= 6 && vagueWords.MatchString(s)
}

var qaCategoryList = strings.Join(qaCategories, ", ")

// ApplyQA records the check. again is true when an extra patch was applied
// and the pages must be checked once more.
func ApplyQA(p *core.Project, raw string) (again bool, err error) {
	qa, initial := ActiveQA(p.Design)
	if qa == nil || p.Design.Status == core.DesignApproved {
		return false, nil // the user kept, undid or approved meanwhile
	}
	var out qaOut
	if err := ExtractJSON(raw, &out); err != nil {
		return false, err
	}
	qa.Issues, qa.Next = nil, nil
	for _, it := range out.Issues {
		is := core.QAIssue{Severity: severities[strings.ToLower(strings.TrimSpace(it.Severity))], Problem: strings.TrimSpace(it.Problem),
			Why: strings.TrimSpace(it.Why), Impact: strings.TrimSpace(it.Impact), Fix: strings.TrimSpace(it.Fix)}
		if is.Problem == "" || vague(is.Problem) && vague(is.Fix) && strings.TrimSpace(it.Component) == "" {
			continue
		}
		if is.Severity == "" {
			is.Severity = "minor"
		}
		is.Category = strings.ToLower(strings.TrimSpace(it.Category))
		if !slices.Contains(qaCategories, is.Category) {
			is.Category = "other"
		}
		if d := strings.ToLower(strings.TrimSpace(it.Device)); slices.Contains(core.Devices, d) {
			is.Device = d
		}
		if pg := findPage(p.Design.Pages, it.Page); pg != nil {
			is.PageID = it.Page
			if slices.ContainsFunc(pg.Components, func(c core.PageComponent) bool { return c.ID == it.Component }) {
				is.Component = it.Component
			}
		}
		qa.Issues = append(qa.Issues, is)
	}
	defer func() { qa.Issues = append(qa.Issues, LintDesign(p, shotPages(qa))...) }()
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
	if initial {
		err = applyInitialFix(p, out.Patches, qa)
	} else {
		err = applyFix(p, designOut{Patches: out.Patches})
	}
	if err != nil {
		qa.Status = "failed"
		qa.Notes = strings.TrimSpace(qa.Notes + "\nPatch perbaikan ditolak: " + err.Error())
		return false, nil
	}
	qa.Round++
	qa.Status = "running"
	return true, nil
}

func shotPages(qa *core.DesignQA) []string {
	var ids []string
	for _, s := range qa.Shots {
		if !slices.Contains(ids, s.PageID) {
			ids = append(ids, s.PageID)
		}
	}
	return ids
}

// applyInitialFix applies the critic's patch to a new design: patches only,
// on the rendered pages only. The next round renders the patched pages on
// the devices their findings were seen on.
func applyInitialFix(p *core.Project, patches []pagePatch, qa *core.DesignQA) error {
	if len(patches) == 0 {
		return fmt.Errorf("perbaikan visual tanpa patch")
	}
	scope := &core.ChangeScope{Fields: true}
	for _, id := range shotPages(qa) {
		scope.Pages = append(scope.Pages, core.ScopePage{ID: id, Fields: true})
	}
	pages, _, _, _, err := mergePatch(p, p.Design.Pages, designOut{Patches: patches}, scope)
	if err != nil {
		return err
	}
	p.Design.Pages = pages
	for _, pp := range patches {
		pg := findPage(pages, pp.Page)
		if pg == nil {
			continue
		}
		var devs []string
		for _, is := range qa.Issues {
			dev := is.Device
			if is.Category == "responsive" && dev == "" {
				dev = "mobile"
			}
			if is.PageID == pg.ID && dev != "" && !slices.Contains(devs, dev) {
				devs = append(devs, dev)
			}
		}
		if len(devs) == 0 {
			devs = []string{core.BaseDevice(*pg)}
		}
		for _, d := range devs {
			qa.Next = append(qa.Next, core.QAShot{PageID: pg.ID, Device: d})
		}
	}
	return nil
}
