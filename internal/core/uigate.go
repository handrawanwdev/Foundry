package core

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// The design quality gate: a design is not complete because every page
// exists. It is complete when each requirement and use case can be followed
// to a screen and an action, the navigation follows the flows, roles,
// data, exceptions and states are represented, and the visuals stay
// consistent. Everything here is derived from the project; nothing is
// estimated, and what the design assumed is listed for confirmation.

// Gate check statuses.
const (
	GatePass = "pass"
	GateWarn = "warn"
	GateFail = "fail"
	GateSkip = "skip" // nothing to check
)

// Requirement trace statuses.
const (
	TraceCovered  = "covered"  // reaches a screen, its use cases are drawn
	TracePartial  = "partial"  // reaches a screen, a use case is not fully drawn
	TraceMissing  = "missing"  // has screens to be on, but none serves it
	TraceNoScreen = "no-ui"    // served without screens (backend, CLI)
	TraceUnlinked = "unlinked" // no feature serves it
)

// MaxGateItems caps the findings listed under one check.
const MaxGateItems = 12

// GateCheck is one check of the quality gate.
type GateCheck struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Status string   `json:"status"`
	Detail string   `json:"detail"`
	Items  []string `json:"items,omitempty"`
}

// ReqTrace follows a requirement to the screens that serve it.
type ReqTrace struct {
	ID       string   `json:"id"`
	Text     string   `json:"text"`
	Priority string   `json:"priority,omitempty"`
	Features []string `json:"features,omitempty"`
	Flows    []string `json:"flows,omitempty"`
	Pages    []string `json:"pages,omitempty"`
	Actions  []string `json:"actions,omitempty"`
	Criteria int      `json:"criteria"`
	Status   string   `json:"status"`
}

// FlowTrace is how far a use case is drawn in the screens.
type FlowTrace struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Role     string   `json:"role,omitempty"`
	Pages    []string `json:"pages"`
	Steps    int      `json:"steps"`    // user steps
	Placed   int      `json:"placed"`   // user steps with a place on a page
	Branches int      `json:"branches"` // alternatives and failures with a screen
	Shown    int      `json:"shown"`    // of those, the ones a page shows
	Inferred bool     `json:"inferred,omitempty"`
	Declared bool     `json:"declared,omitempty"`
	Status   string   `json:"status"` // pass | warn
}

// Gate verdicts.
const (
	VerdictReady   = "ready"   // every check passes
	VerdictReview  = "review"  // nothing fails; findings to review
	VerdictBlocked = "blocked" // a check fails or the design has errors
)

// DesignGate is the quality gate report of a design. Ready means nothing
// blocks; Verdict says whether findings are still to be reviewed.
type DesignGate struct {
	Ready        bool        `json:"ready"`
	Verdict      string      `json:"verdict"`
	Summary      string      `json:"summary"`
	Checks       []GateCheck `json:"checks"`
	Requirements []ReqTrace  `json:"requirements"`
	Flows        []FlowTrace `json:"flows"`
	Errors       int         `json:"errors"`
	Warnings     int         `json:"warnings"`
	Notes        int         `json:"notes"`
	// Assumptions the design made and Unknowns it could not resolve: to be
	// confirmed with the user before the design is final.
	Assumptions []string `json:"assumptions,omitempty"`
	Unknowns    []string `json:"unknowns,omitempty"`
}

