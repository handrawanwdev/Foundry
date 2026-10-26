// Package uicheck evaluates implemented screens against the UI/UX blueprint.
// Foundry does not render pages and does not take a screenshot itself: a
// browser automation tool, the coding agent or a person submits measured
// snapshots and screenshot files, and this package checks them
// deterministically (contrast, overflow, spacing/typography tokens, labels,
// headings, touch targets, primary action, evidence). No evidence means the
// page is untested, never passed.
package uicheck

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

var (
	ErrNoDesign = errors.New("rancangan UI belum disusun; tidak ada halaman untuk dievaluasi")
	ErrNoPage   = errors.New("halaman tidak ada di layout spec")
	ErrNoProof  = errors.New("evaluasi visual memerlukan minimal satu snapshot")
)

// MaxFindings bounds one page audit so a broken page cannot flood the report.
const MaxFindings = 300

// Review checks one page's snapshots and records the audit on the project.
func Review(p *core.Project, in core.VisualReviewIn, now time.Time) (core.PageAudit, error) {
	if p == nil || p.UI == nil || p.UI.Layout == nil || len(p.UI.Layout.Pages) == 0 {
		return core.PageAudit{}, ErrNoDesign
	}
	var pg *core.LayoutPage
	for i := range p.UI.Layout.Pages {
		if p.UI.Layout.Pages[i].ID == strings.TrimSpace(in.Page) {
			pg = &p.UI.Layout.Pages[i]
			break
		}
	}
	if pg == nil {
		return core.PageAudit{}, fmt.Errorf("%w: %q", ErrNoPage, in.Page)
	}
	if len(in.Snapshots) == 0 {
		return core.PageAudit{}, ErrNoProof
	}
	for _, s := range in.Snapshots {
		st := strings.TrimSpace(s.State)
		if st != "" && st != "default" && !slices.Contains(core.UIStates, st) {
			return core.PageAudit{}, fmt.Errorf("state %q tidak dikenal (%s)", st, strings.Join(core.UIStates, ", "))
		}
	}
	now = now.UTC()
	audit := inspect(p.Dir, p.UI.Rules, pg, in, now)
	var prev core.PageAudit
	if p.Visual != nil {
		prev = p.Visual.Pages[pg.ID]
	}
	audit.Previous = prev.Status
	audit.Resolved = resolved(prev.Findings, audit.Findings)

	if p.Visual == nil {
		p.Visual = &core.VisualAudit{Pages: map[string]core.PageAudit{}}
	}
	if p.Visual.Pages == nil {
		p.Visual.Pages = map[string]core.PageAudit{}
	}
	p.Visual.Pages[pg.ID] = audit
	p.Visual.Version++
	p.Visual.UpdatedAt = now
	p.Visual.Source = audit.Source
	p.Visual.Reviews = append(p.Visual.Reviews, core.VisualReviewLog{At: now, Page: pg.ID, Source: audit.Source,
		Status: audit.Status, Findings: len(audit.Findings), Viewports: audit.Viewports})
	if n := len(p.Visual.Reviews); n > 100 {
		p.Visual.Reviews = p.Visual.Reviews[n-100:]
	}
	return audit, nil
}

// Summary counts the measured pages against the design.
func Summary(p *core.Project) core.VisualSummary {
	if p == nil || p.UI == nil || p.UI.Layout == nil {
		return core.VisualSummary{}
	}
	return p.Visual.Summary(p.UI.Layout.Pages)
}

