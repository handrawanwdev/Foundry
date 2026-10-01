package work

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

// Design intelligence: the decisions taken before a layout is composed.
// The designer AI states each page's intent and plan; Foundry derives the
// same decisions from the intent with the rules below (Decide), shows the
// rules to the designer as knowledge and checks the composed IR against
// them (LintPage). The IR stays the record; the renderer only draws it.

// Vocabulary of intent and plan.
var (
	Tasks  = []string{"authenticate", "enter", "configure", "search", "browse", "compare", "process", "monitor", "inspect", "analyze", "overview"}
	Levels = []string{"low", "medium", "high"}
	// LayoutStrategies: focused (one task in one column), sections
	// (grouped sections), list, table, master-detail (list + selected
	// record), workspace (navigation + primary work area + context panel),
	// dashboard (summary over today's work), steps (wizard).
	LayoutStrategies = []string{"focused", "sections", "list", "table", "master-detail", "workspace", "dashboard", "steps"}
	// ResponsiveStrategies: how the information architecture changes on a
	// small screen.
	ResponsiveStrategies = []string{"same", "stack", "table_to_list", "list_then_detail", "context_to_overlay"}
	Domains              = []string{"trading", "finance", "ticketing", "developer_tools", "admin_operations"}
)

// Decision is what the planner decides for one page.
type Decision struct {
	PageType   string   `json:"page_type"`
	Layout     string   `json:"layout_strategy"`
	Navigation string   `json:"navigation_strategy"`
	Density    string   `json:"density"`
	Responsive string   `json:"responsive_strategy"`
	Reasons    []string `json:"reasons"`
}

// PlanInput is what the planner decides from.
type PlanInput struct {
	Intent   core.PageIntent
	Platform string
	Domain   string
	HasNav   bool
}

// denseDomains work with many records at once and favour compact screens.
var denseDomains = map[string]bool{"trading": true, "ticketing": true, "developer_tools": true, "admin_operations": true}

// Decide applies the layout decision rules to one page intent.
func Decide(in PlanInput) Decision {
	it := in.Intent
	mobile := in.Platform == core.PlatformAndroid || in.Platform == core.PlatformIOS
	d := Decision{Responsive: "stack"}
	why := func(s string) { d.Reasons = append(d.Reasons, s) }
	heavy := it.InformationDensity == "high" || it.Comparison

	switch it.PrimaryTask {
	case "authenticate":
		d.PageType, d.Layout, d.Responsive = "auth", "focused", "same"
		why("satu tugas: form masuk adalah seluruh halaman")
	case "enter":
		d.PageType, d.Layout = "form", "focused"
		if it.Complexity == "high" {
			d.Layout = "sections"
			why("input panjang dikelompokkan per bagian")
		}
	case "configure":
		d.PageType, d.Layout = "settings", "sections"
	case "search":
		d.PageType, d.Layout = "search", "list"
	case "monitor":
		d.PageType, d.Layout = "monitoring", "dashboard"
		if heavy || it.Frequency == "high" {
			d.Layout = "workspace"
			why("pemantauan terus-menerus dengan banyak data: workspace padat dengan status selalu terlihat")
		}
	case "analyze":
		d.PageType, d.Layout = "report", "sections"
	case "overview":
		d.PageType, d.Layout = "dashboard", "dashboard"
	case "process":
		d.PageType, d.Layout, d.Responsive = "master-detail", "master-detail", "list_then_detail"
		if it.Frequency == "high" {
			d.PageType = "workspace"
			why("tugas berulang dari antrean: daftar dan detail terpilih tampil bersamaan")
		}
	case "compare", "browse":
		d.PageType, d.Layout = "list", "list"
		if heavy || it.PrimaryTask == "compare" {
			d.PageType, d.Layout, d.Responsive = "table", "table", "table_to_list"
			why("membandingkan banyak record: tabel, bukan kartu terpisah")
		}
	case "inspect":
		d.PageType, d.Layout = "detail", "sections"
		if in.Domain == "developer_tools" && heavy {
			d.Layout = "workspace"
			why("developer tool: hierarki resource, detail dan log dalam satu workspace")
		}
	default:
		d.PageType, d.Layout = "detail", "sections"
	}

	switch {
	case it.PrimaryTask == "authenticate":
		d.Density = "spacious"
	case it.InformationDensity == "high" || it.Frequency == "high" && it.Complexity != "low":
		d.Density = "compact"
		why("sering dipakai dan padat informasi: compact dan efisien")
	case it.Frequency == "low" && it.InformationDensity != "high":
		d.Density = "spacious"
		why("jarang dipakai: dipandu dan lega")
	case denseDomains[in.Domain] && it.Frequency != "low":
		d.Density = "compact"
	default:
		d.Density = "comfortable"
	}
	if it.PrimaryTask == "enter" && d.Density == "compact" {
		d.Density = "comfortable" // input needs room for labels and errors
	}

	switch {
	case !in.HasNav || d.PageType == "auth":
		d.Navigation = "none"
	case mobile:
		d.Navigation = "bottom"
	default:
		d.Navigation = "side"
	}

	if mobile {
		switch d.Layout {
		case "master-detail", "workspace", "table":
			d.Layout = "list"
			why("layar HP tidak muat dua panel: daftar membuka halaman detail")
		}
		d.Responsive = "same"
	} else if d.Layout == "workspace" && d.Responsive == "stack" {
		d.Responsive = "context_to_overlay"
	}
	return d
}

