// Package stack holds the technology decision of the product per area:
// application stack (framework with its language and runtime, UI library,
// API protocol), data and communication, architecture and repository,
// infrastructure and operations, engineering standards.
//
// Foundry keeps no catalog, profile or default technology. The decision
// comes from facts and from the AI: what the project's code already uses
// (scan), what the user chose, and the AI's technology plan, which designs
// every other area for THIS system. This package only records, validates
// and versions what those sources decided.
package stack

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// multiAreas hold one tool per language, joined with ", ".
var multiAreas = map[string]bool{core.TechTesting: true, core.TechQuality: true}

// parts splits a value into its tools (multiAreas) or returns it whole.
func parts(area, value string) []string {
	if !multiAreas[area] {
		return []string{value}
	}
	var out []string
	for _, p := range strings.Split(value, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Analyze returns the technical needs of the product as the discovery
// analyst (AI) reported them. Foundry does not guess needs from keywords.
func Analyze(p *core.Project) []string {
	var out []string
	if d := p.Discovery; d != nil {
		for _, n := range d.TechNeeds {
			n = strings.ToLower(strings.TrimSpace(n))
			if n != "" && !slices.Contains(out, n) {
				out = append(out, n)
			}
		}
	}
	return out
}

func platformOf(p *core.Project) core.TechPlatform {
	var t core.TechPlatform
	if p.Platform == nil {
		return t
	}
	for _, x := range p.Platform.Targets {
		switch x.Kind {
		case core.PlatformWeb:
			t.Web = true
		case core.PlatformAndroid:
			t.Mobile, t.Android = true, true
		case core.PlatformIOS:
			t.Mobile, t.IOS = true, true
		case core.PlatformDesktop:
			t.Desktop = true
		case core.PlatformBackend:
			t.Backend = true
		case core.PlatformCLI:
			t.CLI = true
		}
	}
	return t
}

// Baseline is the decision before the AI designs the technology: an
// existing project's code decides the areas it shows, a new project starts
// with none. Overrides are not carried; use Carry.
func Baseline(p *core.Project) *core.TechStack {
	needs, plat := Analyze(p), platformOf(p)
	if p.Structure != nil && p.Structure.Mode == core.StructureExisting {
		s := fromCode(p, needs, plat)
		sortDecisions(s)
		return s
	}
	return &core.TechStack{Mode: core.StructureNew, Needs: needs, Platform: plat}
}

func sortDecisions(s *core.TechStack) {
	slices.SortStableFunc(s.Decisions, func(a, b core.TechDecision) int {
		return areaOrder(a.Area) - areaOrder(b.Area)
	})
}

// areaOrder puts the known areas first in their order, others after.
func areaOrder(area string) int {
	if i := slices.Index(core.TechAreas, area); i >= 0 {
		return i
	}
	return len(core.TechAreas)
}

// fromExisting reports whether d was read from the project's code.
func fromExisting(s *core.TechStack, d core.TechDecision) bool {
	return d.Source == core.SourceExisting || d.Source == "" && s.Mode == core.StructureExisting
}

func orNone(v string) string {
	if v == "" {
		return "—"
	}
	return v
}

// AreaName is the area's label for messages and documents.
func AreaName(area string) string {
	if n, ok := areaNames[area]; ok {
		return n
	}
	return area
}

var areaNames = map[string]string{
	core.TechFrontend: "Frontend", core.TechUI: "UI library", core.TechMobile: "Mobile", core.TechDesktop: "Desktop", core.TechCLI: "CLI",
	core.TechBackend: "Backend", core.TechAPI: "API protocol", core.TechDatabase: "Database", core.TechCache: "Cache", core.TechMessaging: "Messaging",
	core.TechWorkers: "Worker", core.TechStorage: "File storage", core.TechSearch: "Search", core.TechArchitecture: "Arsitektur",
	core.TechRepository: "Repository", core.TechDeployment: "Deployment", core.TechObservability: "Observability",
	core.TechTesting: "Testing", core.TechQuality: "Lint & format",
}

// fromCode reads the stack of an existing project from its components:
// each area the code shows is a decision with its evidence.
func fromCode(p *core.Project, needs []string, plat core.TechPlatform) *core.TechStack {
	s := &core.TechStack{Mode: core.StructureExisting, Needs: needs, Platform: plat}
	// component kinds are area names, except "web" for the frontend
	area := map[string]string{"web": core.TechFrontend}
	for _, a := range core.TechAreas {
		if a != core.TechFrontend {
			area[a] = a
		}
	}
	byArea := map[string]int{}
	for _, c := range p.Structure.Components {
		a, ok := area[c.Kind]
		if !ok || c.Tech == "" {
			continue
		}
		switch a {
		case core.TechFrontend:
			s.Platform.Web = true
		case core.TechMobile:
			s.Platform.Mobile = true
		case core.TechDesktop:
			s.Platform.Desktop = true
		case core.TechCLI:
			s.Platform.CLI = true
		case core.TechBackend, core.TechDatabase:
			s.Platform.Backend = true
		}
		evidence := c.Path + "/" + c.Evidence
		if c.Path == "." || c.Path == "" {
			evidence = c.Evidence
		}
		if i, ok := byArea[a]; ok {
			d := &s.Decisions[i]
			sep := " + "
			if multiAreas[a] {
				sep = ", "
			}
			if !slices.Contains(parts(a, d.Recommended), c.Tech) && !strings.Contains(d.Recommended, c.Tech) {
				d.Recommended += sep + c.Tech
			}
			if !slices.Contains(d.Evidence, evidence) {
				d.Evidence = append(d.Evidence, evidence)
				d.Reason = "Sudah dipakai di kode: " + strings.Join(d.Evidence, ", ") + "."
			}
			continue
		}
		s.Decisions = append(s.Decisions, core.TechDecision{Area: a, Recommended: c.Tech, Language: c.Language,
			Reason: "Sudah dipakai di kode: " + evidence + ".", Source: core.SourceExisting, Evidence: []string{evidence}})
		byArea[a] = len(s.Decisions) - 1
	}
	s.Decisions = append(s.Decisions, core.TechDecision{Area: core.TechArchitecture, Recommended: "Struktur yang ada",
		Reason: "Project existing mengikuti struktur kodenya sendiri.", Source: core.SourceExisting})
	return s
}

// Carry keeps the user's overrides (and their pending reviews) from old in
// next for areas that are still valid.
func Carry(old, next *core.TechStack) {
	if old == nil || next == nil {
		return
	}
	for _, d := range old.Decisions {
		if d.Override != nil {
			_ = Override(next, d.Area, d.Override.Value, d.Override.Reason)
			if n := next.Decision(d.Area); n != nil && n.Override != nil {
				n.Override.Language, n.Override.Runtime = d.Override.Language, d.Override.Runtime
			}
		}
	}
	for _, d := range old.Decisions {
		if n := next.Decision(d.Area); n != nil && d.Review != "" && (n.Override != nil || fromExisting(next, *n)) {
			n.Review = d.Review
		}
	}
	next.History, next.Status, next.Approved = old.History, old.State(), old.Approved
	if old.Proposal != nil {
		next.SetStatus(old.Proposal.Previous) // the proposal was made on the old decisions
	}
}

var ErrExistingReason = errors.New("mengganti teknologi yang sudah dipakai di kode adalah migrasi: tuliskan alasannya")

// areaID is the shape of an area name: the known areas and any area the
// AI's plan or the user names.
var areaID = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

// Override sets the user's choice for area; an empty value or the
// recommended value removes the override. Any technology is accepted:
// Foundry has no list of allowed values. Replacing a technology the code
// already uses needs a reason. An area without a decision gets one.
func Override(s *core.TechStack, area, value, reason string) error {
	if s == nil {
		return errors.New("teknologi belum ditentukan")
	}
	area = strings.TrimSpace(area)
	value, reason = strings.TrimSpace(value), strings.TrimSpace(reason)
	if strings.ContainsAny(value, "\n\r") || len(value) > 120 || len(reason) > 400 {
		return errors.New("pilihan teknologi terlalu panjang")
	}
	d := s.Decision(area)
	if d == nil {
		if !areaID.MatchString(area) {
			return fmt.Errorf("area %q tidak valid", area)
		}
		if value == "" {
			return nil
		}
		s.Decisions = append(s.Decisions, core.TechDecision{Area: area, Source: core.SourcePreference})
		sortDecisions(s)
		d = s.Decision(area)
	}
	if value == "" || strings.EqualFold(value, d.Recommended) {
		d.Override, d.Review = nil, ""
		return nil
	}
	if fromExisting(s, *d) && reason == "" {
		return ErrExistingReason
	}
	d.Override, d.Review = &core.TechOverride{Value: value, Reason: reason}, ""
	return nil
}

// Resync brings the needs and platform of a stack up to date with the
// project, without touching its decisions.
func Resync(p *core.Project, s *core.TechStack) {
	plat := platformOf(p)
	if s.Mode == core.StructureExisting {
		o := s.Platform
		plat.Web, plat.Mobile, plat.Desktop = plat.Web || o.Web, plat.Mobile || o.Mobile, plat.Desktop || o.Desktop
		plat.Backend, plat.CLI = plat.Backend || o.Backend, plat.CLI || o.CLI
	}
	s.Needs, s.Platform = Analyze(p), plat
	Refresh(s)
}