// inspect runs every check for one page.
func inspect(dir string, rules *core.DesignRules, pg *core.LayoutPage, in core.VisualReviewIn, now time.Time) core.PageAudit {
	a := core.PageAudit{Page: pg.ID, URL: strings.TrimSpace(in.URL), Source: strings.TrimSpace(in.Source),
		CheckedAt: now, Findings: []core.VisualFinding{}}
	if a.Source == "" {
		a.Source = "agent"
	}
	status := map[string]string{}
	add := func(f core.VisualFinding) {
		if len(a.Findings) >= MaxFindings {
			return
		}
		a.Findings = append(a.Findings, f)
	}
	mark := func(criterion, st string) {
		if st == core.VisualPass {
			if status[criterion] == "" {
				status[criterion] = st
			}
			return
		}
		if st == core.VisualFail || status[criterion] == "" {
			status[criterion] = st
		}
	}

	var spacing, fonts []float64
	if rules != nil {
		spacing = lengthTokens(rules.Tokens.Spacing)
		fonts = lengthTokens(rules.Tokens.Typography)
	}

	evidence := false
	widths := map[float64]bool{}

	for _, snap := range in.Snapshots {
		label := snapshotLabel(snap)
		view := strings.TrimSpace(snap.Viewport.Name)
		if view == "" {
			view = fmt.Sprintf("%.0fx%.0f", snap.Viewport.Width, snap.Viewport.Height)
		}
		a.Viewports = appendUnique(a.Viewports, view)
		if snap.Viewport.Width > 0 {
			widths[snap.Viewport.Width] = true
		}
		if rel, ok := screenshot(dir, snap.Screenshot); ok {
			evidence = true
			a.Screenshots = appendUnique(a.Screenshots, rel)
		} else {
			add(core.VisualFinding{Page: pg.ID, Criterion: "evidence", Severity: core.SeverityHigh,
				Message:  "screenshot tidak tersedia atau bukan berkas gambar di dalam project",
				Evidence: label + " " + strings.TrimSpace(snap.Screenshot),
				Fix:      "Simpan screenshot setiap viewport di dalam project dan kirim path relatifnya."})
		}
		if snap.Viewport.Width <= 0 || snap.Viewport.Height <= 0 {
			add(core.VisualFinding{Page: pg.ID, Criterion: "evidence", Severity: core.SeverityMedium,
				Message: "ukuran viewport tidak diisi", Evidence: label,
				Fix: "Isi viewport.width dan viewport.height dalam CSS pixel."})
		}

		var headings []int
		hasHeading := false
		interactive := false
		contrastChecked := false
		nodes := visible(snap.Nodes)
		styles := map[string]core.VisualNode{}
		for _, n := range snap.Nodes {
			styles[n.Path] = n
		}
		if len(nodes) > 0 {
			mark("overflow", core.VisualPass)
			mark("accessibility", core.VisualPass)
			mark("interaction", core.VisualPass)
		}
		for _, n := range nodes {
			tag := strings.ToLower(n.Tag)
			if lvl := headingLevel(tag); lvl > 0 {
				headings = append(headings, lvl)
				hasHeading = true
			}
			if snap.Viewport.Width > 0 {
				if n.Bounds.X < -1 || n.Bounds.X+n.Bounds.Width > snap.Viewport.Width+1 {
					mark("overflow", core.VisualFail)
					add(core.VisualFinding{Page: pg.ID, Criterion: "overflow", Severity: core.SeverityHigh,
						Message:  fmt.Sprintf("elemen melebihi lebar viewport (%.0f > %.0f px)", n.Bounds.X+n.Bounds.Width, snap.Viewport.Width),
						Evidence: label + " " + n.Path, Fix: "Bungkus konten yang panjang, izinkan wrap, atau sembunyikan kolom sekunder."})
				}
			}
			if n.Interactive() {
				interactive = true
				if snap.Viewport.Width <= 600 && (n.Bounds.Width < 44 || n.Bounds.Height < 44) {
					mark("interaction", core.VisualFail)
					add(core.VisualFinding{Page: pg.ID, Criterion: "interaction", Severity: core.SeverityMedium,
						Message:  fmt.Sprintf("target sentuh %s hanya %.0fx%.0f px; minimal 44x44 px", nameOf(n), n.Bounds.Width, n.Bounds.Height),
						Evidence: label + " " + n.Path, Fix: "Perbesar padding atau tinggi minimum target sentuh menjadi 44 px."})
				}
				if !accessibleName(n) {
					mark("accessibility", core.VisualFail)
					add(core.VisualFinding{Page: pg.ID, Criterion: "accessibility", Severity: core.SeverityHigh,
						Message:  "kontrol interaktif tanpa nama yang dapat dibaca (label/aria-label/teks)",
						Evidence: label + " " + n.Path, Fix: "Tambahkan label atau aria-label yang menjelaskan aksi."})
				}
			}
			if tag == "img" {
				if _, ok := n.Attrs["alt"]; !ok {
					mark("accessibility", core.VisualFail)
					add(core.VisualFinding{Page: pg.ID, Criterion: "accessibility", Severity: core.SeverityHigh,
						Message: "gambar tanpa atribut alt", Evidence: label + " " + n.Path,
						Fix: "Isi alt dengan deskripsi, atau alt=\"\" bila dekoratif."})
				}
			}
			if strings.TrimSpace(n.Text) != "" {
				if fg := parseColor(n.Style.Color); fg.ok {
					if bg, ok := backgroundOf(n, styles, snap); ok {
						contrastChecked = true
						ratio := contrastRatio(fg, bg)
						limit := 4.5
						if n.Style.FontSize >= 24 || (n.Style.FontSize >= 18.66 && n.Style.FontWeight >= 700) {
							limit = 3.0
						}
						if ratio < limit {
							mark("contrast", core.VisualFail)
							add(core.VisualFinding{Page: pg.ID, Criterion: "contrast", Severity: core.SeverityHigh,
								Message:  fmt.Sprintf("kontras teks %.1f:1 di bawah %.1f:1", ratio, limit),
								Evidence: label + " " + n.Path, Fix: "Gelapkan teks atau terangkan latar sampai memenuhi WCAG AA."})
						}
					}
				}
			}
		}
		if contrastChecked {
			mark("contrast", core.VisualPass)
		}
		if len(nodes) > 0 {
			if !hasHeading {
				mark("hierarchy", core.VisualFail)
				add(core.VisualFinding{Page: pg.ID, Criterion: "hierarchy", Severity: core.SeverityMedium,
					Message: "tidak ada heading (h1-h6) untuk menyatakan hierarki halaman", Evidence: label,
					Fix: "Beri h1 untuk tujuan halaman dan h2/h3 untuk bagian."})
			} else {
				mark("hierarchy", core.VisualPass)
				checkHeadings(pg, label, headings, add, mark)
			}
			checkAlignment(pg, label, nodes, add, mark)
		}
		if !interactive && pg.PrimaryAction == "" {
			// A read-only page can legitimately have no control.
			mark("interaction", core.VisualUntested)
		}
		tokenFindings(pg, label, nodes, spacing, fonts, add, mark)
	}

	if evidence {
		mark("evidence", core.VisualPass)
	} else {
		mark("evidence", core.VisualUntested)
	}
	// Responsive is only proven with more than one viewport.
	if small, large := minMax(widths); small > 0 && small <= 600 && large >= 1000 {
		mark("responsive", core.VisualPass)
	} else {
		mark("responsive", core.VisualUntested)
	}
	checkPrimaryAction(pg, in.Snapshots, add, mark)
	stateChecks(pg, in.Snapshots, dir, add, mark)

	a.Findings = dedupe(a.Findings)
	sortFindings(a.Findings)
	a.Criteria = criteria(status, pg)
	a.Status = overall(a.Findings, a.Criteria, evidence)
	return a
}