// DesignGate runs the quality gate over the current design.
func (p *Project) DesignGate() DesignGate {
	g := DesignGate{Verdict: VerdictBlocked, Checks: []GateCheck{}, Requirements: []ReqTrace{}, Flows: []FlowTrace{}}
	d := p.UI
	issues := p.DesignIssues(d)
	for _, is := range issues {
		switch is.Level {
		case IssueError:
			g.Errors++
		case IssueWarning:
			g.Warnings++
		default:
			g.Notes++
		}
	}
	if d == nil || d.Layout == nil || len(d.Layout.Pages) == 0 {
		g.Summary = "Belum ada rancangan halaman."
		return g
	}
	l := d.Layout
	walks := FlowWalks(p, l)
	g.Requirements = requirementTraces(p, l, walks)
	g.Flows = flowTraces(walks)
	g.Checks = []GateCheck{
		reqCheck(g.Requirements),
		useCaseCheck(g.Flows),
		navigationCheck(p, l, walks, d.Format),
		roleCheck(p, l, walks),
		dataCheck(l, issues),
		actionCheck(l, walks),
		exceptionCheck(walks),
		lifecycleCheck(p, l),
		responsiveCheck(d, issues),
		visualCheck(issues),
		prototypeCheck(p, l),
	}
	g.Assumptions, g.Unknowns = slices.Clone(l.Assumptions), slices.Clone(l.Unknowns)
	for _, dc := range l.Decisions {
		if dc.Source == "assumption" {
			g.Assumptions = append(g.Assumptions, fmt.Sprintf("%s: %s", dc.Subject, dc.Choice))
		}
	}
	for _, dc := range p.Decisions {
		if dc.Open() && (dc.Area == AreaUI || dc.Area == AreaUseCases) {
			g.Unknowns = append(g.Unknowns, fmt.Sprintf("%s %s", dc.ID, dc.Text))
		}
	}
	fail, warn := 0, 0
	for _, c := range g.Checks {
		switch c.Status {
		case GateFail:
			fail++
		case GateWarn:
			warn++
		}
	}
	g.Ready = g.Errors == 0 && fail == 0
	switch {
	case g.Ready && warn == 0:
		g.Verdict, g.Summary = VerdictReady, "Semua pemeriksaan lolos."
	case g.Ready:
		g.Verdict, g.Summary = VerdictReview, fmt.Sprintf("Tidak ada yang memblokir; %d pemeriksaan perlu ditinjau sebelum desain dianggap lengkap.", warn)
	default:
		g.Summary = fmt.Sprintf("Belum siap: %d pemeriksaan gagal, %d error rancangan.", fail, g.Errors)
	}
	return g
}

func requirementTraces(p *Project, l *LayoutSpec, walks []FlowWalk) []ReqTrace {
	out := []ReqTrace{}
	for _, r := range p.Requirements {
		t := ReqTrace{ID: r.ID, Text: r.Text, Priority: r.Priority, Criteria: len(r.Acceptance)}
		screens := false
		for _, f := range p.Features {
			if !slices.Contains(f.RequirementIDs, r.ID) {
				continue
			}
			t.Features = append(t.Features, f.ID)
			screens = screens || slices.ContainsFunc(f.Platforms, func(pl string) bool { return slices.Contains(UIPlatform, pl) })
		}
		addPage := func(id string) {
			if id != "" && !slices.Contains(t.Pages, id) {
				t.Pages = append(t.Pages, id)
			}
		}
		for _, pg := range l.Pages {
			if pg.FeatureID != "" && slices.Contains(t.Features, pg.FeatureID) {
				addPage(pg.ID)
			}
		}
		drawn := true
		for _, w := range walks {
			fl := p.flow(w.ID)
			if fl == nil || !slices.Contains(t.Features, fl.FeatureID) {
				continue
			}
			t.Flows = append(t.Flows, w.ID)
			if w.Inferred {
				drawn = false
				continue
			}
			for _, id := range w.Pages {
				addPage(id)
			}
			if slices.ContainsFunc(w.Steps, func(st FlowStep) bool { return !st.System && !st.Placed }) {
				drawn = false
			}
		}
		for _, id := range t.Pages {
			if a := pageAction(l, walks, id); a != "" && !slices.Contains(t.Actions, a) {
				t.Actions = append(t.Actions, a)
			}
		}
		switch {
		case len(t.Features) == 0:
			t.Status = TraceUnlinked
		case !screens && len(t.Pages) == 0:
			t.Status = TraceNoScreen
		case len(t.Pages) == 0:
			t.Status = TraceMissing
		case !drawn:
			t.Status = TracePartial
		default:
			t.Status = TraceCovered
		}
		out = append(out, t)
	}
	return out
}

// pageAction is the page's primary action: its own, else the create step a
// flow starts with there.
func pageAction(l *LayoutSpec, walks []FlowWalk, id string) string {
	pg := layoutPage(l, id)
	if pg == nil {
		return ""
	}
	if pg.PrimaryAction != "" {
		return pg.PrimaryAction
	}
	return FlowPrimaryAction(walks, id)
}