// ---------- Domain ----------

var domainSignals = map[string]*regexp.Regexp{
	"trading":          regexp.MustCompile(`(?i)\b(trading|trader|saham|order ?book|broker|kripto|crypto|forex|watchlist|candlestick|bursa|portofolio|portfolio)\b`),
	"finance":          regexp.MustCompile(`(?i)\b(keuangan|finance|financial|akuntansi|accounting|saldo|balance|transaksi|transaction|anggaran|budget|invoice|faktur|rekening|ledger|pengeluaran)\b`),
	"ticketing":        regexp.MustCompile(`(?i)\b(tiket|ticket|ticketing|helpdesk|help ?desk|keluhan|complaint|sla|eskalasi|escalation|customer support|layanan pelanggan)\b`),
	"developer_tools":  regexp.MustCompile(`(?i)\b(developer|deploy|deployment|repository|repo|log|logs|sdk|cli|devops|api key|ci/cd|pipeline|konfigurasi server)\b`),
	"admin_operations": regexp.MustCompile(`(?i)\b(admin|operasional|operations|inventori|inventory|gudang|warehouse|logistik|logistics|back ?office|persetujuan|approval)\b`),
}

// DetectDomain picks the domain the product text names most; "" when none.
func DetectDomain(p *core.Project) string {
	var b strings.Builder
	b.WriteString(p.Story)
	if d := p.Discovery; d != nil {
		b.WriteString(" " + d.Summary + " " + d.Business + " " + strings.Join(d.Workflows, " "))
	}
	for _, f := range p.Features {
		b.WriteString(" " + f.Name + " " + f.Description)
	}
	text := b.String()
	best, n := "", 0
	for _, dom := range Domains {
		if c := len(domainSignals[dom].FindAllString(text, -1)); c > n {
			best, n = dom, c
		}
	}
	return best
}

// domainProfiles are constraints and preferences, never a fixed layout.
var domainProfiles = map[string]string{
	"trading":          "high information density, continuous monitoring, comparison of instruments, fast actions (order entry always reachable), status and price changes always visible; working objects: market data, watchlist, order book, chart, order entry, positions, executions.",
	"finance":          "summary first (balances), then transactions, clear financial hierarchy (totals above detail), progressive detail; amounts right-aligned with currency; working objects: accounts, balances, transactions, categories, budgets.",
	"ticketing":        "queue with priority, status, SLA and assignee scannable at once; the selected ticket's conversation and customer context next to the queue; working objects: queue, ticket, conversation, customer, assignee, SLA.",
	"developer_tools":  "workspace with persistent navigation and resource hierarchy, detail panels, logs and configuration; keyboard friendly, compact; working objects: projects, resources, deployments, logs, settings.",
	"admin_operations": "dense operational tables with filters and bulk actions, status visibility, approval queues; working objects: records, statuses, approvals, assignees.",
}

// ---------- Knowledge for the designer ----------

