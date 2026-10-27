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
	// Format is the schema generation. Format 2 carries the autonomous UX
	// layer (users, goals, navigation, decisions, field rules); older
	// designs keep loading and only warn about what they lack.
	Format int `json:"format,omitempty"`
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
	// Reference is the design template or product the user asked the UI to
	// follow ("Duolingo", "YouTube", a custom template). The rules below are
	// derived from it, adapted to this product; the reference itself is the
	// user's words and is never copied as brand identity or assets.
	Reference string `json:"reference,omitempty"`
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

// UIDesignFormat is the current schema generation of UIDesign.
const UIDesignFormat = 2

// LayoutSpec is the layout of every page: shared shells, then pages made of
// sections in visual order. Users, goals, navigation and decisions are the
// UX layer that makes every layout choice traceable; they are written by the
// designer, not derived from table names.
type LayoutSpec struct {
	Shells      []LayoutShell    `json:"shells"`
	Pages       []LayoutPage     `json:"pages"`
	Users       []UIUser         `json:"users,omitempty"`
	Goals       []UIGoal         `json:"goals,omitempty"`
	Navigation  []NavGroup       `json:"navigation,omitempty"`
	Decisions   []DesignDecision `json:"decisions,omitempty"`
	Assumptions []string         `json:"assumptions,omitempty"`
	Unknowns    []string         `json:"unknowns,omitempty"`
}

// UIUser is one kind of user with the context and tasks the design serves.
type UIUser struct {
	Role     string   `json:"role"`
	Platform string   `json:"platform,omitempty"`
	Context  string   `json:"context,omitempty"`
	Goals    []string `json:"goals,omitempty"`
	Tasks    []string `json:"tasks,omitempty"`
}

// UIGoal is one measurable user goal the screens must support.
type UIGoal struct {
	ID       string `json:"id"`
	Role     string `json:"role,omitempty"`
	Text     string `json:"text"`
	Success  string `json:"success,omitempty"`
	Priority string `json:"priority,omitempty"` // must | should | could
}

// NavGroup is one information-architecture group and its destinations.
type NavGroup struct {
	Name  string    `json:"name"`
	Items []NavItem `json:"items"`
}

// NavItem is one navigation destination; Page is a page id.
type NavItem struct {
	Label   string `json:"label"`
	Page    string `json:"page"`
	Primary bool   `json:"primary,omitempty"`
}

// DesignDecision is one contextual design choice and why it was made.
// Basis names the user goal, flow, entity, permission, platform or token
// the choice is traceable to.
type DesignDecision struct {
	// Subject is the element the choice affects: a page id, section id,
	// component name, or * for the whole app.
	Subject string `json:"subject"`
	// Topic is the design aspect the choice is about when the element alone
	// is not enough: navigation, density, data status, price change,
	// real-time updates, ... It keeps the aspect distinct from the element.
	Topic       string   `json:"topic,omitempty"`
	Choice      string   `json:"choice"`
	Why         []string `json:"why"`
	Basis       []string `json:"basis,omitempty"`
	Source      string   `json:"source"` // fact | assumption | constraint
	Alternative string   `json:"alternative,omitempty"`
}