func visible(nodes []core.VisualNode) []core.VisualNode {
	var out []core.VisualNode
	for _, n := range nodes {
		if !n.Hidden() {
			out = append(out, n)
		}
	}
	return out
}

func headingLevel(tag string) int {
	if len(tag) == 2 && tag[0] == 'h' && tag[1] >= '1' && tag[1] <= '6' {
		return int(tag[1] - '0')
	}
	return 0
}

func checkHeadings(pg *core.LayoutPage, label string, hs []int, add func(core.VisualFinding), mark func(string, string)) {
	h1 := 0
	prev := 0
	for _, h := range hs {
		if h == 1 {
			h1++
		}
		if prev > 0 && h > prev+1 {
			mark("hierarchy", core.VisualFail)
			add(core.VisualFinding{Page: pg.ID, Criterion: "hierarchy", Severity: core.SeverityLow,
				Message: fmt.Sprintf("tingkat heading melompat dari h%d ke h%d", prev, h), Evidence: label,
				Fix: "Urutkan heading tanpa melewati tingkat (h1 -> h2 -> h3)."})
		}
		prev = h
	}
	switch {
	case h1 == 0:
		mark("hierarchy", core.VisualFail)
		add(core.VisualFinding{Page: pg.ID, Criterion: "hierarchy", Severity: core.SeverityMedium,
			Message: "tidak ada h1 sebagai judul halaman", Evidence: label,
			Fix: "Tambahkan satu h1 yang menyatakan tujuan halaman."})
	case h1 > 1:
		mark("hierarchy", core.VisualFail)
		add(core.VisualFinding{Page: pg.ID, Criterion: "hierarchy", Severity: core.SeverityMedium,
			Message: fmt.Sprintf("ada %d h1 pada satu halaman", h1), Evidence: label,
			Fix: "Pakai satu h1; jadikan judul bagian h2."})
	}
}