// pagePatterns are the structure of each page type: context, not a
// template.
var pagePatterns = []string{
	"auth: brand + one form + one primary action + recovery link.",
	"list: title with its action, filter only when the data needs it, the records.",
	"table: dense comparable records in columns, sort/filter, bulk actions when needed; on mobile a list that opens the detail.",
	"detail: identity/header (name, status) + primary information + actions + supporting information.",
	"form: grouped fields + submit/cancel (drawn by the form).",
	"dashboard: the user's work today first; at most 3 numbers that drive a decision.",
	"workspace: persistent navigation + primary work area + contextual information panel.",
	"master-detail: master list + the selected record's detail, side by side on desktop.",
	"settings: sections of related fields.",
	"search: query + results; recent searches when empty.",
	"monitoring: live status of many items, problems first, always-visible status.",
	"report: question-driven sections, a headline number with change, only charts that answer a question.",
}

// designKnowledgeRules are the rules Decide and LintPage apply, as the
// designer reads them.
var designKnowledgeRules = []string{
	"IF the user performs the task frequently AND compares multiple records THEN prefer a dense table, list, master-detail or workspace over large card-based composition. WHY: scanning and comparing needs aligned rows, cards hide the comparison.",
	"IF the page has one primary task THEN make it visually dominant (the plan's primary_focus first in reading order, one action.primary) and avoid competing primary actions. WHY: the purpose must be clear within 2-3 seconds.",
	"IF frequency is high THEN compact and efficient (density compact, persistent navigation, actions next to the data); IF frequency is low THEN guided and spacious. WHY: experts want speed, occasional users need guidance.",
	"IF information density is high THEN workspace, table or list; medium THEN structured sections; low THEN a focused single column.",
	"IF the user processes items from a queue (review and respond) THEN master-detail: the queue and the selected item together on desktop; on mobile the list opens the detail page (devices.mobile hides one of them).",
	"IF a table cannot stay readable on a phone (more than 4 columns or long cells) THEN swap it to a list on mobile (devices.mobile.swap) and move detail to the detail page, never just shrink the table.",
	"IF a context panel (customer info, metadata, history) supports the primary work THEN it is secondary: narrower column on desktop, hidden or behind an overlay on mobile.",
	"IF the page is a form THEN do not use density compact: labels and errors need room.",
	"Hierarchy is explicit: primary information, secondary information, supporting information/metadata, primary action, secondary actions, navigation. Never give everything the same visual weight.",
}

var antiSlop = "Every visual element must have a functional, informational, navigational or hierarchical reason, stated in plan.reasons; no reason, leave it out. " +
	"Avoid: excessive cards, generic SaaS dashboard, giant hero, meaningless statistics (more than 3 stats), random icons, decorative gradients, glassmorphism, " +
	"panels in panels or every section on a surface, shadows on anything that is not an overlay, duplicate actions, excessive whitespace, fake charts, decorative sections."

// designKnowledge is the knowledge section of the design prompt.
func designKnowledge(domain string) string {
	var b strings.Builder
	b.WriteString("DESIGN KNOWLEDGE (rules with their reason; patterns are context, not templates):\n")
	if prof := domainProfiles[domain]; prof != "" {
		fmt.Fprintf(&b, "Domain detected: %s. Constraints and preferences: %s\n", domain, prof)
	}
	b.WriteString("Page patterns:\n- " + strings.Join(pagePatterns, "\n- ") + "\n")
	b.WriteString("Decision rules:\n- " + strings.Join(designKnowledgeRules, "\n- ") + "\n")
	b.WriteString("Anti-slop: " + antiSlop + "\n")
	return b.String()
}

const designProcess = `DESIGN PROCESS, per page, before composing its layout (write the JSON fields in this order):
1. Understand: "intent" = {"primary_task": one of ` + "authenticate|enter|configure|search|browse|compare|process|monitor|inspect|analyze|overview" + `,
   "frequency": low|medium|high (how often the role does it), "complexity": low|medium|high, "information_density": low|medium|high, "comparison": true when the user compares many records}.
   The user is "role" and the goal is "purpose".
2. Classify: "kind" from the workflow, not from the page name.
3. Plan: "plan" = {"layout_strategy": focused|sections|list|table|master-detail|workspace|dashboard|steps,
   "responsive_strategy": same|stack|table_to_list|list_then_detail|context_to_overlay,
   "primary_focus": the component id that carries the primary task, "secondary_content": [component ids], "supporting_content": [component ids: metadata, history, context],
   "reasons": {"component-id": "why it is on the page"} for every non-nav component}.
4. Decide: "density" from frequency and complexity, "layout.nav" from the navigation the workflow needs, and the mobile composition from responsive_strategy.
5. Compose: the "layout" tree that records these decisions: primary_focus first in reading order (or the wide side of a split), supporting content narrower or later.
`