func flowTraces(walks []FlowWalk) []FlowTrace {
	out := []FlowTrace{}
	for _, w := range walks {
		t := FlowTrace{ID: w.ID, Name: w.Name, Role: w.Role, Pages: w.Pages, Inferred: w.Inferred, Declared: w.Declared, Status: GatePass}
		if t.Pages == nil {
			t.Pages = []string{}
		}
		for _, st := range w.Steps {
			if st.System {
				continue
			}
			t.Steps++
			if st.Placed {
				t.Placed++
			}
		}
		for _, br := range w.Branches {
			if br.System {
				continue
			}
			t.Branches++
			if br.Placed {
				t.Shown++
			}
		}
		if w.Inferred || len(w.Pages) == 0 || t.Placed < t.Steps {
			t.Status = GateWarn
		}
		out = append(out, t)
	}
	return out
}

func check(id, name string) GateCheck { return GateCheck{ID: id, Name: name, Status: GatePass} }

// note adds a finding and raises the status (pass < warn < fail).
func (c *GateCheck) note(status, format string, a ...any) {
	if status == GateFail || status == GateWarn && c.Status == GatePass {
		c.Status = status
	}
	if len(c.Items) < MaxGateItems {
		c.Items = append(c.Items, fmt.Sprintf(format, a...))
	}
}

func reqCheck(reqs []ReqTrace) GateCheck {
	c := check("requirement-screen", "Requirement → layar")
	n, done := 0, 0
	for _, r := range reqs {
		switch r.Status {
		case TraceNoScreen, TraceUnlinked:
			continue
		case TraceCovered:
			done++
		case TraceMissing:
			// A requirement with a use case is a user task and must reach a
			// screen; one without may be non-functional (backup, logging),
			// so it is only flagged for review.
			status, why := GateWarn, " (tanpa use case: abaikan bila non-fungsional)"
			if len(r.Flows) > 0 {
				why = ""
				if strings.EqualFold(r.Priority, "MUST") {
					status = GateFail
				}
			}
			c.note(status, "%s (%s) belum punya layar: %s%s", r.ID, firstNonBlank(r.Priority, "-"), r.Text, why)
		case TracePartial:
			c.note(GateWarn, "%s terwakili sebagian: use case %s belum tergambar penuh", r.ID, strings.Join(r.Flows, ", "))
		}
		n++
	}
	if n == 0 {
		c.Status = GateSkip
	}
	c.Detail = fmt.Sprintf("%d dari %d requirement berlayar terwakili penuh", done, n)
	return c
}

func useCaseCheck(flows []FlowTrace) GateCheck {
	c := check("usecase-screen", "Use case → layar")
	steps, placed := 0, 0
	for _, f := range flows {
		steps, placed = steps+f.Steps, placed+f.Placed
		switch {
		case len(f.Pages) == 0:
			c.note(GateWarn, "%s %s belum punya halaman", f.ID, f.Name)
		case f.Inferred:
			c.note(GateWarn, "%s %s: halaman ditebak, belum dipetakan desainer", f.ID, f.Name)
		case f.Placed < f.Steps:
			c.note(GateWarn, "%s %s: %d dari %d langkah pengguna tergambar", f.ID, f.Name, f.Placed, f.Steps)
		}
	}
	if len(flows) == 0 {
		c.Status = GateSkip
	}
	c.Detail = fmt.Sprintf("%d dari %d langkah pengguna punya tempat di layar", placed, steps)
	return c
}

func navigationCheck(p *Project, l *LayoutSpec, walks []FlowWalk, format int) GateCheck {
	c := check("workflow-navigation", "Alur → navigasi")
	missing := GateWarn
	if format >= UIDesignFormat {
		missing = GateFail
	}
	moves := 0
	for _, w := range walks {
		if w.Inferred {
			continue
		}
		for i := 1; i < len(w.Pages); i++ {
			from, to := w.Pages[i-1], w.Pages[i]
			if from == to || ParallelHop(w.Steps, from, to) {
				continue
			}
			moves++
			if !linked(l, from, to) {
				c.note(missing, "%s: %s belum punya link ke %s", w.ID, from, to)
			}
		}
		for _, br := range w.Branches {
			for _, id := range br.Flows {
				other := p.flow(id)
				if other == nil || br.Page == "" {
					continue
				}
				if to := flowPages(l, *other); len(to) > 0 && to[0] != br.Page {
					moves++
					if !linked(l, br.Page, to[0]) {
						c.note(GateWarn, "%s → %s: %s belum punya link ke %s", w.ID, id, br.Page, to[0])
					}
				}
			}
		}
	}
	if moves == 0 {
		c.Status = GateSkip
	}
	c.Detail = fmt.Sprintf("%d perpindahan halaman di alur use case", moves)
	return c
}