func checkAlignment(pg *core.LayoutPage, label string, nodes []core.VisualNode, add func(core.VisualFinding), mark func(string, string)) {
	groups := map[string][]core.VisualNode{}
	for _, n := range nodes {
		if !isLeftAligned(n.Tag) {
			continue
		}
		parent := n.Path
		if i := strings.LastIndex(parent, ">"); i > 0 {
			parent = strings.TrimSpace(parent[:i])
		}
		groups[parent] = append(groups[parent], n)
	}
	checked := false
	for _, group := range groups {
		if len(group) < 2 {
			continue
		}
		checked = true
		lefts := make([]float64, len(group))
		for i, n := range group {
			lefts[i] = n.Bounds.X
		}
		base := lefts[0]
		for _, x := range lefts[1:] {
			if d := abs(x - base); d > 0.9 && d <= 8 {
				mark("alignment", core.VisualFail)
				add(core.VisualFinding{Page: pg.ID, Criterion: "alignment", Severity: core.SeverityLow,
					Message: fmt.Sprintf("perataan kiri bergeser %.0f px di antara elemen bersaudara", d), Evidence: label,
					Fix: "Samakan left edge elemen dalam satu grup dengan grid/token spacing."})
				break
			}
		}
	}
	if checked {
		mark("alignment", core.VisualPass)
	}
}

