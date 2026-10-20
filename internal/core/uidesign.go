package core

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// UIDesign is the UI/UX design of a product with screens, as text and
// structured data only: the design rules every screen follows and the
// layout specification of its pages. Foundry never draws, renders or
// generates images of it; an external coding agent implements it. Both
// parts are blueprint artifacts (design-rules, layout-spec), so a change
// makes the commands that recorded them stale.
type UIDesign struct {
	Rules  *DesignRules `json:"rules,omitempty"`
	Layout *LayoutSpec  `json:"layout,omitempty"`
	// Request is a change waiting for the AI. Nil (or a request without
	// text) while nothing is written yet means the AI writes the design.
	Request *ChangeRequest `json:"request,omitempty"`
	// Version counts the changes.
	Version int `json:"version"`
	// TechVersion is the technology version the design was last checked
	// against; a later version that affects it asks for a review.
	TechVersion int       `json:"tech_version,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Written reports whether rules or layout exist.
func (d *UIDesign) Written() bool {
	return d != nil && (d.Rules != nil || d.Layout != nil && len(d.Layout.Pages) > 0)
}

// Design rules sources.
const (
	RulesNew      = "new"      // designed for this product
	RulesExisting = "existing" // follows the design system the project already has
)

// DesignRules are the rules a coding agent applies to every screen.
type DesignRules struct {
	Source string `json:"source"`
	// Direction is the visual direction in one sentence.
	Direction  string          `json:"direction,omitempty"`
	Principles []string        `json:"principles"`
	UX         []string        `json:"ux_guidelines"`
	Tokens     DesignTokens    `json:"tokens"`
	Components []ComponentRule `json:"components"`
	// Hierarchy is how importance is shown (size, weight, color, position).
	Hierarchy     []string        `json:"visual_hierarchy,omitempty"`
	Accessibility []string        `json:"accessibility"`
	Responsive    []string        `json:"responsive"`
	Interaction   []string        `json:"interaction,omitempty"`
	Platforms     []PlatformRules `json:"platforms,omitempty"`
	// Sources are the files of an existing project the tokens and
	// components were read from.
	Sources []string `json:"sources,omitempty"`
}

// DesignTokens: the build maps them onto the project's styling instead of
// arbitrary values.
type DesignTokens struct {
	Colors      []Token `json:"colors"`
	Typography  []Token `json:"typography"`
	Spacing     []Token `json:"spacing"`
	Radius      []Token `json:"radius,omitempty"`
	Shadow      []Token `json:"shadow,omitempty"`
	Breakpoints []Token `json:"breakpoints,omitempty"`
}

// Groups lists the token groups by name, in a fixed order.
func (t *DesignTokens) Groups() []TokenGroup {
	return []TokenGroup{{"colors", &t.Colors}, {"typography", &t.Typography}, {"spacing", &t.Spacing},
		{"radius", &t.Radius}, {"shadow", &t.Shadow}, {"breakpoints", &t.Breakpoints}}
}

type TokenGroup struct {
	Name   string
	Tokens *[]Token
}

type Token struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Use   string `json:"use,omitempty"`
}

// ComponentRule is how one shared component is used everywhere.
type ComponentRule struct {
	Name     string   `json:"name"`
	Use      string   `json:"use"`
	Variants []string `json:"variants,omitempty"`
	Rules    []string `json:"rules,omitempty"`
	// Source is the file of an existing component to reuse; empty for a
	// component the build creates.
	Source string `json:"source,omitempty"`
}

// PlatformRules are the rules only one platform follows.
type PlatformRules struct {
	Platform string   `json:"platform"`
	Rules    []string `json:"rules"`
}

// LayoutSpec is the layout of every page: shared shells, then pages made of
// sections in visual order.
type LayoutSpec struct {
	Shells []LayoutShell `json:"shells"`
	Pages  []LayoutPage  `json:"pages"`
}

// LayoutShell is a layout pages share (app frame, auth frame, ...).
type LayoutShell struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Platform   string    `json:"platform,omitempty"`
	Pattern    string    `json:"pattern"`
	Regions    []Section `json:"regions"`
	Navigation string    `json:"navigation,omitempty"`
	Grid       string    `json:"grid,omitempty"`
	Responsive []string  `json:"responsive,omitempty"`
	// Existing marks a shell the code already has; Source is its file.
	Existing bool   `json:"existing,omitempty"`
	Source   string `json:"source,omitempty"`
}

// LayoutPage is one page or screen.
type LayoutPage struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Platform  string `json:"platform"`
	Route     string `json:"route,omitempty"`
	Role      string `json:"role,omitempty"`
	Access    string `json:"access,omitempty"` // public | auth
	FeatureID string `json:"feature_id,omitempty"`
	Purpose   string `json:"purpose"`
	// Shell is the id of the shell the page sits in; empty for a page
	// with its own frame.
	Shell         string `json:"shell,omitempty"`
	Pattern       string `json:"pattern"`
	PrimaryAction string `json:"primary_action,omitempty"`
	// Sections are in visual order; Hierarchy lists section ids from the
	// most important (empty: the visual order).
	Sections   []Section         `json:"sections"`
	Hierarchy  []string          `json:"hierarchy,omitempty"`
	Grid       string            `json:"grid,omitempty"`
	Responsive []string          `json:"responsive,omitempty"`
	Links      []PageLink        `json:"links,omitempty"`
	States     map[string]string `json:"states,omitempty"`
	Data       []string          `json:"data,omitempty"`
	// Existing marks a page the code already has: the spec describes the
	// change, the page is not built again.
	Existing bool `json:"existing,omitempty"`
}

// Section is a region of a page or shell. Content lists its elements in
// order; Components the shared components (design rules) it reuses.
type Section struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Pattern    string            `json:"pattern,omitempty"`
	Purpose    string            `json:"purpose,omitempty"`
	Content    []string          `json:"content"`
	Components []string          `json:"components,omitempty"`
	Grid       string            `json:"grid,omitempty"`
	Alignment  string            `json:"alignment,omitempty"`
	Spacing    string            `json:"spacing,omitempty"`
	Sizing     string            `json:"sizing,omitempty"`
	Responsive string            `json:"responsive,omitempty"`
	States     map[string]string `json:"states,omitempty"`
}

// PageLink is a way from one page to another.
type PageLink struct {
	To  string `json:"to"`
	Via string `json:"via"`
}

// The finite vocabulary of the layout specification.
var (
	LayoutPatterns = []string{"sidebar", "navbar", "dashboard", "form", "table", "list", "detail", "master-detail",
		"wizard", "auth", "settings", "search", "cards", "feed", "tabs", "split", "modal", "editor", "chat",
		"calendar", "kanban", "map", "report", "landing", "empty", "custom"}
	UIStates   = []string{"loading", "empty", "no_results", "error", "success", "disabled", "interactive"}
	UIPlatform = []string{PlatformWeb, PlatformAndroid, PlatformIOS, PlatformDesktop}
	Accesses   = []string{"public", "auth"}
	// RequiredStates are the states a page of the pattern must specify.
	RequiredStates = map[string][]string{
		"dashboard": {"loading", "empty", "error"}, "table": {"loading", "empty", "error"},
		"list": {"loading", "empty", "error"}, "search": {"loading", "no_results", "error"},
		"feed": {"loading", "empty", "error"}, "cards": {"loading", "empty", "error"},
		"report": {"loading", "empty", "error"}, "kanban": {"loading", "empty", "error"},
		"calendar": {"loading", "empty", "error"}, "detail": {"loading", "error"},
		"master-detail": {"loading", "empty", "error"}, "form": {"error", "success", "disabled"},
		"wizard": {"error", "success", "disabled"}, "auth": {"error", "disabled"},
		"settings": {"error", "success", "disabled"},
	}
)

// Limits of one design.
const (
	MaxLayoutPages    = 80
	MaxLayoutShells   = 12
	MaxPageSections   = 30
	MaxDesignListSize = 60
)

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,47}$`)