func linked(l *LayoutSpec, from, to string) bool {
	pg := layoutPage(l, from)
	return pg != nil && slices.ContainsFunc(pg.Links, func(ln PageLink) bool { return ln.To == to })
}

func roleCheck(p *Project, l *LayoutSpec, walks []FlowWalk) GateCheck {
	c := check("role-permission", "Peran → permission")
	var roles []string
	for _, u := range l.Users {
		if r := roleKey(u.Role); r != "" {
			roles = append(roles, r)
		}
	}
	n := 0
	for _, w := range walks {
		fl := p.flow(w.ID)
		if fl == nil {
			continue
		}
		n++
		if r := roleKey(fl.Role); r != "" && len(roles) > 0 &&
			!slices.ContainsFunc(roles, func(u string) bool { return strings.Contains(r, u) || strings.Contains(u, r) }) {
			c.note(GateWarn, "aktor %q (%s) tidak ada di daftar pengguna desain", fl.Role, fl.ID)
		}
		if len(fl.Permissions) == 0 {
			continue
		}
		for _, id := range w.Pages {
			pg := layoutPage(l, id)
			if pg == nil || pg.AccessIsPublic() {
				continue
			}
			if strings.TrimSpace(pg.Role) == "" {
				c.note(GateWarn, "%s memakai permission %s tetapi halaman %s belum menyebut peran", fl.ID, strings.Join(fl.Permissions, ", "), id)
			}
			if pg.States["permission_denied"] == "" && !sectionsHave(pg.Sections, "permission_denied") {
				c.note(GateWarn, "halaman %s belum menjelaskan tampilan tanpa izin (permission_denied)", id)
			}
		}
	}
	if n == 0 {
		c.Status = GateSkip
	}
	c.Detail = fmt.Sprintf("%d use case diperiksa terhadap peran dan permission halaman", n)
	return c
}

// roleKey is a role name for comparison: "Petugas-Gerbang" meets
// "petugas gerbang".
func roleKey(r string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(r), func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsDigit(c)
	}), " ")
}

var dataPatterns = []string{"table", "list", "detail", "dashboard", "report", "cards", "master-detail"}

func dataCheck(l *LayoutSpec, issues []Issue) GateCheck {
	c := check("data-component", "Data → komponen")
	n := 0
	for _, pg := range l.Pages {
		if pg.Existing || !slices.Contains(dataPatterns, pg.Pattern) {
			continue
		}
		n++
		if len(pg.Data) == 0 && len(pg.Fields) == 0 {
			c.note(GateWarn, "halaman %s (%s) belum menyebut data atau field yang ditampilkan", pg.ID, pg.Pattern)
		}
	}
	for _, is := range issues {
		if strings.Contains(is.Where, "/field:") {
			c.note(levelStatus(is.Level), "%s: %s", is.Where, is.Message)
		}
	}
	if n == 0 && len(c.Items) == 0 {
		c.Status = GateSkip
	}
	c.Detail = fmt.Sprintf("%d halaman data diperiksa", n)
	return c
}

func levelStatus(level string) string {
	if level == IssueError {
		return GateFail
	}
	return GateWarn
}

// actionPagePatterns are the patterns a page needs a primary action on.
var actionPagePatterns = []string{"table", "list", "detail", "dashboard", "form", "wizard"}

func actionCheck(l *LayoutSpec, walks []FlowWalk) GateCheck {
	c := check("primary-action", "Aksi utama")
	n, have := 0, 0
	for _, pg := range l.Pages {
		if pg.Existing || !slices.Contains(actionPagePatterns, pg.Pattern) {
			continue
		}
		n++
		if pageAction(l, walks, pg.ID) != "" || formLikePattern(pg.Pattern) && pageHasSubmit(&pg) {
			have++
			continue
		}
		c.note(GateWarn, "halaman %s (%s) belum punya aksi utama", pg.ID, pg.Pattern)
	}
	if n == 0 {
		c.Status = GateSkip
	}
	c.Detail = fmt.Sprintf("%d dari %d halaman punya aksi utama", have, n)
	return c
}

func formLikePattern(p string) bool { return p == "form" || p == "wizard" }

// pageHasSubmit reports whether a form names its submit button.
func pageHasSubmit(pg *LayoutPage) bool {
	for _, s := range pg.Sections {
		for _, it := range s.Content {
			if hasStem(stems(it), "tombol", "simpan", "kirim", "ajukan", "submit", "lanjut", "masuk", "daftar", "bayar") {
				return true
			}
		}
	}
	return false
}

