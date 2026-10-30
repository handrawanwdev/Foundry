package work

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

// designDetailPrefix marks an internal follow-up: pages landed without
// sections and Foundry asks only for those details.
const designDetailPrefix = "Fill in layout details for pages that still lack sections"

const designDetailBatch = 6

// DesignNeedsDetails reports whether a landed design still needs a detail
// round (rules without pages, or pages without sections).
func DesignNeedsDetails(p *core.Project) bool {
	return p != nil && p.UI.Written() && pagesNeedDetails(p.UI)
}

func pagesNeedDetails(d *core.UIDesign) bool {
	if d == nil {
		return false
	}
	if d.Layout == nil || len(d.Layout.Pages) == 0 {
		return d.Rules != nil
	}
	for _, pg := range d.Layout.Pages {
		if len(pg.Sections) == 0 {
			return true
		}
	}
	return false
}

func pagesWithoutSections(d *core.UIDesign) []core.LayoutPage {
	if d == nil || d.Layout == nil {
		return nil
	}
	var out []core.LayoutPage
	for _, pg := range d.Layout.Pages {
		if len(pg.Sections) == 0 {
			out = append(out, pg)
		}
	}
	return out
}

func designDetailRequestText(n int) string {
	return fmt.Sprintf("%s (%d)", designDetailPrefix, n)
}

func remainingDetailCount(d *core.UIDesign) int {
	if d == nil || d.Layout == nil || len(d.Layout.Pages) == 0 {
		if d != nil && d.Rules != nil {
			return -1 // pages themselves are missing
		}
		return 0
	}
	return len(pagesWithoutSections(d))
}

// designDetailPrompt asks only for sections/fields/states of pages that
// landed as inventory. Rules and page ids stay as they are.
func designDetailPrompt(p *core.Project) string {
	var b strings.Builder
	b.WriteString(`You are the UI/UX designer of Foundry. Fill in the missing layout details. Do not rewrite the design rules or replace page ids.
Write every human-readable text in the same language as the story.

PROJECT: ` + p.Name + "\nSTORY:\n" + p.Story + "\n\n")
	writeBinding(&b, p)
	if brief := uxBrief(p); brief != "" {
		b.WriteString(brief + "\n")
	}
	d := p.UI
	if d != nil && d.Rules != nil {
		b.WriteString("CURRENT DESIGN RULES:\n" + mustJSON(d.Rules) + "\n\n")
	}
	if d != nil && d.Layout != nil {
		var inv []string
		for _, pg := range d.Layout.Pages {
			inv = append(inv, fmt.Sprintf("%s (%s, %s, %s)", pg.ID, pg.Name, pg.Platform, pg.Pattern))
		}
		if len(inv) > 0 {
			b.WriteString("PAGE INVENTORY (ids for links): " + strings.Join(inv, "; ") + "\n\n")
		}
		if len(d.Layout.Shells) > 0 {
			b.WriteString("SHELLS:\n" + mustJSON(d.Layout.Shells) + "\n\n")
		}
	}
	fmt.Fprintf(&b, layoutRules+"\n", strings.Join(core.LayoutPatterns, ", "), strings.Join(core.UIStates, ", "), requiredStates())
	b.WriteString(outputDiscipline + "\n")
	if d != nil && d.Request != nil && d.Request.Rejected != "" {
		b.WriteString("YOUR LAST ANSWER WAS REFUSED, fix this: " + d.Request.Rejected + "\n\n")
	}

	missing := pagesWithoutSections(d)
	if len(missing) == 0 {
		b.WriteString(`The design has rules but no pages yet. Return the layout of every page the features need, each in full (sections, fields, states, links).

Reply with ONLY one JSON object in a ` + "```json" + ` block:
{"pages": [{"id": "antrean", "name": "Antrean", "platform": "web", "route": "/antrean", "pattern": "table", "purpose": "Saring", "sections": [{"id": "tabel", "name": "Tabel", "content": ["No, Status"]}], "states": {"loading": "", "empty": "", "error": ""}}], "shells": [{"id": "app", "name": "App", "platform": "web", "pattern": "sidebar", "regions": [{"id": "nav", "name": "Nav", "content": ["Menu"]}]}]}`)
		return b.String()
	}
	if len(missing) > designDetailBatch {
		missing = missing[:designDetailBatch]
	}
	b.WriteString(fmt.Sprintf("DETAIL PHASE — fill sections, fields, states and links for these %d pages. Return each page in full, keeping its id. Do not omit these pages.\n", len(missing)))
	b.WriteString(mustJSON(missing) + "\n\n")
	b.WriteString(`Reply with ONLY one JSON object in a ` + "```json" + ` block:
{"pages": [{"id": "antrean", "name": "Antrean", "platform": "web", "route": "/antrean", "pattern": "table", "purpose": "Saring", "goal": "GOAL-1", "shell": "app", "sections": [{"id": "tabel", "name": "Tabel", "content": ["No, Status"], "components": ["Button"]}], "fields": [{"field": "Appointment.status", "label": "Status", "presentation": "badge"}], "links": [{"to": "detail", "via": "Baris"}], "states": {"loading": "", "empty": "", "error": ""}}]}`)
	return b.String()
}