// ValidID holds for design ids: lowercase letters, digits and dashes.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// Issue levels: an error blocks saving and finalizing, a warning is
// shown only.
const (
	IssueError   = "error"
	IssueWarning = "warning"
)

// Issue is one problem of structured blueprint data (design, use cases,
// acceptance criteria, decisions).
type Issue struct {
	Level   string `json:"level"`
	Where   string `json:"where,omitempty"` // rules | layout | page:<id> | shell:<id> | FLOW-001 | REQ-001 | DEC-001
	Message string `json:"message"`
}

// Invalid refuses structured data with errors. Subject names what was
// refused; empty means the UI design.
type Invalid struct {
	Subject string
	Issues  []Issue
}

func (e *Invalid) Error() string {
	var msgs []string
	for _, is := range e.Issues {
		if is.Level != IssueError {
			continue
		}
		if len(msgs) == 6 {
			msgs = append(msgs, "…")
			break
		}
		msgs = append(msgs, strings.TrimSpace(is.Where+": "+is.Message))
	}
	subject := e.Subject
	if subject == "" {
		subject = "rancangan UI"
	}
	return subject + " tidak valid: " + strings.Join(msgs, "; ")
}

// Errors keeps the errors of issues.
func Errors(issues []Issue) []Issue {
	var out []Issue
	for _, is := range issues {
		if is.Level == IssueError {
			out = append(out, is)
		}
	}
	return out
}