// ---------- Normalization ----------

func normIntent(in *core.PageIntent) *core.PageIntent {
	if in == nil {
		return nil
	}
	low := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
	out := core.PageIntent{Comparison: in.Comparison}
	if t := low(in.PrimaryTask); slices.Contains(Tasks, t) {
		out.PrimaryTask = t
	}
	for _, f := range []struct{ in, out *string }{{&in.Frequency, &out.Frequency}, {&in.Complexity, &out.Complexity}, {&in.InformationDensity, &out.InformationDensity}} {
		if v := low(*f.in); slices.Contains(Levels, v) {
			*f.out = v
		}
	}
	if out == (core.PageIntent{}) {
		return nil
	}
	return &out
}

// normPlan keeps known strategies and component ids; a plan without a
// strategy is derived from the intent.
func normPlan(pl *core.PagePlan, pg core.Page) *core.PagePlan {
	ids := map[string]bool{}
	for _, c := range pg.Components {
		if c.ID != "" {
			ids[c.ID] = true
		}
	}
	out := core.PagePlan{}
	if pl != nil {
		low := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
		if v := low(pl.Layout); slices.Contains(LayoutStrategies, v) {
			out.Layout = v
		}
		if v := low(pl.Responsive); slices.Contains(ResponsiveStrategies, v) {
			out.Responsive = v
		}
		if ids[strings.TrimSpace(pl.Primary)] {
			out.Primary = strings.TrimSpace(pl.Primary)
		}
		known := func(in []string) []string {
			var o []string
			for _, id := range in {
				if id = strings.TrimSpace(id); ids[id] && id != out.Primary && !slices.Contains(o, id) {
					o = append(o, id)
				}
			}
			return o
		}
		out.Secondary = known(pl.Secondary)
		out.Supporting = slices.DeleteFunc(known(pl.Supporting), func(id string) bool { return slices.Contains(out.Secondary, id) })
		for k, v := range pl.Reasons {
			if v = strings.TrimSpace(v); v != "" && k != "" {
				if out.Reasons == nil {
					out.Reasons = map[string]string{}
				}
				out.Reasons[strings.TrimSpace(k)] = v
			}
		}
	}
	if pg.Intent != nil && (out.Layout == "" || out.Responsive == "") {
		d := Decide(PlanInput{Intent: *pg.Intent, Platform: pg.Platform, HasNav: hasNav(pg)})
		if out.Layout == "" {
			out.Layout = d.Layout
		}
		if out.Responsive == "" {
			out.Responsive = d.Responsive
		}
	}
	if out.Layout == "" && out.Responsive == "" && out.Primary == "" && len(out.Secondary)+len(out.Supporting)+len(out.Reasons) == 0 {
		return nil
	}
	return &out
}

func hasNav(pg core.Page) bool {
	return slices.ContainsFunc(pg.Components, func(c core.PageComponent) bool { return c.Type == "nav" })
}

// ---------- Lint: the design rules checked on the composed IR ----------

var densityRank = map[string]int{"compact": 0, "comfortable": 1, "spacious": 2}

// LintDesign checks the given pages (all when ids is empty).
func LintDesign(p *core.Project, ids []string) []core.QAIssue {
	if p.Design == nil {
		return nil
	}
	var out []core.QAIssue
	theme := currentTheme(p)
	for _, pg := range p.Design.Pages {
		if len(ids) == 0 || slices.Contains(ids, pg.ID) {
			out = append(out, LintPage(pg, p.Design.Domain)...)
			out = append(out, ThemeLint(pg, theme)...)
		}
	}
	return out
}