// applyDesignDetails merges page details onto a sketch that already landed.
func applyDesignDetails(p *core.Project, out designOut) error {
	if out.Rules == nil && len(out.Shells)+len(out.Pages)+len(out.RemoveShells)+len(out.RemovePages)+len(out.Flows) == 0 && !out.hasExperience() {
		return errors.New("tidak ada detail halaman")
	}
	next := cloneDesign(p.UI)
	if out.Rules != nil {
		next.Rules = completeRules(p, normRules(*out.Rules))
	} else if next.Rules == nil {
		next.Rules = defaultRules(p)
	}
	l := next.Layout
	if l == nil {
		l = &core.LayoutSpec{}
		next.Layout = l
	}
	if out.Users != nil {
		l.Users = normUsers(*out.Users)
	}
	if out.Goals != nil {
		l.Goals = normGoals(*out.Goals)
	}
	if out.Navigation != nil {
		l.Navigation = normNavigation(*out.Navigation)
	}
	if out.Decisions != nil {
		l.Decisions = mergeDesignDecisions(l.Decisions, *out.Decisions)
	}
	if out.Assumptions != nil {
		l.Assumptions = nonEmpty(*out.Assumptions)
	}
	if out.Unknowns != nil {
		l.Unknowns = nonEmpty(*out.Unknowns)
	}
	for _, sh := range out.Shells {
		sh = normShell(sh)
		if i := slices.IndexFunc(l.Shells, func(x core.LayoutShell) bool { return x.ID == sh.ID }); i >= 0 {
			if len(sh.Regions) == 0 && len(l.Shells[i].Regions) > 0 {
				continue
			}
			l.Shells[i] = sh
		} else {
			l.Shells = append(l.Shells, sh)
		}
	}
	filled := 0
	for _, pg := range out.Pages {
		pg = normPage(pg)
		if i := slices.IndexFunc(l.Pages, func(x core.LayoutPage) bool { return x.ID == pg.ID }); i >= 0 {
			if len(pg.Sections) == 0 && len(l.Pages[i].Sections) > 0 {
				continue
			}
			if len(pg.Sections) > 0 {
				filled++
			}
			l.Pages[i] = pg
		} else if len(pg.Sections) > 0 || pg.Name != "" {
			if len(pg.Sections) > 0 {
				filled++
			}
			l.Pages = append(l.Pages, pg)
		}
	}
	if filled == 0 && remainingDetailCount(p.UI) != -1 {
		return errors.New("tidak ada detail halaman: isi sections pada halaman yang masih kosong")
	}
	if remainingDetailCount(p.UI) == -1 && len(l.Pages) == 0 {
		return errors.New("rancangan UI tidak lengkap: pages wajib diisi")
	}
	flows := slices.Clone(p.Flows)
	for id, pages := range out.Flows {
		i := slices.IndexFunc(flows, func(f core.Flow) bool { return f.ID == id })
		if i < 0 {
			return fmt.Errorf("alur %q tidak ada", id)
		}
		flows[i].PageIDs = nil
		for _, pg := range pages {
			flows[i].PageIDs = append(flows[i].PageIDs, normID(pg))
		}
	}
	applyExisting(p, next)
	completeDesign(p, next)
	if len(l.Users) > 0 && len(l.Goals) > 0 && len(l.Navigation) > 0 && len(l.Decisions) > 0 {
		next.Format = core.UIDesignFormat
	}
	if pagesNeedDetails(next) {
		return commitDesignSketch(p, next, flows)
	}
	return commitDesign(p, next, flows)
}