// UIPlatforms are the platforms with screens the project targets.
func (p *Project) UIPlatforms() []string {
	var out []string
	if p.Platform != nil {
		for _, t := range p.Platform.Targets {
			if slices.Contains(UIPlatform, t.Kind) && !slices.Contains(out, t.Kind) {
				out = append(out, t.Kind)
			}
		}
	}
	return out
}

// DesignIssues checks the design rules and layout spec of d against the
// project: references resolve, vocabulary is known, every page specifies
// the states its pattern needs. Nil d checks p.UI.
func (p *Project) DesignIssues(d *UIDesign) []Issue {
	if d == nil {
		d = p.UI
	}
	var out []Issue
	add := func(level, where, format string, a ...any) {
		out = append(out, Issue{Level: level, Where: where, Message: fmt.Sprintf(format, a...)})
	}
	if d == nil || d.Rules == nil {
		add(IssueError, "rules", "aturan desain belum ada")
	} else {
		out = append(out, rulesIssues(d.Rules, p.UIPlatforms())...)
	}
	if d == nil || d.Layout == nil || len(d.Layout.Pages) == 0 {
		add(IssueError, "layout", "spesifikasi layout belum berisi halaman")
		return out
	}
	var comps map[string]bool
	if d.Rules != nil {
		comps = map[string]bool{}
		for _, c := range d.Rules.Components {
			comps[strings.ToLower(c.Name)] = true
		}
	}
	return append(out, layoutIssues(p, d.Layout, comps)...)
}

func rulesIssues(r *DesignRules, platforms []string) []Issue {
	var out []Issue
	add := func(level, format string, a ...any) {
		out = append(out, Issue{Level: level, Where: "rules", Message: fmt.Sprintf(format, a...)})
	}
	if r.Source != RulesNew && r.Source != RulesExisting {
		add(IssueError, "source harus %q atau %q", RulesNew, RulesExisting)
	}
	for _, l := range []struct {
		name  string
		items []string
	}{{"principles", r.Principles}, {"ux_guidelines", r.UX}, {"accessibility", r.Accessibility}, {"responsive", r.Responsive}} {
		if len(l.items) == 0 {
			add(IssueError, "%s belum diisi", l.name)
		}
	}
	for _, g := range r.Tokens.Groups() {
		toks := *g.Tokens
		switch g.Name {
		case "colors", "typography", "spacing":
			if len(toks) == 0 {
				add(IssueError, "token %s belum diisi", g.Name)
			}
		}
		seen := map[string]bool{}
		for _, t := range toks {
			switch {
			case t.Name == "" || t.Value == "":
				add(IssueError, "token %s tanpa nama atau nilai", g.Name)
			case seen[t.Name]:
				add(IssueError, "token %s %q ganda", g.Name, t.Name)
			}
			seen[t.Name] = true
		}
	}
	seen := map[string]bool{}
	for _, c := range r.Components {
		n := strings.ToLower(c.Name)
		switch {
		case n == "" || c.Use == "":
			add(IssueError, "komponen tanpa nama atau pemakaian")
		case seen[n]:
			add(IssueError, "komponen %q ganda", c.Name)
		}
		seen[n] = true
	}
	for _, pr := range r.Platforms {
		switch {
		case !slices.Contains(UIPlatform, pr.Platform):
			add(IssueError, "aturan platform %q: bukan platform dengan layar (%s)", pr.Platform, strings.Join(UIPlatform, ", "))
		case len(platforms) > 0 && !slices.Contains(platforms, pr.Platform):
			add(IssueWarning, "aturan platform %q: platform ini tidak dipilih untuk project", pr.Platform)
		}
		if len(pr.Rules) == 0 {
			add(IssueError, "aturan platform %q kosong", pr.Platform)
		}
	}
	return out
}