func isLeftAligned(tag string) bool {
	switch strings.ToLower(strings.TrimSpace(tag)) {
	case "p", "label", "input", "select", "textarea":
		return true
	}
	return headingLevel(strings.ToLower(strings.TrimSpace(tag))) > 0
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func tokenFindings(pg *core.LayoutPage, label string, nodes []core.VisualNode, spacing, fonts []float64,
	add func(core.VisualFinding), mark func(string, string)) {
	if len(spacing) > 0 {
		applicable := false
		for _, n := range nodes {
			var bad []string
			for _, v := range append(append([]float64{}, n.Style.Padding...), n.Style.Margin...) {
				if v <= 0 {
					continue
				}
				applicable = true
				if !inSet(v, spacing) {
					bad = append(bad, fmt.Sprintf("%.0fpx", v))
				}
			}
			if n.Style.Gap > 0 {
				applicable = true
				if !inSet(n.Style.Gap, spacing) {
					bad = append(bad, fmt.Sprintf("gap %.0fpx", n.Style.Gap))
				}
			}
			if len(bad) == 0 {
				continue
			}
			mark("spacing", core.VisualFail)
			add(core.VisualFinding{Page: pg.ID, Criterion: "spacing", Severity: core.SeverityLow,
				Message: "spasi di luar token: " + strings.Join(unique(bad), ", "), Evidence: label + " " + n.Path,
				Fix: "Ganti nilai spasi dengan token spacing terdekat."})
		}
		if applicable {
			mark("spacing", core.VisualPass)
		}
	}
	if len(fonts) > 0 {
		checked := false
		for _, n := range nodes {
			if n.Style.FontSize <= 0 || strings.TrimSpace(n.Text) == "" {
				continue
			}
			checked = true
			if inSet(n.Style.FontSize, fonts) {
				mark("typography", core.VisualPass)
				continue
			}
			mark("typography", core.VisualFail)
			add(core.VisualFinding{Page: pg.ID, Criterion: "typography", Severity: core.SeverityLow,
				Message:  fmt.Sprintf("ukuran font %.0fpx tidak berasal dari token tipografi", n.Style.FontSize),
				Evidence: label + " " + n.Path, Fix: "Pakai salah satu token tipografi dari design rules."})
		}
		_ = checked
	}
}

func checkPrimaryAction(pg *core.LayoutPage, snaps []core.VisualSnapshot, add func(core.VisualFinding), mark func(string, string)) {
	action := strings.TrimSpace(pg.PrimaryAction)
	if action == "" {
		return
	}
	want := strings.ToLower(action)
	for _, s := range snaps {
		for _, n := range s.Nodes {
			if n.Hidden() {
				continue
			}
			if strings.Contains(strings.ToLower(n.Text), want) || strings.Contains(strings.ToLower(n.Attrs["aria-label"]), want) ||
				strings.Contains(strings.ToLower(n.Attrs["value"]), want) {
				mark("interaction", core.VisualPass)
				return
			}
		}
	}
	mark("interaction", core.VisualFail)
	add(core.VisualFinding{Page: pg.ID, Criterion: "interaction", Severity: core.SeverityMedium,
		Message: "aksi utama " + strconvQuote(action) + " tidak ditemukan pada snapshot", Evidence: "semua viewport",
		Fix: "Tampilkan aksi utama dengan label yang sama di layar, atau perbarui primary_action bila label berubah."})
}

func stateChecks(pg *core.LayoutPage, snaps []core.VisualSnapshot, dir string, add func(core.VisualFinding), mark func(string, string)) {
	seen := map[string]bool{}
	for _, s := range snaps {
		st := strings.TrimSpace(s.State)
		if st == "" {
			st = "default"
		}
		if _, ok := screenshot(dir, s.Screenshot); !ok {
			continue
		}
		seen[st] = true
	}
	for _, st := range core.RequiredStates[pg.Pattern] {
		if seen[st] {
			mark("state:"+st, core.VisualPass)
			continue
		}
		mark("state:"+st, core.VisualUntested)
		add(core.VisualFinding{Page: pg.ID, Criterion: "state:" + st, Severity: core.SeverityLow,
			Message: "belum ada bukti screenshot untuk state " + st,
			Fix:     "Kirim snapshot dengan state=\"" + st + "\" beserta screenshot-nya."})
	}
}

// criteria expands the checked statuses into the reviewer-visible list.
func criteria(status map[string]string, pg *core.LayoutPage) []core.VisualCriterion {
	type def struct{ id, title, severity string }
	defs := []def{
		{"evidence", "Bukti screenshot tersedia", core.SeverityHigh},
		{"hierarchy", "Hierarki judul dan aksi utama", core.SeverityMedium},
		{"alignment", "Perataan elemen", core.SeverityLow},
		{"overflow", "Tidak ada konten yang meluap", core.SeverityHigh},
		{"contrast", "Kontras teks memenuhi WCAG AA", core.SeverityHigh},
		{"typography", "Tipografi memakai token", core.SeverityLow},
		{"spacing", "Spasi dan alignment memakai token", core.SeverityLow},
		{"accessibility", "Label, alt dan nama kontrol", core.SeverityHigh},
		{"interaction", "Aksi utama dan target sentuh", core.SeverityMedium},
		{"responsive", "Diuji pada lebih dari satu viewport", core.SeverityMedium},
	}
	for _, st := range core.RequiredStates[pg.Pattern] {
		defs = append(defs, def{"state:" + st, "State " + st + " terlihat", core.SeverityLow})
	}
	var out []core.VisualCriterion
	for _, d := range defs {
		st := status[d.id]
		if st == "" {
			st = core.VisualUntested
		}
		detail := ""
		switch st {
		case core.VisualUntested:
			detail = "belum dapat diuji dari snapshot yang dikirim"
		case core.VisualPass:
			detail = "lulus"
		case core.VisualFail:
			detail = "ada temuan"
		}
		out = append(out, core.VisualCriterion{ID: d.id, Title: d.title, Status: st, Severity: d.severity, Detail: detail})
	}
	return out
}

func overall(findings []core.VisualFinding, criteria []core.VisualCriterion, evidence bool) string {
	if !evidence {
		return core.VisualUntested
	}
	for _, f := range findings {
		if f.Severity == core.SeverityHigh || f.Severity == core.SeverityCritical {
			return core.VisualFail
		}
	}
	for _, c := range criteria {
		if c.Status != core.VisualPass {
			return core.VisualPartial
		}
	}
	if len(findings) > 0 {
		return core.VisualPartial
	}
	return core.VisualPass
}

// backgroundOf resolves the nearest opaque background from the node, its
// ancestors in the same snapshot, then the snapshot background.
func backgroundOf(n core.VisualNode, styles map[string]core.VisualNode, snap core.VisualSnapshot) (color, bool) {
	if c := parseColor(n.Style.Background); c.opaque() {
		return c, true
	}
	parts := strings.Split(n.Path, ">")
	for i := len(parts) - 1; i > 0; i-- {
		parent := strings.TrimSpace(strings.Join(parts[:i], ">"))
		if p, ok := styles[parent]; ok {
			if c := parseColor(p.Style.Background); c.opaque() {
				return c, true
			}
		}
	}
	if c := parseColor(snap.Background); c.opaque() {
		return c, true
	}
	return color{}, false
}

func accessibleName(n core.VisualNode) bool {
	if strings.TrimSpace(n.Text) != "" {
		return true
	}
	for _, k := range []string{"aria-label", "aria-labelledby", "title", "alt", "placeholder", "label"} {
		if strings.TrimSpace(n.Attrs[k]) != "" {
			return true
		}
	}
	tag := strings.ToLower(n.Tag)
	if tag == "input" {
		switch strings.ToLower(strings.TrimSpace(n.Attrs["type"])) {
		case "hidden":
			return true
		case "submit", "button", "reset":
			return strings.TrimSpace(n.Attrs["value"]) != ""
		}
	}
	return false
}

func nameOf(n core.VisualNode) string {
	if t := strings.TrimSpace(n.Text); t != "" {
		return truncate(t, 30)
	}
	if l := strings.TrimSpace(n.Attrs["aria-label"]); l != "" {
		return truncate(l, 30)
	}
	return strings.ToLower(n.Tag)
}

// screenshot resolves a screenshot reference and proves it is a non-empty
// image file inside the project. Absolute paths outside the project are
// refused: evidence must belong to the project it describes.
func screenshot(dir, ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.TrimSpace(dir) == "" {
		return "", false
	}
	base, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	abs := filepath.FromSlash(ref)
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(base, abs)
	}
	abs, err = filepath.Abs(abs)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(base, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	switch strings.ToLower(filepath.Ext(abs)) {
	case ".png", ".jpg", ".jpeg", ".webp":
	default:
		return "", false
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() || info.Size() == 0 {
		return "", false
	}
	if !imageFile(abs) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// imageFile checks the file signature, so a text file renamed .png is not
// accepted as screenshot evidence.
func imageFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 12)
	n, _ := f.Read(head)
	head = head[:n]
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return len(head) >= 8 && string(head[:8]) == "\x89PNG\r\n\x1a\n"
	case ".jpg", ".jpeg":
		return len(head) >= 3 && head[0] == 0xff && head[1] == 0xd8 && head[2] == 0xff
	case ".webp":
		return len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP"
	}
	return false
}