// commitDesignSketch lands a partial design (rules + page inventory) so the
// next round only fills missing sections instead of regenerating everything.
func commitDesignSketch(p *core.Project, next *core.UIDesign, flows []core.Flow) error {
	check := *p
	check.Flows = flows
	if errs := core.Errors(check.DesignSketchIssues(next)); len(errs) > 0 {
		return &core.Invalid{Issues: errs}
	}
	n := remainingDetailCount(next)
	next.Request = &core.ChangeRequest{Text: designDetailRequestText(n)}
	next.Version++
	next.UpdatedAt = time.Now().UTC()
	p.UI, p.Flows = next, flows
	return nil
}

func salvageDesign(raw string) *designOut {
	out := &designOut{}
	if rules := salvageJSONValue[core.DesignRules](raw, `"rules"`); rules != nil {
		out.Rules = rules
	}
	if pages := salvageJSONArray[core.LayoutPage](raw, `"pages"`); len(pages) > 0 {
		out.Pages = pages
	}
	if shells := salvageJSONArray[core.LayoutShell](raw, `"shells"`); len(shells) > 0 {
		out.Shells = shells
	}
	if users := salvageJSONArray[core.UIUser](raw, `"users"`); len(users) > 0 {
		out.Users = &users
	}
	if goals := salvageJSONArray[core.UIGoal](raw, `"goals"`); len(goals) > 0 {
		out.Goals = &goals
	}
	if nav := salvageJSONArray[core.NavGroup](raw, `"navigation"`); len(nav) > 0 {
		out.Navigation = &nav
	}
	if dec := salvageJSONArray[core.DesignDecision](raw, `"decisions"`); len(dec) > 0 {
		out.Decisions = &dec
	}
	if out.Rules == nil && len(out.Pages) == 0 && len(out.Shells) == 0 {
		return nil
	}
	return out
}

func salvageJSONArray[T any](raw, key string) []T {
	var best []T
	for _, rest := range keyTails(raw, key) {
		j := strings.IndexByte(rest, '[')
		if j < 0 {
			continue
		}
		src := rest[j:]
		dec := json.NewDecoder(strings.NewReader(src))
		if _, err := dec.Token(); err != nil {
			continue
		}
		var out []T
		for dec.More() {
			var o T
			if err := dec.Decode(&o); err != nil {
				break
			}
			out = append(out, o)
		}
		if len(out) > len(best) {
			best = out
		}
	}
	return best
}

func salvageJSONValue[T any](raw, key string) *T {
	for _, rest := range keyTails(raw, key) {
		j := strings.IndexByte(rest, '{')
		if j < 0 {
			continue
		}
		src := rest[j:]
		var o T
		if json.NewDecoder(strings.NewReader(src)).Decode(&o) == nil {
			return &o
		}
		if json.Unmarshal([]byte(closeJSON(src)), &o) == nil {
			return &o
		}
	}
	return nil
}

func keyTails(raw, key string) []string {
	var tails []string
	for start := 0; start < len(raw); {
		i := strings.Index(raw[start:], key)
		if i < 0 {
			break
		}
		i += start
		tails = append(tails, raw[i+len(key):])
		start = i + len(key)
	}
	return tails
}