// FieldRule is how one data field (or one row's action) is presented on a
// page.
type FieldRule struct {
	Field        string `json:"field"` // Entity.field
	Label        string `json:"label,omitempty"`
	Presentation string `json:"presentation"`
	Format       string `json:"format,omitempty"`
	Note         string `json:"note,omitempty"`
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
	// Goal is the user goal the page serves, from the layout goals.
	Goal string `json:"goal,omitempty"`
	// Shell is the id of the shell the page sits in; empty for a page
	// with its own frame.
	Shell         string `json:"shell,omitempty"`
	Pattern       string `json:"pattern"`
	PrimaryAction string `json:"primary_action,omitempty"`
	// Fields maps data fields to how they are shown on this page.
	Fields []FieldRule `json:"fields,omitempty"`
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
	UIStates   = []string{"loading", "empty", "no_results", "error", "success", "disabled", "permission_denied", "interactive"}
	UIPlatform = []string{PlatformWeb, PlatformAndroid, PlatformIOS, PlatformDesktop}
	// RulesPlatforms may carry UX rules for a platform without screen
	// layouts too (terminal/CLI conventions), but layout pages stay on
	// UIPlatform.
	RulesPlatforms = []string{PlatformWeb, PlatformAndroid, PlatformIOS, PlatformDesktop, PlatformCLI}
	Accesses       = []string{"public", "auth"}
	// DecisionSources tell a fact from an assumption and a platform limit.
	DecisionSources = []string{"fact", "assumption", "constraint"}
	// FieldPresentations are the ways a field may be shown. "row-action" is
	// an action (button/link) on each row of a table or list, not a data
	// value; the field's note names what the action does. "card-value" is a
	// prominent value in a card (a big price); "delta" is a change indicator
	// (amount or percent with a direction).
	FieldPresentations = []string{"table-column", "list-item", "card-title", "card-meta", "card-value", "delta",
		"detail", "form-input", "filter", "summary", "chart", "badge", "row-action", "hidden"}
	GoalPriorities = []string{"must", "should", "could"}
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

// lengthLiteral finds an arbitrary CSS length in a layout value. Layout
// values must name design tokens, never carry a hardcoded distance.
var lengthLiteral = regexp.MustCompile(`(?i)\b\d+(?:\.\d+)?(?:px|rem|em)\b`)

// ValidID holds for design ids: lowercase letters, digits and dashes.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// Issue levels: an error blocks saving and finalizing, a warning is shown
// for review, info is a non-blocking recommendation.
const (
	IssueError   = "error"
	IssueWarning = "warning"
	IssueInfo    = "info"
)

// Issue is one problem or recommendation of structured blueprint data
// (design, use cases, acceptance criteria, decisions, cross-artifact
// consistency). Level is error (blocks), warning (should be reviewed) or
// info (non-blocking suggestion).
type Issue struct {
	Level   string `json:"level"`
	Where   string `json:"where,omitempty"` // rules | layout | page:<id> | shell:<id> | FLOW-001 | REQ-001 | DEC-001 | module:<id> | feature:<id> | entity:<id>
	Message string `json:"message"`
	// Impact is the consequence if the problem stays; Suggestion how to
	// resolve it. They are set by cross-artifact checks and stay empty on
	// single-artifact schema errors.
	Impact     string `json:"impact,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
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

// UXPlatforms are the platforms whose UX rules the design may specify,
// including terminal/CLI conventions that have no layout pages.
func (p *Project) UXPlatforms() []string {
	var out []string
	if p.Platform != nil {
		for _, t := range p.Platform.Targets {
			if slices.Contains(RulesPlatforms, t.Kind) && !slices.Contains(out, t.Kind) {
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
		out = append(out, rulesIssues(d.Rules, p.UXPlatforms())...)
		// The user asked the UI to follow a design reference; the rules must
		// record it so the derived tokens, components and patterns stay
		// traceable to the template. A design that ignored the reference is
		// incomplete.
		if p.Survey != nil {
			if ref := strings.TrimSpace(p.Survey.DesignReference); ref != "" && strings.TrimSpace(d.Rules.Reference) == "" {
				add(IssueError, "rules", "referensi desain %q belum dicatat pada rules.reference; turunkan aturan dari referensi itu dan catat namanya", ref)
			}
		}
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
	return append(out, layoutIssues(p, d.Layout, d.Rules, comps, d.Format)...)
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
		case !slices.Contains(RulesPlatforms, pr.Platform):
			add(IssueError, "aturan platform %q: platform harus salah satu dari %s", pr.Platform, strings.Join(RulesPlatforms, ", "))
		case len(platforms) > 0 && !slices.Contains(platforms, pr.Platform):
			add(IssueWarning, "aturan platform %q: platform ini tidak dipilih untuk project", pr.Platform)
		}
		if len(pr.Rules) == 0 {
			add(IssueError, "aturan platform %q kosong", pr.Platform)
		}
	}
	return out
}

func layoutIssues(p *Project, l *LayoutSpec, rules *DesignRules, comps map[string]bool, format int) []Issue {
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
			for _, v := range []string{s.Grid, s.Spacing, s.Sizing, s.Responsive} {
				if lengthLiteral.MatchString(v) {
					add(IssueWarning, sw, "nilai visual %q hardcode; pakai token spacing/breakpoint dari design rules", v)
					break
				}
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
		for _, v := range append([]string{sh.Grid}, sh.Responsive...) {
			if lengthLiteral.MatchString(v) {
				add(IssueWarning, where, "nilai visual %q hardcode; pakai token breakpoint/spacing", v)
				break
			}
		}
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
		for _, v := range append([]string{pg.Grid}, pg.Responsive...) {
			if lengthLiteral.MatchString(v) {
				add(IssueWarning, where, "nilai visual %q hardcode; pakai token breakpoint/spacing", v)
				break
			}
		}
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
		if pg.Access == "auth" && pg.States["permission_denied"] == "" && !sectionsHave(pg.Sections, "permission_denied") &&
			pageRestricted(p, pg) {
			add(IssueWarning, where, "halaman dibatasi peran/permission tetapi state permission_denied belum dijelaskan")
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
	out = append(out, experienceIssues(p, l, rules, comps, format)...)
	return out
}

// experienceIssues checks the UX layer: who the users are, what they must
// achieve, how they navigate, and why every layout choice was made. A design
// in the current format must carry all of it; an older design is only
// warned, so existing projects keep loading.
func experienceIssues(p *Project, l *LayoutSpec, rules *DesignRules, comps map[string]bool, format int) []Issue {
	var out []Issue
	add := func(level, where, format string, a ...any) {
		out = append(out, Issue{Level: level, Where: where, Message: fmt.Sprintf(format, a...)})
	}
	missing := IssueWarning
	if format >= UIDesignFormat {
		missing = IssueError
	}
	known := designBasis(p, l, rules)
	subjects := map[string]bool{"*": true}
	for _, pg := range l.Pages {
		subjects[pg.ID] = true
		for _, s := range pg.Sections {
			subjects[s.ID] = true
		}
	}
	for _, sh := range l.Shells {
		subjects[sh.ID] = true
		for _, s := range sh.Regions {
			subjects[s.ID] = true
		}
	}
	for name := range comps {
		subjects[name] = true
	}

	if len(l.Users) == 0 {
		add(missing, "layout", "jenis pengguna, konteks dan tugasnya (users) belum diisi")
	}
	roles := map[string]bool{}
	for _, u := range l.Users {
		r := strings.ToLower(strings.TrimSpace(u.Role))
		switch {
		case r == "":
			add(IssueError, "layout", "user tanpa peran")
		case roles[r]:
			add(IssueWarning, "layout", "peran pengguna %q ganda", u.Role)
		}
		roles[r] = true
		if u.Platform != "" && !slices.Contains(UIPlatform, u.Platform) {
			add(IssueError, "user:"+u.Role, "platform %q tidak dikenal", u.Platform)
		}
	}

	if len(l.Goals) == 0 {
		add(missing, "layout", "tujuan pengguna (goals) belum diisi")
	}
	goals := map[string]bool{}
	for _, g := range l.Goals {
		id := strings.ToLower(strings.TrimSpace(g.ID))
		switch {
		case id == "":
			add(IssueError, "layout", "tujuan pengguna tanpa id")
		case goals[id]:
			add(IssueError, "layout", "id tujuan %q ganda", g.ID)
		}
		goals[id] = true
		if strings.TrimSpace(g.Text) == "" {
			add(IssueError, "goal:"+g.ID, "isi tujuan belum diisi")
		}
		if g.Priority != "" && !slices.Contains(GoalPriorities, g.Priority) {
			add(IssueError, "goal:"+g.ID, "prioritas %q harus must, should atau could", g.Priority)
		}
		if g.Role != "" && len(roles) > 0 && !roles[strings.ToLower(strings.TrimSpace(g.Role))] {
			add(IssueWarning, "goal:"+g.ID, "peran %q tidak ada di users", g.Role)
		}
	}

	nav := map[string]bool{}
	navEmpty := len(l.Navigation) == 0
	if navEmpty {
		add(missing, "layout", "information architecture dan navigasi (navigation) belum diisi")
	}
	for _, g := range l.Navigation {
		where := "nav:" + strings.TrimSpace(g.Name)
		if strings.TrimSpace(g.Name) == "" {
			add(IssueError, "layout", "grup navigasi tanpa nama")
		}
		if len(g.Items) == 0 {
			add(IssueError, where, "grup navigasi tanpa tujuan")
		}
		for _, it := range g.Items {
			it.Page = strings.TrimSpace(it.Page)
			switch {
			case strings.TrimSpace(it.Label) == "":
				add(IssueError, where, "item navigasi tanpa label")
			case it.Page == "":
				add(IssueError, where, "item %q tanpa halaman", it.Label)
			case !slices.ContainsFunc(l.Pages, func(pg LayoutPage) bool { return pg.ID == it.Page }):
				add(IssueError, where, "item %q menuju halaman %q yang tidak ada", it.Label, it.Page)
			}
			nav[it.Page] = true
		}
	}

	decisions := map[string]int{}
	for i, d := range l.Decisions {
		where := fmt.Sprintf("decision:%d", i+1)
		subject := strings.TrimSpace(d.Subject)
		switch {
		case subject == "":
			add(IssueError, where, "subjek keputusan belum diisi (halaman, section, komponen atau *)")
		case !subjects[subject] && !subjects[strings.ToLower(subject)]:
			add(IssueError, where, "subjek %q bukan halaman, section, atau komponen", subject)
		}
		decisions[subject]++
		if strings.TrimSpace(d.Choice) == "" {
			add(IssueError, where, "pilihan desain (choice) belum diisi")
		}
		if len(d.Why) == 0 {
			add(IssueError, where, "alasan (why) belum diisi")
		}
		if !slices.Contains(DecisionSources, d.Source) {
			add(IssueError, where, "source %q harus %s", d.Source, strings.Join(DecisionSources, ", "))
		}
		if len(d.Basis) == 0 {
			add(missing, where, "dasar keputusan (basis) belum diisi: sebut tujuan, alur, entity, permission, platform atau token")
		}
		for _, b := range d.Basis {
			b = strings.TrimSpace(b)
			if b == "" {
				continue
			}
			if !known[strings.ToLower(b)] {
				add(IssueError, where, "basis %q tidak ada di blueprint (tujuan, alur, fitur, requirement, entity, permission, platform, token atau halaman)", b)
			}
		}
	}

	entities := designEntities(p)
	for _, pg := range l.Pages {
		where := "page:" + pg.ID
		if strings.TrimSpace(pg.Goal) == "" {
			lvl := missing
			if pg.Existing {
				lvl = IssueWarning
			}
			add(lvl, where, "tujuan pengguna halaman (goal) belum diisi")
		} else if len(l.Goals) > 0 && !goals[strings.ToLower(strings.TrimSpace(pg.Goal))] {
			add(IssueError, where, "goal %q tidak ada di daftar goals", pg.Goal)
		}
		if n := decisions[pg.ID]; n == 0 {
			lvl := missing
			if pg.Existing {
				lvl = IssueWarning
			}
			add(lvl, where, "belum ada keputusan desain (decisions) yang menjelaskan pilihan halaman ini")
		}
		for j, f := range pg.Fields {
			fw := fmt.Sprintf("%s/field:%s", where, f.Field)
			switch {
			case strings.TrimSpace(f.Field) == "":
				add(IssueError, where, "field ke-%d tanpa nama", j+1)
			case !slices.Contains(FieldPresentations, f.Presentation):
				add(IssueError, fw, "presentation %q harus %s", f.Presentation, strings.Join(FieldPresentations, ", "))
			}
			checkFieldEntity(p, entities, fw, f.Field, add)
		}
		out = append(out, pageAdvice(p, l, pg)...)
	}

	// Every page must be reachable from the navigation or from another page.
	if !navEmpty {
		reach := map[string]bool{}
		var walk func(string)
		walk = func(id string) {
			if reach[id] {
				return
			}
			reach[id] = true
			for _, pg := range l.Pages {
				if pg.ID != id {
					continue
				}
				for _, ln := range pg.Links {
					walk(ln.To)
				}
			}
		}
		for id := range nav {
			walk(id)
		}
		// A page may be entered from a shell (login, settings) without a link.
		for _, pg := range l.Pages {
			if !reach[pg.ID] && !pg.Existing && !pg.AccessIsPublic() {
				add(IssueWarning, "page:"+pg.ID, "halaman tidak dapat dicapai dari navigasi atau link halaman lain")
			}
		}
	}
	return out
}

// AccessIsPublic reports whether the page needs no signed-in user.
func (pg LayoutPage) AccessIsPublic() bool { return pg.Access == "public" }

// designEntities indexes the blueprint entities by lowercase name.
func designEntities(p *Project) map[string][]string {
	out := map[string][]string{}
	if p.Blueprint == nil {
		return out
	}
	for _, e := range p.Blueprint.Database.Entities {
		name := strings.ToLower(strings.TrimSpace(e.Name))
		if name == "" {
			continue
		}
		var fields []string
		for _, f := range e.Fields {
			fields = append(fields, strings.ToLower(strings.TrimSpace(f)))
		}
		out[name] = fields
	}
	return out
}

func checkFieldEntity(_ *Project, entities map[string][]string, where, field string, add func(string, string, string, ...any)) {
	entity, name, ok := strings.Cut(field, ".")
	if !ok {
		return // a page-local label, not an entity field
	}
	fields, known := entities[strings.ToLower(strings.TrimSpace(entity))]
	if !known {
		add(IssueWarning, where, "entity %q tidak ada di blueprint data", entity)
		return
	}
	if len(fields) == 0 {
		return
	}
	if !slices.Contains(fields, strings.ToLower(strings.TrimSpace(name))) {
		add(IssueWarning, where, "field %q tidak tercatat pada entity %s", name, entity)
	}
}

// designBasis lists every id a design decision may point at.
func designBasis(p *Project, l *LayoutSpec, rules *DesignRules) map[string]bool {
	out := map[string]bool{}
	add := func(v string) {
		if v = strings.ToLower(strings.TrimSpace(v)); v != "" {
			out[v] = true
		}
	}
	for _, g := range l.Goals {
		add(g.ID)
	}
	for _, u := range l.Users {
		add(u.Role)
	}
	if p.Discovery != nil {
		for _, r := range p.Discovery.Roles {
			add(r.Name)
		}
	}
	for _, pg := range l.Pages {
		add(pg.ID)
	}
	for _, fl := range p.Flows {
		add(fl.ID)
		for _, perm := range fl.Permissions {
			add(perm)
		}
	}
	for _, f := range p.Features {
		add(f.ID)
	}
	for _, r := range p.Requirements {
		add(r.ID)
	}
	for _, t := range p.UIPlatforms() {
		add(t)
	}
	if p.Blueprint != nil {
		for _, e := range p.Blueprint.Database.Entities {
			add(e.Name)
			for _, f := range e.Fields {
				add(e.Name + "." + f)
			}
		}
		for _, perm := range p.Blueprint.Authz.Permissions {
			add(perm)
		}
	}
	if rules != nil {
		for _, g := range rules.Tokens.Groups() {
			for _, tok := range *g.Tokens {
				add(tok.Name)
			}
		}
	}
	return out
}

// pageAdvice reports contextual design risks: a layout that is unlikely to
// serve the page's tasks on its platform. They are warnings, not schema
// errors, so the designer can justify an intentional exception.
func pageAdvice(p *Project, l *LayoutSpec, pg LayoutPage) []Issue {
	var out []Issue
	add := func(where, format string, a ...any) {
		out = append(out, Issue{Level: IssueWarning, Where: where, Message: fmt.Sprintf(format, a...)})
	}
	where := "page:" + pg.ID
	mobile := pg.Platform == PlatformAndroid || pg.Platform == PlatformIOS
	if mobile && pg.Pattern == "table" {
		for _, s := range pg.Sections {
			for _, line := range s.Content {
				if cols := strings.Count(line, ",") + strings.Count(line, "|") + 1; cols > 4 {
					add(where, "tabel %d kolom di layar %s: pertimbangkan list atau kartu dengan informasi prioritas", cols, pg.Platform)
					break
				}
			}
		}
	}
	if pg.Pattern == "form" && len(pg.Fields) > 8 {
		add(where, "form memuat %d field: kelompokkan per bagian atau pecah menjadi wizard", len(pg.Fields))
	}
	if !pg.Existing && pg.PrimaryAction == "" && slices.Contains([]string{"table", "list", "detail", "dashboard", "form", "wizard"}, pg.Pattern) {
		add(where, "aksi utama belum ditentukan untuk pattern %s", pg.Pattern)
	}
	if !pg.Existing && len(pg.Data) == 0 && slices.Contains([]string{"table", "list", "detail", "dashboard", "report"}, pg.Pattern) {
		add(where, "halaman menampilkan data tetapi daftar data (data) kosong")
	}
	for _, fl := range p.Flows {
		if !slices.Contains(fl.PageIDs, pg.ID) || len(fl.Steps) < 5 {
			continue
		}
		allForm := len(fl.PageIDs) > 0
		for _, id := range fl.PageIDs {
			i := slices.IndexFunc(l.Pages, func(x LayoutPage) bool { return x.ID == id })
			if i < 0 || l.Pages[i].Pattern != "form" {
				allForm = false
				break
			}
		}
		if allForm {
			add(where, "alur %s punya %d langkah tetapi seluruh halamannya form: pertimbangkan wizard atau stepper", fl.ID, len(fl.Steps))
		}
	}
	for _, fl := range p.Flows {
		if !slices.Contains(fl.PageIDs, pg.ID) || len(fl.Permissions) == 0 {
			continue
		}
		if pg.Role == "" && pg.Access != "public" {
			add(where, "alur %s memakai permission %s tetapi halaman belum menyebut peran yang diizinkan", fl.ID, strings.Join(fl.Permissions, ", "))
		}
	}
	return out
}

func sectionsHave(secs []Section, state string) bool {
	return slices.ContainsFunc(secs, func(s Section) bool { return s.States[state] != "" })
}

// pageRestricted reports whether a page serves a role or a use case with
// permissions, so a permission-denied state is relevant.
func pageRestricted(p *Project, pg LayoutPage) bool {
	if strings.TrimSpace(pg.Role) != "" {
		return true
	}
	for _, fl := range p.Flows {
		if slices.Contains(fl.PageIDs, pg.ID) && len(fl.Permissions) > 0 {
			return true
		}
	}
	return false
}
