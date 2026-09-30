package work

import (
	"fmt"
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
		fmt.Fprintf(&b, "PAGE %s %q on %s\nSCREENSHOT (open this image file): %s\nRENDERED OUTLINE ([n] type is components[n], (otomatis) is drawn by the wireframe):\n%s\nCOMPONENTS:\n",
			pg.ID, pg.Name, s.Device, s.Image, s.Outline)
		for i, c := range pg.Components {
			fmt.Fprintf(&b, "[%d] %s\n", i, mustJSON(c))
		}
		b.WriteString("\n")
	}
	b.WriteString(`Check only two things:
1. The screen now shows what the request asked.
2. The patch did not visibly break the page (duplicates, overlap, an element in the wrong place).
Do not judge style or anything the request did not mention.

Reply with ONLY one JSON object in a ` + "```json" + ` block:
{"status": "pass"}
or
{"status": "fix", "notes": "what is wrong on screen", "patches": [{"page": "PAGE-001", "operations": [{"op": "move", "target": 3, "after": 2}]}]}
A fix is an extra patch only, never a full page: ops add | remove | move | update ("update" with "component" replaces [n] whole).
"target"/"after" are the [n] of COMPONENTS above; "after": -1 = top, no "after" = bottom. Stay on the pages above and on what the request touched.`)
	return b.String()
}

type qaOut struct {
	Status  string      `json:"status"`
	Notes   string      `json:"notes"`
	Patches []pagePatch `json:"patches"`
}

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
