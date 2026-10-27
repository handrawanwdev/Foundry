package work

import (
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// UIPreset is a reusable, framework-agnostic UI design profile: the design
// language an external coding agent applies when the product has screens.
// A preset is policy, not code: it seeds the design prompt, and the AI writes
// the concrete design-rules and layout-spec for this project. Presets never
// name a framework or UI library; the target stack decides those.
type UIPreset struct {
	ID          string
	Name        string
	Description string
	// Match are lowercase keywords that select this preset when the product
	// type is generic and no preset is chosen explicitly.
	Match []string
	// Direction is the visual character in one sentence.
	Direction string
	// Principles are actionable, checkable rules of this design language.
	Principles []string
	// AppShell is the layout blueprint: frame, navigation, content area.
	AppShell []string
	// Components are the shared component conventions to reuse, not invent.
	Components []PresetComponent
	// PagePatterns are concrete rules for the common page patterns.
	PagePatterns []string
	// Interaction are the loading/empty/error/success/modal/toast/validation
	// and confirmation rules.
	Interaction []string
	// Tokens are the semantic token roles the design should define.
	Tokens []string
	// Accessibility and Responsive are the baseline rules.
	Accessibility []string
	Responsive    []string
	// QualityChecklist is the concrete verification checklist for the result.
	QualityChecklist []string
}

// PresetComponent is one shared component convention of a preset.
type PresetComponent struct {
	Name     string
	Use      string
	Variants []string
	Rules    []string
}

// Preset ids.
const (
	presetCleanSaaS     = "clean-saas"
	presetEnterprise    = "enterprise-admin"
	presetMinimalData   = "minimal-data"
	presetDeveloperTool = "developer-tool"
)

// Presets lists every built-in preset in selection order (most specific
// first). It is the reusable catalog; DetectPreset picks from it.
var Presets = []*UIPreset{
	cleanSaaSPreset(),
	enterpriseAdminPreset(),
	developerToolPreset(),
	minimalDataPreset(),
}

// PresetByID returns the preset with the id, nil when unknown.
func PresetByID(id string) *UIPreset {
	for _, p := range Presets {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// DetectPreset picks the design-language preset for a project. It follows the
// detected product type so the two stay consistent; a product type with its
// own platform guidance (mobile, e-commerce, content, communication,
// workflow) has no design-language preset and returns nil.
func DetectPreset(p *core.Project) *UIPreset {
	if p == nil {
		return PresetByID(presetMinimalData)
	}
	switch DetectProductType(p) {
	case ProductDashboard, ProductSaaS, ProductAnalytics:
		return PresetByID(presetCleanSaaS)
	case ProductAdmin:
		return PresetByID(presetEnterprise)
	case ProductMobile, ProductECommerce, ProductContent, ProductCommunication, ProductWorkflow:
		return nil
	}
	// ProductGeneric: developer tool when the signal is clear, otherwise the
	// minimal data-management baseline.
	if hasDeveloperSignal(p) {
		return PresetByID(presetDeveloperTool)
	}
	return PresetByID(presetMinimalData)
}

// hasDeveloperSignal reports whether the product reads as a developer-facing
// tool with screens (SDK, IDE, API console, playground, code editor, ...).
func hasDeveloperSignal(p *core.Project) bool {
	text := strings.ToLower(p.Story + " " + joinFeatures(p))
	return containsAny(text, []string{
		"developer tool", "dev tool", "developer", "sdk", "ide", "code editor",
		"playground", "api console", "api client", "debugger", "codegen", "package manager",
	})
}

// PresetPrompt renders a preset as design guidance for the design prompt.
func PresetPrompt(pr *UIPreset) string {
	if pr == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("UI DESIGN PRESET: " + pr.Name + "\n")
	b.WriteString(pr.Description + "\n\n")
	b.WriteString("VISUAL DIRECTION:\n" + pr.Direction + "\n\n")
	b.WriteString("PRINCIPLES:\n")
	for _, s := range pr.Principles {
		b.WriteString("- " + s + "\n")
	}
	b.WriteString("\nLAYOUT BLUEPRINT (app shell + hierarchy):\n")
	for _, s := range pr.AppShell {
		b.WriteString("- " + s + "\n")
	}
	if len(pr.Components) > 0 {
		b.WriteString("\nCOMPONENTS TO REUSE:\n")
		for _, c := range pr.Components {
			b.WriteString("- " + c.Name)
			if len(c.Variants) > 0 {
				b.WriteString(" (" + strings.Join(c.Variants, ", ") + ")")
			}
			b.WriteString(": " + c.Use)
			for _, r := range c.Rules {
				b.WriteString(" " + r)
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("\nPAGE PATTERN RULES:\n")
	for _, s := range pr.PagePatterns {
		b.WriteString("- " + s + "\n")
	}
	b.WriteString("\nINTERACTION RULES:\n")
	for _, s := range pr.Interaction {
		b.WriteString("- " + s + "\n")
	}
	b.WriteString("\nDESIGN TOKEN ROLES:\n")
	for _, s := range pr.Tokens {
		b.WriteString("- " + s + "\n")
	}
	b.WriteString("\nACCESSIBILITY:\n")
	for _, s := range pr.Accessibility {
		b.WriteString("- " + s + "\n")
	}
	b.WriteString("\nRESPONSIVE:\n")
	for _, s := range pr.Responsive {
		b.WriteString("- " + s + "\n")
	}
	b.WriteString("\nQUALITY CHECKLIST (verify before reporting done):\n")
	for _, s := range pr.QualityChecklist {
		b.WriteString("- [ ] " + s + "\n")
	}
	b.WriteString("\nThe preset is guidance, not a replacement for what the project already has: an existing design system wins over the preset. Adapt it to the platform and stack; do not force a framework or UI library.\n")
	return b.String()
}

func cleanSaaSPreset() *UIPreset {
	return &UIPreset{
		ID:          presetCleanSaaS,
		Name:        "Clean SaaS Dashboard",
		Description: "Produk SaaS multi-tenant atau dasbor yang mengutamakan kejelasan: pengguna baru memahami produk dalam hitungan detik, pengguna rutin menyelesaikan tugas tanpa gesekan.",
		Match:       []string{"saas", "dashboard", "subscription", "multi-tenant", "analytics", "metrics"},
		Direction:   "Clean, professional, low-chrome: generous whitespace, one clear hierarchy, and a single primary action per view.",
		Principles: []string{
			"One primary action per page; secondary actions never compete visually.",
			"Empty states educate: explain the feature and the next step, not just 'no data'.",
			"Progressive disclosure: show common options first, advanced on demand.",
			"Every KPI and chart answers a decision the user actually makes; no decorative numbers.",
			"Copy is specific and task-oriented ('Save changes', not 'OK').",
		},
		AppShell: []string{
			"Sidebar (collapsible) or top bar for primary navigation, grouped by the user's job, kept small (<=7 top-level items).",
			"Page header: title, short description, primary action on the right.",
			"Content area with one max-width per task: forms narrow, data tables wide.",
			"No second sidebar or topbar when the app shell already exists.",
		},
		Components: []PresetComponent{
			{Name: "Button", Use: "one primary per view", Variants: []string{"primary", "secondary", "ghost", "destructive"}, Rules: []string{"every variant has hover, focus, active, disabled states"}},
			{Name: "Card", Use: "group a small related set; never nest cards inside cards"},
			{Name: "DataTable", Use: "compare/operate on many records", Rules: []string{"sortable columns, row actions, bulk select when needed"}},
			{Name: "FormField", Use: "labeled input with inline validation"},
			{Name: "Modal", Use: "short side task without leaving context", Rules: []string{"title, managed focus, explicit close"}},
			{Name: "Toast", Use: "transient success/failure feedback only; never for critical info"},
			{Name: "Badge", Use: "status with a label, never color alone"},
			{Name: "EmptyState", Use: "explain the empty state and the next action"},
			{Name: "Skeleton", Use: "loading shape that does not shift layout"},
		},
		PagePatterns: []string{
			"dashboard: KPI cards with trend + sparkline only when decision-relevant; charts with title, axis labels, legend, tooltip.",
			"table/list: page header + one primary action; search/filter only when data or use case needs it; pagination/sort only when the data layer supports it.",
			"detail: header with record identity and relevant actions; group information by meaning, not API field order.",
			"form: group related fields; label always visible; validation near the field; preserve valid input on error.",
			"settings: group by area; explain the impact of unclear settings; confirm high-risk changes.",
			"auth: one task, minimal distraction; never leak sensitive detail in errors.",
		},
		Interaction: []string{
			"Loading state on any operation that takes time; prevent double submit.",
			"Don't destroy user input when a request fails.",
			"Confirm only risky or hard-to-undo actions.",
			"Don't assume success before the operation confirms it.",
			"Honor prefers-reduced-motion; no decorative animation.",
		},
		Tokens: []string{
			"Semantic color roles: color.bg, color.surface, color.text, color.text-muted, color.border, color.primary, color.success, color.warning, color.danger, color.focus.",
			"Typography scale with limited sizes: xs (helper), sm (label), md (body), lg (subheading), xl (page heading); weights normal/medium/bold.",
			"Spacing scale space.1..space.n; radius.sm/md/lg; shadow.sm/md; control height sm/md/lg; content max-width.",
			"Every visual value comes from a token; never a literal hex or px in a component.",
		},
		Accessibility: []string{
			"WCAG 2.2 AA contrast; visible focus; full keyboard use.",
			"Labels on inputs; alt on informative images; status never by color alone.",
			"Ordered headings that mirror content structure.",
		},
		Responsive: []string{
			"No horizontal overflow at narrow viewports.",
			"Tables use horizontal scroll or a readable alternative; never hide essential columns.",
			"Sidebar follows the app's existing responsive pattern.",
			"Test narrow, medium and wide viewports when a browser is available.",
		},
		QualityChecklist: []string{
			"Reuse existing components; no new dependency without a clear need.",
			"All visual values from tokens; no hardcoded color/size/spacing.",
			"Primary action present and obvious; loading/empty/error/success handled.",
			"Lint, typecheck, test, build pass where available; failures reported honestly.",
			"Visual check on relevant viewports when a browser is available; never claim visual pass without it.",
		},
	}
}

func enterpriseAdminPreset() *UIPreset {
	return &UIPreset{
		ID:          presetEnterprise,
		Name:        "Enterprise Admin",
		Description: "Sistem operasional internal dengan banyak peran, hak akses, dan audit: efisiensi dan kejelasan status di atas tampilan.",
		Match:       []string{"admin", "management", "enterprise", "rbac", "audit", "approval"},
		Direction:   "Dense and operational: information-rich layouts, compact rows, prominent status, minimal clicks for common tasks.",
		Principles: []string{
			"Minimize clicks: common tasks reachable in one or two actions.",
			"Status is prominent: badges, progress, timestamps; never inferred.",
			"Permission-driven UI: hide or disable actions the user cannot perform.",
			"Audit is visible: who did what, when, with what outcome.",
			"Search and filter are first-class: admin datasets are large.",
		},
		AppShell: []string{
			"Sidebar navigation grouped by operational area, with quick access to management sections.",
			"Page header with title, description, and the primary operational action.",
			"Content prioritizes density: compact tables over card grids for data.",
		},
		Components: []PresetComponent{
			{Name: "DataTable", Use: "the core pattern", Variants: []string{"sortable", "filterable", "selectable"}, Rules: []string{"bulk actions (select, delete, export), column selection by user task"}},
			{Name: "StatusBadge", Use: "state with label + optional icon", Rules: []string{"never color alone"}},
			{Name: "Form", Use: "scannable, grouped fields, inline validation"},
			{Name: "ConfirmDialog", Use: "destructive or bulk actions", Rules: []string{"state the impact"}},
			{Name: "AuditTimeline", Use: "who did what and when"},
			{Name: "PermissionGate", Use: "hide/disable what the role cannot do"},
			{Name: "FilterBar", Use: "search + filters above large lists"},
			{Name: "EmptyState", Use: "guide the user to create the first item"},
		},
		PagePatterns: []string{
			"table: dense rows, sortable columns, bulk actions, search/filter, pagination.",
			"form: group related fields; inline validation; preserve input on error.",
			"detail: record identity, status, related records, audit trail.",
			"approval: clear Approve/Reject/Request Changes actions.",
			"settings: grouped by area; permission and role management explicit.",
		},
		Interaction: []string{
			"Loading, empty, error and success states on every list and form.",
			"Confirm destructive and bulk actions; state the impact.",
			"Retry on transient errors; never destroy input.",
			"Double-submit prevention on every mutating action.",
		},
		Tokens: []string{
			"Semantic roles for status (success/warning/danger) with text labels.",
			"Compact spacing and control heights for dense tables; a density token if the app supports dense mode.",
			"Typography that stays readable at small sizes; a mono token for ids/codes if shown.",
		},
		Accessibility: []string{
			"WCAG 2.2 AA; keyboard for all core actions; visible focus.",
			"Bulk-select and row actions keyboard-reachable.",
			"Status readable by screen reader (label, not color only).",
		},
		Responsive: []string{
			"Tables keep key columns readable on narrow screens (horizontal scroll or priority columns).",
			"Filter bar stacks vertically on small screens.",
			"Dialogs never exceed the viewport.",
		},
		QualityChecklist: []string{
			"Reuse existing components and permission patterns.",
			"Bulk and destructive actions confirm and state impact.",
			"Audit trail present for admin mutations.",
			"Tests and build pass; failures reported honestly.",
			"Visual check on relevant viewports when available.",
		},
	}
}

func developerToolPreset() *UIPreset {
	return &UIPreset{
		ID:          presetDeveloperTool,
		Name:        "Developer Tool",
		Description: "Perkakas untuk developer (SDK, IDE, API console, playground, debugger): teknis, presisi, padat tapi mudah dibaca, keyboard-first.",
		Match:       []string{"developer", "sdk", "ide", "api console", "playground", "code editor", "debugger"},
		Direction:   "Technical and precise: monospace for data and identifiers, dense-but-readable layout, keyboard-first, minimal decoration.",
		Principles: []string{
			"Data and identifiers in monospace; prose in the UI font.",
			"Keyboard-first: every core action reachable without a mouse.",
			"Precision over polish: exact values, no decorative UI.",
			"Code, commands and identifiers are copyable and selectable.",
			"Errors state the problem and a recovery or reference, not a vague message.",
		},
		AppShell: []string{
			"App shell with a compact primary navigation; a contextual secondary area (file tree, endpoint list) where the tool needs it.",
			"Content favors split/master-detail for rapid switching between items.",
			"No marketing layout; this is a tool, not a showcase.",
		},
		Components: []PresetComponent{
			{Name: "CodeBlock", Use: "code with syntax highlight and copy button"},
			{Name: "Input", Use: "commands/identifiers with clear affordance"},
			{Name: "Tabs", Use: "parallel facets of one item"},
			{Name: "Table", Use: "structured data (logs, requests, results)"},
			{Name: "Tooltip", Use: "supplementary info, never the only place for critical info"},
			{Name: "Badge", Use: "method/status/version (GET, 200, v2) with label"},
			{Name: "Tree", Use: "nested navigation (files, endpoints)"},
			{Name: "EmptyState", Use: "tell the user what to run or connect first"},
		},
		PagePatterns: []string{
			"master-detail: list on one side, item detail on the other; keyboard switching.",
			"table: logs, requests, results; exact values, sortable.",
			"form: request builders, settings; validated inline.",
			"detail: reference pages (endpoint, type, config) with copyable ids.",
		},
		Interaction: []string{
			"Loading state for slow operations; keep the last result visible while re-running.",
			"Retry on failure; preserve input.",
			"Copy feedback near the copy action.",
			"Keyboard focus visible and tab order logical; common actions have shortcuts.",
		},
		Tokens: []string{
			"Font roles: font.ui (prose) and font.mono (code/ids/data).",
			"Semantic colors for status (success/warning/danger) and syntax accents.",
			"Compact spacing for dense panels; a theme-aware surface for code areas.",
		},
		Accessibility: []string{
			"WCAG 2.2 AA; visible focus; keyboard for core actions.",
			"Syntax highlight must not be the only signal of meaning.",
			"Copyable content reachable by keyboard and screen reader.",
		},
		Responsive: []string{
			"Master-detail collapses to a single column with a back action on narrow screens.",
			"Code blocks scroll horizontally without breaking the page.",
			"Toolbars wrap instead of overflowing.",
		},
		QualityChecklist: []string{
			"Reuse existing components and code-rendering approach.",
			"Identifiers and code copyable; monospace where appropriate.",
			"Keyboard path works for core actions.",
			"Tests and build pass; failures reported honestly.",
			"Visual check on relevant viewports when available.",
		},
	}
}

func minimalDataPreset() *UIPreset {
	return &UIPreset{
		ID:          presetMinimalData,
		Name:        "Minimal Data Management",
		Description: "Aplikasi pengelolaan data sederhana (CRUD): satu tugas per halaman, chrome minimal, daftar dan form polos tanpa dekorasi.",
		Match:       []string{"crud", "data", "management", "list", "record"},
		Direction:   "Sparse and focused: one task per page, plain lists and forms, minimal chrome, no decoration without a function.",
		Principles: []string{
			"One task per page; a screen exists for a task, not for an entity.",
			"Choose a pattern by task and data volume: table to compare, list to scan, form to input, detail to view one record.",
			"Do not turn every entity into a page, and do not make every page a dashboard.",
			"Show only the fields the task needs, not every field of an entity.",
			"No nested cards, decorative gradients, or ornament.",
		},
		AppShell: []string{
			"Simple app shell: top bar or sidebar with few destinations, then a content area.",
			"Page header with title and one primary action.",
			"Content width fits the task: forms narrow, tables wider.",
		},
		Components: []PresetComponent{
			{Name: "DataTable", Use: "compare/operate on records", Rules: []string{"sortable, row actions"}},
			{Name: "List", Use: "scan records with one dominant attribute"},
			{Name: "Form", Use: "create/edit with labeled, validated fields"},
			{Name: "DetailView", Use: "view one record with grouped fields"},
			{Name: "EmptyState", Use: "guide the user to create the first record"},
			{Name: "ConfirmDialog", Use: "destructive actions"},
			{Name: "Badge", Use: "status with label, never color alone"},
		},
		PagePatterns: []string{
			"table/list: header + primary action; search/filter only when the dataset needs it.",
			"detail: grouped by meaning; status and metadata consistent.",
			"form: grouped fields; inline validation; preserve input on error.",
			"settings: grouped by area; confirm high-risk changes.",
		},
		Interaction: []string{
			"Loading, empty, error and success states per page pattern.",
			"Confirm destructive actions; state the impact.",
			"Retry on failure; never destroy input.",
			"Double-submit prevention on forms.",
		},
		Tokens: []string{
			"Semantic color roles; typography scale; spacing scale; radius; shadow when elevation is meaningful.",
			"Status colors carry a label or icon, never color alone.",
		},
		Accessibility: []string{
			"WCAG 2.2 AA; visible focus; keyboard use.",
			"Labels on inputs; alt on informative images.",
			"Status readable by screen reader.",
		},
		Responsive: []string{
			"Tables degrade to scroll or list on narrow screens without losing essential columns.",
			"Forms become one column when two columns get too narrow.",
			"No horizontal overflow.",
		},
		QualityChecklist: []string{
			"Reuse existing components; no new dependency without need.",
			"All visual values from tokens.",
			"Loading/empty/error/success handled per page.",
			"Tests and build pass; failures reported honestly.",
			"Visual check on relevant viewports when available.",
		},
	}
}