func defaultRules(p *core.Project) *core.DesignRules {
	r := &core.DesignRules{
		Source:     core.RulesNew,
		Direction:  "Jelas, konsisten, satu tugas utama per halaman",
		Principles: []string{"Satu tugas utama per halaman", "Istilah produk, bukan jargon UI", "Aksi destruktif minta konfirmasi"},
		UX:         []string{"Navigasi mengikuti tugas pengguna", "Validasi dekat field", "Empty state menjelaskan langkah berikutnya", "Error menyertakan cara pulih"},
		Tokens: core.DesignTokens{
			Colors: []core.Token{
				{Name: "surface.page", Value: "#f8fafc", Use: "latar halaman"},
				{Name: "surface.card", Value: "#ffffff", Use: "kartu dan panel"},
				{Name: "text.primary", Value: "#0f172a", Use: "teks utama"},
				{Name: "text.muted", Value: "#64748b", Use: "teks sekunder"},
				{Name: "border.default", Value: "#e2e8f0", Use: "garis pemisah"},
				{Name: "action.primary", Value: "#1f6feb", Use: "aksi utama"},
				{Name: "status.success", Value: "#15803d", Use: "sukses"},
				{Name: "status.warning", Value: "#b45309", Use: "peringatan"},
				{Name: "status.error", Value: "#b91c1c", Use: "error"},
				{Name: "focus.ring", Value: "#1d4ed8", Use: "fokus keyboard"},
			},
			Typography: []core.Token{
				{Name: "type.body", Value: "Inter 14px/20px 400", Use: "teks isi"},
				{Name: "type.title", Value: "Inter 20px/28px 600", Use: "judul halaman"},
				{Name: "type.label", Value: "Inter 12px/16px 500", Use: "label field"},
			},
			Spacing: []core.Token{
				{Name: "space.xs", Value: "4px"}, {Name: "space.sm", Value: "8px"},
				{Name: "space.md", Value: "16px"}, {Name: "space.lg", Value: "24px"},
			},
			Radius:      []core.Token{{Name: "radius.md", Value: "8px"}},
			Breakpoints: []core.Token{{Name: "breakpoint.tablet", Value: "768px"}, {Name: "breakpoint.desktop", Value: "1024px"}},
		},
		Components: []core.ComponentRule{
			{Name: "Button", Use: "aksi", Variants: []string{"primary", "secondary"}, Rules: []string{"disabled saat kirim"}},
		},
		Accessibility: []string{"Kontras WCAG 2.2 AA", "Fokus keyboard terlihat", "Kontrol punya label"},
		Responsive:    []string{"Form satu kolom di layar sempit", "Tabel boleh scroll horizontal"},
		Interaction:   []string{"Loading pada operasi lama", "Jangan hapus input saat gagal"},
	}
	if p != nil && p.Survey != nil {
		if ref := strings.TrimSpace(p.Survey.DesignReference); ref != "" {
			r.Reference = ref
		}
	}
	if p != nil && p.Structure != nil && p.Structure.Mode == core.StructureExisting {
		r.Source = core.RulesExisting
	}
	return r
}

func completeRules(p *core.Project, r *core.DesignRules) *core.DesignRules {
	if r == nil {
		return defaultRules(p)
	}
	def := defaultRules(p)
	if r.Source == "" {
		r.Source = def.Source
	}
	if r.Reference == "" {
		r.Reference = def.Reference
	}
	if len(r.Principles) == 0 {
		r.Principles = def.Principles
	}
	if len(r.UX) == 0 {
		r.UX = def.UX
	}
	if len(r.Accessibility) == 0 {
		r.Accessibility = def.Accessibility
	}
	if len(r.Responsive) == 0 {
		r.Responsive = def.Responsive
	}
	if len(r.Tokens.Colors) == 0 {
		r.Tokens.Colors = def.Tokens.Colors
	}
	if len(r.Tokens.Typography) == 0 {
		r.Tokens.Typography = def.Tokens.Typography
	}
	if len(r.Tokens.Spacing) == 0 {
		r.Tokens.Spacing = def.Tokens.Spacing
	}
	if len(r.Components) == 0 {
		r.Components = def.Components
	}
	if p != nil && len(r.Platforms) > 0 {
		if allowed := p.UXPlatforms(); len(allowed) > 0 {
			var keep []core.PlatformRules
			for _, pr := range r.Platforms {
				if slices.Contains(allowed, pr.Platform) {
					keep = append(keep, pr)
				}
			}
			r.Platforms = keep
		}
	}
	return r
}

func defaultPresentation(pattern string) string {
	switch pattern {
	case "table":
		return "table-column"
	case "list":
		return "list-item"
	case "cards", "dashboard":
		return "card-meta"
	case "form", "wizard", "auth", "settings":
		return "form-input"
	default:
		return "detail"
	}
}

func firstUIPlatform(p *core.Project) string {
	if p == nil {
		return core.PlatformWeb
	}
	if plats := p.UIPlatforms(); len(plats) > 0 {
		return plats[0]
	}
	return core.PlatformWeb
}