func layoutIssues(p *Project, l *LayoutSpec, comps map[string]bool) []Issue {
	var out []Issue
	add := func(level, where, format string, a ...any) {
		out = append(out, Issue{Level: level, Where: where, Message: fmt.Sprintf(format, a...)})
	}
	platforms := p.UIPlatforms()
	checkPlatform := func(where, pl string, required bool) {
		switch {
		case pl == "" && !required:
		case !slices.Contains(UIPlatform, pl):
			add(IssueError, where, "platform %q bukan platform dengan layar (%s)", pl, strings.Join(UIPlatform, ", "))
		case len(platforms) > 0 && !slices.Contains(platforms, pl):
			add(IssueError, where, "platform %q tidak dipilih untuk project (%s)", pl, strings.Join(platforms, ", "))
		}
	}
	checkStates := func(where string, states map[string]string) {
		for k, v := range states {
			switch {
			case !slices.Contains(UIStates, k):
				add(IssueError, where, "state %q tidak dikenal (%s)", k, strings.Join(UIStates, ", "))
			case strings.TrimSpace(v) == "":
				add(IssueError, where, "state %q tanpa keterangan", k)
			}
		}
	}
	checkSections := func(where string, secs []Section) map[string]bool {
		ids := map[string]bool{}
		if len(secs) == 0 {
			add(IssueError, where, "belum punya section")
		}
		if len(secs) > MaxPageSections {
			add(IssueError, where, "terlalu banyak section (%d, maks %d)", len(secs), MaxPageSections)
		}
		for _, s := range secs {
			sw := where + "/" + s.ID
			switch {
			case !ValidID(s.ID):
				add(IssueError, where, "id section %q tidak sah (huruf kecil, angka, tanda minus)", s.ID)
			case ids[s.ID]:
				add(IssueError, where, "id section %q ganda", s.ID)
			}
			ids[s.ID] = true
			if s.Name == "" {
				add(IssueError, sw, "nama section belum diisi")
			}
			if len(s.Content) == 0 {
				add(IssueError, sw, "isi section (content) belum diisi")
			}
			if s.Pattern != "" && !slices.Contains(LayoutPatterns, s.Pattern) {
				add(IssueError, sw, "pattern %q tidak dikenal", s.Pattern)
			}
			for _, c := range s.Components {
				if comps != nil && !comps[strings.ToLower(c)] {
					add(IssueWarning, sw, "komponen %q tidak ada di aturan desain", c)
				}
			}
			checkStates(sw, s.States)
		}
		return ids
	}

	if len(l.Shells) > MaxLayoutShells {
		add(IssueError, "layout", "terlalu banyak shell (%d, maks %d)", len(l.Shells), MaxLayoutShells)
	}
	if len(l.Pages) > MaxLayoutPages {
		add(IssueError, "layout", "terlalu banyak halaman (%d, maks %d)", len(l.Pages), MaxLayoutPages)
	}
	shells := map[string]*LayoutShell{}
	for i := range l.Shells {
		sh := &l.Shells[i]
		where := "shell:" + sh.ID
		switch {
		case !ValidID(sh.ID):
			add(IssueError, "layout", "id shell %q tidak sah (huruf kecil, angka, tanda minus)", sh.ID)
		case shells[sh.ID] != nil:
			add(IssueError, "layout", "id shell %q ganda", sh.ID)
		}
		shells[sh.ID] = sh
		if sh.Name == "" {
			add(IssueError, where, "nama shell belum diisi")
		}
		if !slices.Contains(LayoutPatterns, sh.Pattern) {
			add(IssueError, where, "pattern %q tidak dikenal (%s)", sh.Pattern, strings.Join(LayoutPatterns, ", "))
		}
		checkPlatform(where, sh.Platform, false)
		checkSections(where, sh.Regions)
	}

	pages := map[string]bool{}
	for _, pg := range l.Pages {
		switch {
		case !ValidID(pg.ID):
			add(IssueError, "layout", "id halaman %q tidak sah (huruf kecil, angka, tanda minus)", pg.ID)
		case pages[pg.ID]:
			add(IssueError, "layout", "id halaman %q ganda", pg.ID)
		}
		pages[pg.ID] = true
	}
	routes := map[string]string{}
	for _, pg := range l.Pages {
		where := "page:" + pg.ID
		if pg.Name == "" {
			add(IssueError, where, "nama halaman belum diisi")
		}
		if pg.Purpose == "" {
			add(IssueError, where, "tujuan halaman belum diisi")
		}
		checkPlatform(where, pg.Platform, true)
		if !slices.Contains(LayoutPatterns, pg.Pattern) {
			add(IssueError, where, "pattern %q tidak dikenal (%s)", pg.Pattern, strings.Join(LayoutPatterns, ", "))
		}
		if pg.Access != "" && !slices.Contains(Accesses, pg.Access) {
			add(IssueError, where, "akses %q harus public atau auth", pg.Access)
		}
		if pg.Shell != "" {
			sh := shells[pg.Shell]
			switch {
			case sh == nil:
				add(IssueError, where, "shell %q tidak ada", pg.Shell)
			case sh.Platform != "" && sh.Platform != pg.Platform:
				add(IssueError, where, "shell %q untuk platform %s, halaman %s", pg.Shell, sh.Platform, pg.Platform)
			}
		}
		if pg.Route != "" {
			if pg.Platform == PlatformWeb && !strings.HasPrefix(pg.Route, "/") {
				add(IssueError, where, "route web %q harus diawali /", pg.Route)
			}
			key := pg.Platform + " " + pg.Route
			if other, dup := routes[key]; dup {
				add(IssueError, where, "route %q sudah dipakai halaman %s", pg.Route, other)
			}
			routes[key] = pg.ID
		}
		if pg.FeatureID != "" && p.Feature(pg.FeatureID) == nil {
			add(IssueError, where, "fitur %q tidak ada", pg.FeatureID)
		}
		secs := checkSections(where, pg.Sections)
		for _, h := range pg.Hierarchy {
			if !secs[h] {
				add(IssueError, where, "hierarchy menyebut section %q yang tidak ada", h)
			}
		}
		for _, ln := range pg.Links {
			switch {
			case !pages[ln.To]:
				add(IssueError, where, "link ke halaman %q yang tidak ada", ln.To)
			case ln.To == pg.ID:
				add(IssueError, where, "link ke dirinya sendiri")
			}
			if ln.Via == "" {
				add(IssueError, where, "link ke %q tanpa pemicu (via)", ln.To)
			}
		}
		checkStates(where, pg.States)
		for _, st := range RequiredStates[pg.Pattern] {
			if pg.States[st] == "" && !sectionsHave(pg.Sections, st) {
				add(IssueError, where, "state %q wajib untuk pattern %s", st, pg.Pattern)
			}
		}
	}
	for _, fl := range p.Flows {
		for _, id := range fl.PageIDs {
			if !pages[id] {
				add(IssueError, "layout", "alur %s menyebut halaman %q yang tidak ada", fl.ID, id)
			}
		}
	}
	// Every feature on a platform with screens needs a page there.
	for _, f := range p.Features {
		screens := slices.ContainsFunc(f.Platforms, func(pl string) bool { return slices.Contains(UIPlatform, pl) })
		if screens && !slices.ContainsFunc(l.Pages, func(pg LayoutPage) bool { return pg.FeatureID == f.ID }) {
			add(IssueWarning, "layout", "fitur %s %s belum punya halaman", f.ID, f.Name)
		}
	}
	return out
}

func sectionsHave(secs []Section, state string) bool {
	return slices.ContainsFunc(secs, func(s Section) bool { return s.States[state] != "" })
}