func snapshotLabel(s core.VisualSnapshot) string {
	name := strings.TrimSpace(s.Viewport.Name)
	if name == "" {
		name = fmt.Sprintf("%.0fx%.0f", s.Viewport.Width, s.Viewport.Height)
	}
	if s.State != "" {
		return name + "/" + s.State
	}
	return name
}

func dedupe(in []core.VisualFinding) []core.VisualFinding {
	seen := map[string]bool{}
	out := in[:0]
	for _, f := range in {
		k := f.Page + "\x00" + f.Criterion + "\x00" + f.Evidence + "\x00" + f.Message
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, f)
	}
	if out == nil {
		return []core.VisualFinding{}
	}
	return out
}

func sortFindings(in []core.VisualFinding) {
	rank := map[string]int{core.SeverityCritical: 0, core.SeverityHigh: 1, core.SeverityMedium: 2, core.SeverityLow: 3}
	sort.SliceStable(in, func(i, j int) bool {
		if rank[in[i].Severity] != rank[in[j].Severity] {
			return rank[in[i].Severity] < rank[in[j].Severity]
		}
		if in[i].Criterion != in[j].Criterion {
			return in[i].Criterion < in[j].Criterion
		}
		return in[i].Message < in[j].Message
	})
}

func resolved(prev, now []core.VisualFinding) []string {
	if len(prev) == 0 {
		return nil
	}
	current := map[string]bool{}
	for _, f := range now {
		current[f.Criterion+"\x00"+f.Message] = true
	}
	var out []string
	for _, f := range prev {
		if !current[f.Criterion+"\x00"+f.Message] {
			out = append(out, f.Message)
		}
	}
	return out
}

func appendUnique(xs []string, v string) []string {
	if !slices.Contains(xs, v) {
		xs = append(xs, v)
	}
	return xs
}

func unique(xs []string) []string {
	var out []string
	for _, x := range xs {
		if !slices.Contains(out, x) {
			out = append(out, x)
		}
	}
	return out
}

func minMax(m map[float64]bool) (min, max float64) {
	first := true
	for v := range m {
		if first || v < min {
			min = v
		}
		if first || v > max {
			max = v
		}
		first = false
	}
	return min, max
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func strconvQuote(s string) string { return `"` + s + `"` }