// ensurePageShells creates a minimal shell for every pages[].shell the
// designer named but did not emit. Designer-written shells always win.
func ensurePageShells(l *core.LayoutSpec) {
	if l == nil {
		return
	}
	have := map[string]bool{}
	for _, sh := range l.Shells {
		have[sh.ID] = true
	}
	for _, pg := range l.Pages {
		id := pg.Shell
		if id == "" || have[id] || !core.ValidID(id) {
			continue
		}
		if len(l.Shells) >= core.MaxLayoutShells {
			return
		}
		l.Shells = append(l.Shells, core.LayoutShell{ID: id, Pattern: defaultShellPattern(id)})
		have[id] = true
	}
}

func defaultShellPattern(id string) string {
	switch {
	case id == "auth" || strings.Contains(id, "auth"):
		return "auth"
	case strings.Contains(id, "nav") && !strings.Contains(id, "sidebar"):
		return "navbar"
	default:
		return "sidebar"
	}
}

// repairLayout fixes recoverable layout mistakes so a nearly-valid answer
// lands instead of a full LLM retry. Designer text always wins when valid.
func repairLayout(p *core.Project, d *core.UIDesign) {
	if d == nil || d.Layout == nil {
		return
	}
	l := d.Layout
	plat := firstUIPlatform(p)
	ensurePageShells(l)
	pageIDs := map[string]bool{}
	for i := range l.Pages {
		pg := &l.Pages[i]
		if pg.Name == "" {
			pg.Name = pg.ID
		}
		if pg.Purpose == "" {
			pg.Purpose = pg.Name
		}
		if pg.Pattern == "" {
			pg.Pattern = "detail"
		}
		if pg.Platform == "" || !slices.Contains(core.UIPlatform, pg.Platform) {
			pg.Platform = plat
		}
		pres := defaultPresentation(pg.Pattern)
		for j := range pg.Fields {
			if pg.Fields[j].Presentation == "" || !slices.Contains(core.FieldPresentations, pg.Fields[j].Presentation) {
				pg.Fields[j].Presentation = pres
			}
		}
		pageIDs[pg.ID] = true
	}
	for i := range l.Shells {
		sh := &l.Shells[i]
		if sh.Name == "" {
			sh.Name = sh.ID
		}
		if sh.Pattern == "" {
			sh.Pattern = "sidebar"
		}
		if sh.Platform != "" && !slices.Contains(core.UIPlatform, sh.Platform) {
			sh.Platform = plat
		}
		if len(sh.Regions) == 0 {
			sh.Regions = []core.Section{{
				ID:      "main",
				Name:    firstNonEmpty(sh.Name, "Utama"),
				Content: []string{firstNonEmpty(sh.Navigation, sh.Name, "Konten")},
			}}
		}
	}
	for i := range l.Pages {
		pg := &l.Pages[i]
		var links []core.PageLink
		for _, ln := range pg.Links {
			if ln.To == "" || ln.To == pg.ID || !pageIDs[ln.To] {
				continue
			}
			if ln.Via == "" {
				ln.Via = "Buka"
			}
			links = append(links, ln)
		}
		pg.Links = links
		if len(pg.States) > 0 {
			for k := range pg.States {
				if !slices.Contains(core.UIStates, k) {
					delete(pg.States, k)
				}
			}
		}
	}
}

func repairDecisions(p *core.Project, l *core.LayoutSpec, rules *core.DesignRules) {
	if l == nil {
		return
	}
	subjects := map[string]bool{"*": true}
	for _, pg := range l.Pages {
		subjects[pg.ID] = true
		subjects[strings.ToLower(pg.ID)] = true
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
	if rules != nil {
		for _, c := range rules.Components {
			subjects[strings.ToLower(c.Name)] = true
			subjects[c.Name] = true
		}
	}
	var out []core.DesignDecision
	for _, d := range l.Decisions {
		sub := strings.TrimSpace(d.Subject)
		if a, ok := decisionSubjectAliases[normID(sub)]; ok {
			sub = a
			d.Subject = sub
		}
		if sub == "" || !subjects[sub] && !subjects[strings.ToLower(sub)] {
			continue
		}
		out = append(out, d)
	}
	l.Decisions = out
	completePageDecisions(l)
	sanitizeDecisions(p, l, rules)
}