// LintPage lists where one page's IR breaks the design rules. Findings are
// concrete: category, component and the change that fixes it.
func LintPage(pg core.Page, domain string) []core.QAIssue {
	l := pg.Layout
	if l == nil {
		return nil
	}
	var out []core.QAIssue
	add := func(rule, sev, cat, comp, dev, problem, why, fix string) {
		out = append(out, core.QAIssue{PageID: pg.ID, Severity: sev, Category: cat, Component: comp, Device: dev,
			Problem: problem, Why: why, Fix: fix, Source: "lint", Rule: rule})
	}
	comps := map[string]core.PageComponent{}
	count := map[string]int{}
	for _, c := range pg.Components {
		comps[c.ID] = c
		count[c.Type]++
	}
	base := core.BaseDevice(pg)
	webLike := base == "desktop"
	resolved := map[string]core.PageLayout{}
	swaps := map[string]map[string]string{}
	for _, dev := range core.Devices {
		resolved[dev], swaps[dev] = l.Resolve(dev)
	}
	visible := func(dev, id string) bool {
		found := false
		resolved[dev].Root.Walk(func(n core.LayoutNode, _ *core.LayoutNode) { found = found || n.Component == id })
		return found
	}
	plan := pg.Plan
	if plan == nil {
		plan = &core.PagePlan{}
	}

	// Hierarchy: the primary focus is visible and leads; the primary action
	// has a focal point.
	if id := plan.Primary; id != "" {
		for _, dev := range core.Devices {
			if !visible(dev, id) && !(dev != base && (plan.Responsive == "list_then_detail" || plan.Responsive == "context_to_overlay")) {
				add("primary_hidden", "major", "hierarchy", id, dev, fmt.Sprintf("Fokus utama %q tidak tampil di %s.", id, dev),
					"tugas utama halaman harus selalu terlihat", fmt.Sprintf("Hapus %q dari devices.%s.hide atau ganti primary_focus.", id, dev))
			}
		}
		if before := leadingSupport(resolved[base].Root, id, plan.Supporting); before != "" {
			add("support_before_primary", "minor", "hierarchy", before, base, fmt.Sprintf("Konten pendukung %q tampil sebelum fokus utama %q.", before, id),
				"urutan baca harus mengikuti prioritas informasi", fmt.Sprintf("Pindahkan %q setelah %q, atau ke kolom samping.", before, id))
		}
	}
	if a := pg.PrimaryAction; a != nil && a.Label != "" {
		focal := slices.ContainsFunc(pg.Components, func(c core.PageComponent) bool { return c.Emphasis == "action.primary" }) ||
			count["form"] > 0 && strings.EqualFold(FormSubmit(pg), a.Label)
		if !focal {
			add("no_primary_focal", "major", "hierarchy", "", base, fmt.Sprintf("Aksi utama %q tidak punya titik fokus visual.", a.Label),
				"satu workflow butuh satu aksi fokus", fmt.Sprintf("Beri emphasis action.primary pada tombol %q.", a.Label))
		}
	}
	m := PageMetrics(pg)
	if n := m["primary_actions"]; n > 1 {
		add("duplicate_primary_action", "major", "hierarchy", "", base, fmt.Sprintf("Aksi utama %q tampil %d kali.", pg.PrimaryAction.Label, n),
			"satu workflow punya satu aksi utama; aksi ganda membuat user ragu mana yang benar", "Hapus tombol duplikat; sisakan satu aksi utama.")
	}

	// Spacing follows density.
	l.Root.Walk(func(n core.LayoutNode, _ *core.LayoutNode) {
		if n.Component != "" {
			return
		}
		switch {
		case pg.Density == "compact" && (n.Gap == "space.lg" || n.Gap == "space.xl"):
			add("spacing_density", "minor", "spacing", n.ID, "", fmt.Sprintf("Node %q memakai gap %s di halaman compact.", n.Key(), n.Gap),
				"halaman padat butuh jarak rapat agar banyak data terlihat", fmt.Sprintf("Ubah gap node %q ke space.sm atau space.md.", n.Key()))
		case pg.Density == "spacious" && (n.Gap == "space.none" || n.Gap == "space.xs") && len(n.Children) > 1:
			add("spacing_density", "minor", "spacing", n.ID, "", fmt.Sprintf("Node %q memakai gap %s di halaman spacious.", n.Key(), n.Gap),
				"halaman yang dipandu butuh jarak untuk memisahkan langkah", fmt.Sprintf("Ubah gap node %q ke space.md atau lebih.", n.Key()))
		}
	})

	// Density and composition follow the intent.
	if in := pg.Intent; in != nil {
		d := Decide(PlanInput{Intent: *in, Platform: pg.Platform, Domain: domain, HasNav: hasNav(pg)})
		if r, ok := densityRank[pg.Density]; ok && abs(r-densityRank[d.Density]) == 2 {
			add("density_mismatch", "major", "density", "", "", fmt.Sprintf("Density %s bertentangan dengan intent (frekuensi %s, kepadatan informasi %s).", pg.Density, in.Frequency, in.InformationDensity),
				strings.Join(d.Reasons, "; "), "Ubah density ke "+d.Density+".")
		}
		cardGrid := ""
		l.Root.Walk(func(n core.LayoutNode, _ *core.LayoutNode) {
			if n.Type != "grid" || cardGrid != "" {
				return
			}
			cards := 0
			for _, k := range n.Children {
				if comps[k.Component].Type == "card" {
					cards++
				}
			}
			if cards >= 3 {
				cardGrid = n.Key()
			}
		})
		if cardGrid != "" && (in.Comparison || in.Frequency == "high" && in.InformationDensity == "high") && count["table"]+count["list"] == 0 {
			add("cards_for_comparison", "major", "composition", cardGrid, "", fmt.Sprintf("Record yang dibandingkan disusun sebagai kartu terpisah di node %q.", cardGrid),
				"perbandingan banyak record butuh baris yang sejajar", "Ganti kartu dengan satu table (atau list) berisi record tersebut.")
		}
	}
	if webLike && (plan.Layout == "master-detail" || plan.Layout == "workspace") && !hasNode(resolved[base].Root, "split", "sidebar") {
		add("layout_mismatch", "major", "composition", "", base, fmt.Sprintf("Strategi %s, tetapi layout %s tidak punya split atau sidebar.", plan.Layout, base),
			"daftar dan detail (atau area kerja dan konteks) harus tampil bersamaan di desktop", "Susun root dengan node split atau sidebar berisi dua area tersebut.")
	}

	// Navigation: a nav component must have a place.
	if hasNav(pg) && l.Frame == "app" && (l.Nav == "" || l.Nav == "none") {
		add("nav_hidden", "major", "navigation", "", base, "Halaman punya komponen nav tetapi layout.nav none, jadi menu tidak tampil.",
			"navigasi persisten dibutuhkan untuk berpindah antarhalaman", "Set layout.nav ke side (desktop) atau bottom (mobile).")
	}

	// Responsive: the information architecture changes, not only the size.
	if webLike && pg.Platform != core.PlatformDesktop {
		for _, c := range pg.Components {
			if c.Type == "table" && len(c.Columns) > 4 && visible("mobile", c.ID) && swaps["mobile"][c.ID] != "list" {
				add("table_not_swapped", "major", "responsive", c.ID, "mobile", fmt.Sprintf("Tabel %q dengan %d kolom hanya diperkecil di mobile.", c.ID, len(c.Columns)),
					"tabel lebar tidak terbaca di layar HP", fmt.Sprintf("Tambahkan devices.mobile.swap {%q: \"list\"}.", c.ID))
			}
		}
		if plan.Responsive == "table_to_list" && !slices.ContainsFunc(pg.Components, func(c core.PageComponent) bool { return swaps["mobile"][c.ID] == "list" }) {
			add("table_to_list_missing", "major", "responsive", "", "mobile", "Strategi table_to_list, tetapi tidak ada tabel yang ditukar ke list di mobile.",
				"keputusan responsif harus tercatat di IR", "Tambahkan devices.mobile.swap untuk tabel utama.")
		}
		if plan.Responsive == "list_then_detail" && plan.Primary != "" && len(plan.Secondary) > 0 &&
			visible("mobile", plan.Primary) && slices.ContainsFunc(plan.Secondary, func(id string) bool { return visible("mobile", id) }) {
			add("mobile_stacked_detail", "major", "responsive", plan.Primary, "mobile", "Strategi list_then_detail, tetapi mobile masih menumpuk daftar dan detail di satu layar.",
				"di HP daftar membuka detail; menumpuk keduanya membuat halaman sangat panjang", "Sembunyikan salah satu di devices.mobile.hide (detail dibuka dari daftar).")
		}
	}

	if m["mobile_split_remaining"] > 0 {
		add("mobile_split", "major", "responsive", "", "mobile", "Split atau sidebar desktop masih tampil di mobile.",
			"dua kolom tidak muat di layar HP", "Ubah node tersebut lewat devices.mobile.nodes.<id>.type ke stack.")
	}

	// AI slop.
	if n := m["duplicate_actions"]; n > 0 {
		add("duplicate_action", "minor", "ai_slop", "", "", fmt.Sprintf("%d aksi tampil lebih dari sekali.", n),
			"aksi yang sama di dua tempat menambah elemen tanpa fungsi baru", "Hapus tombol yang mengulang aksi yang sudah ada.")
	}
	if n := m["nested_surfaces"]; n > 0 {
		add("nested_surface", "minor", "ai_slop", "", "", fmt.Sprintf("%d panel (surface) di dalam panel lain.", n),
			"panel di dalam panel menambah garis dan jarak tanpa memisahkan informasi", "Hapus surface dari node bagian dalam.")
	}
	if count["stat"] > 3 {
		add("too_many_stats", "major", "ai_slop", "", "", fmt.Sprintf("%d angka statistik di satu halaman.", count["stat"]),
			"angka hanya untuk yang mendorong keputusan, maksimal 3", "Hapus stat yang tidak dipakai untuk mengambil keputusan.")
	}
	decorative := 0
	for _, c := range pg.Components {
		if c.Type == "card" && c.Target == "" && len(c.Items) == 0 {
			decorative++
		}
	}
	if decorative >= 3 {
		add("decorative_cards", "major", "ai_slop", "", "", fmt.Sprintf("%d kartu tanpa isi atau tujuan.", decorative),
			"kartu dekoratif tidak membawa informasi atau aksi", "Hapus kartu tersebut atau ganti dengan konten yang dipakai user.")
	}
	surfaces, shadows := 0, 0
	l.Root.Walk(func(n core.LayoutNode, _ *core.LayoutNode) {
		if n.Component != "" {
			return
		}
		if n.Surface != "" {
			surfaces++
		}
		if n.Elevation != "" && n.Elevation != "elevation.none" && n.Type != "overlay" {
			shadows++
		}
	})
	if surfaces >= 3 && surfaces >= len(l.Root.Children) {
		add("surface_everywhere", "minor", "ai_slop", "", "", "Setiap bagian diletakkan di panel (surface).",
			"panel hanya untuk mengelompokkan yang perlu dipisah", "Hapus surface dari bagian yang tidak perlu dipisah.")
	}
	if shadows > 1 {
		add("extra_elevation", "minor", "ai_slop", "", "", fmt.Sprintf("%d node non-overlay memakai elevation.", shadows),
			"bayangan hanya untuk lapisan di atas halaman", "Ubah elevation node tersebut ke elevation.none.")
	}
	if len(plan.Reasons) > 0 {
		var none []string
		for _, c := range pg.Components {
			if c.Type != "nav" && plan.Reasons[c.ID] == "" {
				none = append(none, c.ID)
			}
		}
		if len(none) > 0 {
			add("missing_reason", "minor", "ai_slop", none[0], "", "Komponen tanpa alasan: "+strings.Join(none, ", ")+".",
				"setiap elemen harus punya alasan fungsional, informasional, navigasi atau hierarki", "Tulis alasannya di plan.reasons atau hapus komponennya.")
		}
	}
	return out
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func hasNode(n core.LayoutNode, types ...string) bool {
	found := false
	n.Walk(func(x core.LayoutNode, _ *core.LayoutNode) {
		found = found || x.Component == "" && slices.Contains(types, x.Type)
	})
	return found
}

// leadingSupport returns a supporting component that comes before the
// primary focus in a vertical node, else "".
func leadingSupport(root core.LayoutNode, primary string, supporting []string) string {
	found := ""
	root.Walk(func(n core.LayoutNode, _ *core.LayoutNode) {
		if found != "" || n.Component != "" || n.Type == "split" || n.Type == "sidebar" || n.Type == "row" || n.Type == "grid" {
			return
		}
		at := slices.IndexFunc(n.Children, func(k core.LayoutNode) bool { return contains(k, primary) })
		for i := 0; i < at; i++ {
			for _, s := range supporting {
				if contains(n.Children[i], s) {
					found = s
					return
				}
			}
		}
	})
	return found
}

func contains(n core.LayoutNode, id string) bool {
	found := false
	n.Walk(func(x core.LayoutNode, _ *core.LayoutNode) { found = found || x.Component == id })
	return found
}