func exceptionCheck(walks []FlowWalk) GateCheck {
	c := check("exception-flow", "Alternatif & kegagalan")
	n, shown := 0, 0
	for _, w := range walks {
		for _, br := range w.Branches {
			if br.System {
				continue
			}
			n++
			if br.Placed {
				shown++
				continue
			}
			c.note(GateWarn, "%s: %q belum terlihat di layar", w.ID, br.Text)
		}
	}
	if n == 0 {
		c.Status = GateSkip
	}
	c.Detail = fmt.Sprintf("%d dari %d skenario alternatif/kegagalan terlihat di layar", shown, n)
	return c
}

func responsiveCheck(d *UIDesign, issues []Issue) GateCheck {
	c := check("responsive", "Responsif")
	if d.Rules == nil {
		c.note(GateFail, "aturan desain belum ada")
		return c
	}
	if len(d.Rules.Responsive) == 0 {
		c.note(GateWarn, "rules.responsive belum menjelaskan breakpoint dan perubahan tata letak")
	}
	for _, is := range issues {
		if strings.Contains(is.Message, "kolom di layar") || strings.Contains(is.Message, "hardcode; pakai token breakpoint") {
			c.note(GateWarn, "%s: %s", is.Where, is.Message)
		}
	}
	c.Detail = fmt.Sprintf("%d aturan responsif", len(d.Rules.Responsive))
	return c
}

func visualCheck(issues []Issue) GateCheck {
	c := check("visual-consistency", "Konsistensi visual")
	for _, is := range issues {
		if is.Level == IssueInfo {
			continue
		}
		if is.Where == "rules" || strings.HasPrefix(is.Where, "rules") || strings.Contains(is.Message, "tidak ada di aturan desain") ||
			strings.Contains(is.Message, "hardcode") && !strings.Contains(is.Message, "breakpoint") {
			c.note(levelStatus(is.Level), "%s: %s", is.Where, is.Message)
		}
	}
	c.Detail = "token, komponen dan nilai visual mengikuti aturan desain"
	if c.Status != GatePass {
		c.Detail = fmt.Sprintf("%d temuan pada token, komponen atau nilai visual", len(c.Items))
	}
	return c
}

func prototypeCheck(p *Project, l *LayoutSpec) GateCheck {
	c := check("prototype-integrity", "Integritas prototype")
	for _, pg := range l.Pages {
		for _, ln := range pg.Links {
			if layoutPage(l, ln.To) == nil {
				c.note(GateFail, "link %s → %q menuju halaman yang tidak ada", pg.ID, ln.To)
			}
		}
	}
	for _, g := range l.Navigation {
		for _, it := range g.Items {
			if layoutPage(l, strings.TrimSpace(it.Page)) == nil {
				c.note(GateFail, "menu %q menuju halaman %q yang tidak ada", it.Label, it.Page)
			}
		}
	}
	for _, fl := range p.Flows {
		for _, id := range fl.PageIDs {
			if layoutPage(l, id) == nil {
				c.note(GateFail, "%s menyebut halaman %q yang tidak ada", fl.ID, id)
			}
		}
	}
	for _, id := range UnreachablePages(l) {
		c.note(GateWarn, "halaman %s tidak dapat dicapai dari menu atau link", id)
	}
	c.Detail = fmt.Sprintf("%d halaman, %d grup menu", len(l.Pages), len(l.Navigation))
	return c
}

// UnreachablePages are the signed-in pages no menu item or page link leads
// to. Existing and public pages are left out (they are entered from the
// shell or from outside). Without navigation nothing is reported.
func UnreachablePages(l *LayoutSpec) []string {
	if l == nil || len(l.Navigation) == 0 {
		return nil
	}
	reach := map[string]bool{}
	var walk func(string)
	walk = func(id string) {
		if reach[id] {
			return
		}
		reach[id] = true
		if pg := layoutPage(l, id); pg != nil {
			for _, ln := range pg.Links {
				walk(ln.To)
			}
		}
	}
	for _, g := range l.Navigation {
		for _, it := range g.Items {
			walk(strings.TrimSpace(it.Page))
		}
	}
	var out []string
	for _, pg := range l.Pages {
		if !reach[pg.ID] && !pg.Existing && !pg.AccessIsPublic() {
			out = append(out, pg.ID)
		}
	}
	return out
}
