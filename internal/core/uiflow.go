package core

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// The screens follow the use cases: a flow passes through its pages in
// order, each step a user takes happens on one of them (a section, field,
// link or action names it), and consecutive pages are linked. A system step
// (an automated check, a device or provider message) is feedback on the
// page the user is on, not a screen of its own.

// FlowStep is one step of a use case placed on a page.
type FlowStep struct {
	N      int    `json:"n"`
	Text   string `json:"text"`
	System bool   `json:"system,omitempty"`
	Page   string `json:"page,omitempty"`
	// Placed is false for a user step no page of the flow names: the step
	// has no visible place in the screens yet.
	Placed bool `json:"placed"`
	// Decision marks an approval or another decision point; Risky an action
	// that cannot be undone (delete, cancel, post) and needs confirmation.
	Decision bool `json:"decision,omitempty"`
	Risky    bool `json:"risky,omitempty"`
	// Group is the parallel group (1-based) the step runs in, 0 when it
	// follows the step before it; Status the document status changes it
	// makes ("PurchaseOrder: Draft → Diajukan").
	Group  int      `json:"group,omitempty"`
	Status []string `json:"status,omitempty"`
}

// FlowBranch is an alternative or failure scenario of a use case: the step
// it leaves the main flow at, where it goes next, and the page that shows
// it.
type FlowBranch struct {
	Kind string `json:"kind"` // alternative | failure
	Text string `json:"text"`
	// From is the step it branches from (0 when the text does not say);
	// Next is "step:N", "end" or "cancel" when the text says where the flow
	// continues; Flows the use cases it hands over to.
	From  int      `json:"from,omitempty"`
	Next  string   `json:"next,omitempty"`
	Flows []string `json:"flows,omitempty"`
	Page  string   `json:"page,omitempty"`
	// System is set when the designer marks it as handled without a screen.
	System bool `json:"system,omitempty"`
	Placed bool `json:"placed"`
}

// FlowWalk is a use case as the user walks it through the screens.
type FlowWalk struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Role    string   `json:"role,omitempty"`
	Goal    string   `json:"goal,omitempty"`
	Trigger string   `json:"trigger,omitempty"`
	Pre     []string `json:"preconditions,omitempty"`
	Post    []string `json:"postconditions,omitempty"`
	Data    []string `json:"data_effects,omitempty"`
	Pages   []string `json:"pages"`
	// Inferred is set when the design names no pages for the flow and
	// they are guessed from the steps; Declared when the designer placed
	// every step on its page (step_pages) instead of the words matching.
	Inferred bool `json:"inferred,omitempty"`
	Declared bool `json:"declared,omitempty"`
	// BranchesDeclared: the designer placed every alternative and failure
	// (branch_pages) instead of the words matching.
	BranchesDeclared bool         `json:"branches_declared,omitempty"`
	Steps            []FlowStep   `json:"steps"`
	Branches         []FlowBranch `json:"branches,omitempty"`
	Alternatives     []string     `json:"alternatives,omitempty"`
	Failures         []string     `json:"failures,omitempty"`
	// Parallel are the step groups done at the same time.
	Parallel []ParallelSteps `json:"parallel,omitempty"`
}

// FlowWalks lists the use cases that have screens, each step placed on a
// page of the layout.
func FlowWalks(p *Project, l *LayoutSpec) []FlowWalk {
	out := []FlowWalk{}
	if l == nil {
		return out
	}
	for _, fl := range p.Flows {
		if !p.ScreenFlow(fl) {
			continue
		}
		w := FlowWalk{ID: fl.ID, Name: fl.Name, Role: fl.Role, Goal: fl.Goal, Trigger: fl.Trigger, Pre: fl.Pre, Post: fl.Post,
			Data: fl.Data, Alternatives: fl.Alternatives, Failures: fl.Failures, Declared: StepsDeclared(fl),
			BranchesDeclared: branchesDeclared(fl)}
		if len(flowPages(l, fl)) == 0 {
			fl.PageIDs, w.Inferred = InferFlowPages(l, fl), true
		}
		w.Pages, w.Steps = flowPages(l, fl), p.FlowSteps(l, fl)
		w.Branches = MapFlowBranches(l, fl, w.Steps)
		w.Parallel = fl.Parallel
		out = append(out, w)
	}
	return out
}

// FlowSteps places the steps of a use case on its pages (MapFlowSteps)
// with what the use case declares about them (markSteps).
func (p *Project) FlowSteps(l *LayoutSpec, fl Flow) []FlowStep {
	steps := MapFlowSteps(l, fl)
	p.markSteps(fl, steps)
	return steps
}

// markSteps adds what the use case declares to its placed steps: the
// parallel group of each step, and the document status changes from the
// lifecycles. A flow whose transitions name their steps decides where the
// words would only guess: a step is a decision when it approves or rejects
// a document.
func (p *Project) markSteps(fl Flow, steps []FlowStep) {
	trans := p.StepTransitions(fl.ID)
	for i := range steps {
		st := &steps[i]
		st.Group = ParallelGroup(fl, st.N)
		if len(trans) == 0 {
			continue
		}
		st.Decision = false
		for _, t := range trans[st.N] {
			st.Status = append(st.Status, p.LifecycleOf(t.Entity).TransitionText(t.T))
			if !st.System && (t.T.Kind == TransApprove || t.T.Kind == TransReject) {
				st.Decision = true
			}
		}
	}
}

// ParallelHop reports whether moving from one page of a flow to the next
// goes between two steps done at the same time: no one walks from one to
// the other, so the pages need no link.
func ParallelHop(steps []FlowStep, from, to string) bool {
	last := -1
	for j, st := range steps {
		switch {
		case st.Page == from:
			last = j
		case st.Page == to && last >= 0:
			g := steps[last].Group
			return g != 0 && g == st.Group
		}
	}
	return false
}

// StepsDeclared reports whether the designer placed every step of the flow
// on a page (one entry per step).
func StepsDeclared(fl Flow) bool {
	return len(fl.Steps) > 0 && len(fl.StepPages) == len(fl.Steps)
}

// branchesDeclared reports whether the designer placed every alternative
// and failure (one entry each, alternatives first).
func branchesDeclared(fl Flow) bool {
	n := len(fl.Alternatives) + len(fl.Failures)
	return n > 0 && len(fl.BranchPages) == n
}

// ScreenFlow reports whether a use case is done on screens: the design
// maps it to pages, or its feature runs on a platform with screens (any
// flow, when the feature is unknown and the project has screens).
func (p *Project) ScreenFlow(fl Flow) bool {
	if len(fl.PageIDs) > 0 {
		return true
	}
	for _, f := range p.Features {
		if f.ID == fl.FeatureID {
			return slices.ContainsFunc(f.Platforms, func(pl string) bool { return slices.Contains(UIPlatform, pl) })
		}
	}
	return len(p.UIPlatforms()) > 0
}

// flowPages are the flow's pages that exist in the layout, in order: the
// pages its steps are placed on when the designer placed them, else its
// page list.
func flowPages(l *LayoutSpec, fl Flow) []string {
	var out []string
	if StepsDeclared(fl) {
		if out = RouteOf(fl.StepPages); len(out) > 0 {
			out = slices.DeleteFunc(out, func(id string) bool { return layoutPage(l, id) == nil })
			return slices.Compact(out)
		}
	}
	for _, id := range fl.PageIDs {
		if layoutPage(l, id) != nil {
			out = append(out, id)
		}
	}
	return out
}

// RouteOf is the page sequence of per-step pages: system steps (empty)
// left out and a page repeated by consecutive steps listed once.
func RouteOf(stepPages []string) []string {
	var out []string
	for _, id := range stepPages {
		if id = strings.TrimSpace(id); id != "" && (len(out) == 0 || out[len(out)-1] != id) {
			out = append(out, id)
		}
	}
	return out
}

func layoutPage(l *LayoutSpec, id string) *LayoutPage {
	for i := range l.Pages {
		if l.Pages[i].ID == id {
			return &l.Pages[i]
		}
	}
	return nil
}

// MapFlowSteps places each step of a flow on the flow page whose own text
// shares the most words with it, preferring the page of the previous step.
func MapFlowSteps(l *LayoutSpec, fl Flow) []FlowStep {
	pages := flowPages(l, fl)
	words := make([][]string, len(pages))
	for i, id := range pages {
		words[i] = stems(pageText(layoutPage(l, id)))
	}
	out := make([]FlowStep, 0, len(fl.Steps))
	if StepsDeclared(fl) {
		cur := ""
		if len(pages) > 0 {
			cur = pages[0]
		}
		for i, text := range fl.Steps {
			st := FlowStep{N: i + 1, Text: strings.TrimSpace(text)}
			switch id := strings.TrimSpace(fl.StepPages[i]); {
			case id == "":
				st.System, st.Page, st.Placed = true, cur, cur != ""
			case layoutPage(l, id) != nil:
				// A system activity the designer shows on a page (its
				// feedback) stays a system step: it is not a task of the
				// user to place.
				cur = id
				st.Page, st.Placed, st.System = id, true, SystemStep(st.Text)
			default:
				st.Page = cur // a page that does not exist: reported by flowIssues
			}
			if !st.System {
				st.Decision, st.Risky = DecisionStep(st.Text) || ApprovalRequest(st.Text), RiskyStep(st.Text)
			}
			out = append(out, st)
		}
		return out
	}
	cur := 0
	for i, text := range fl.Steps {
		st := FlowStep{N: i + 1, Text: strings.TrimSpace(text), System: SystemStep(text)}
		if !st.System {
			st.Decision, st.Risky = DecisionStep(st.Text) || ApprovalRequest(st.Text), RiskyStep(st.Text)
		}
		if len(pages) == 0 {
			out = append(out, st)
			continue
		}
		if st.System {
			st.Page, st.Placed = pages[cur], true
			out = append(out, st)
			continue
		}
		step := stems(text)
		best, score := cur, overlap(step, words[cur])
		for j := range pages {
			// A later page wins a tie over an earlier one: flows move forward.
			if s := overlap(step, words[j]); s > score || s == score && s > 0 && j > cur && best < cur {
				best, score = j, s
			}
		}
		if score == 0 && st.Decision {
			// A decision is taken where the page offers both outcomes,
			// whatever words the step uses ("Approver memutuskan").
			for j := range pages {
				if decides(words[j]) {
					best, score = j, 1
					break
				}
			}
		}
		if score > 0 {
			cur = best
		}
		st.Page, st.Placed = pages[cur], score > 0
		out = append(out, st)
	}
	return out
}

// MaxInferredPages caps the pages a flow is guessed to pass through.
const MaxInferredPages = 4

// InferFlowPages guesses the pages a flow passes through when the design
// names none. It starts on the flow's feature page; a user step moves to
// another page only when that page shares a distinctive word with it (one
// few pages use, counted double when the current page links there) and
// more than the current page does. Sign-in pages are
// left out unless they belong to the flow's feature.
func InferFlowPages(l *LayoutSpec, fl Flow) []string {
	if l == nil || len(l.Pages) == 0 {
		return nil
	}
	words := make([][]string, len(l.Pages))
	df := map[string]int{}
	for i := range l.Pages {
		words[i] = stems(pageText(&l.Pages[i]))
		for _, w := range words[i] {
			df[w]++
		}
	}
	cur := slices.IndexFunc(l.Pages, func(pg LayoutPage) bool { return fl.FeatureID != "" && pg.FeatureID == fl.FeatureID })
	var out []string
	visit := func(i int) {
		if id := l.Pages[i].ID; !slices.Contains(out, id) && len(out) < MaxInferredPages {
			out = append(out, id)
		}
	}
	for _, text := range fl.Steps {
		if SystemStep(text) {
			continue
		}
		step := stems(text)
		best, score := cur, 0.0
		if cur >= 0 {
			score = weighted(step, words[cur], df)
		}
		for i, pg := range l.Pages {
			if i == cur || pg.Pattern == "auth" && pg.FeatureID != fl.FeatureID {
				continue
			}
			s := weighted(step, words[i], df)
			if cur >= 0 && slices.ContainsFunc(l.Pages[cur].Links, func(ln PageLink) bool { return ln.To == pg.ID }) {
				s *= 2 // the designer already leads there
			}
			if s >= minInferScore && s > score+1e-9 {
				best, score = i, s
			}
		}
		if best >= 0 && (score > 0 || best == cur) {
			cur = best
			visit(cur)
		}
	}
	return out
}

// minInferScore is the weight a step must share with a page to move there:
// a word only that page uses, or one of two pages the current page links to.
const minInferScore = 1.0

// weighted is the overlap where each shared word counts 1/(pages using it).
func weighted(step, page []string, df map[string]int) float64 {
	n := 0.0
	for _, a := range step {
		for _, b := range page {
			if stemMatch(a, b) {
				n += 1 / float64(max(df[b], 1))
				break
			}
		}
	}
	return n
}

// pageText is everything a page shows or names.
func pageText(pg *LayoutPage) string { return strings.Join(pageParts(pg), " ") }

// pageParts are the separate texts of a page (name, purpose, each content
// item, field label, link and state), so a pattern never runs from one
// into the next.
func pageParts(pg *LayoutPage) []string {
	parts := []string{pg.Name, pg.Purpose, pg.PrimaryAction}
	for _, s := range pg.Sections {
		parts = append(parts, s.Name)
		parts = append(parts, s.Content...)
	}
	for _, f := range pg.Fields {
		parts = append(parts, f.Label, f.Note)
	}
	for _, ln := range pg.Links {
		parts = append(parts, ln.Via)
	}
	for _, v := range pg.States {
		parts = append(parts, v)
	}
	for _, s := range pg.Sections {
		for _, v := range s.States {
			parts = append(parts, v)
		}
	}
	return parts
}

var systemWords = map[string]bool{
	"sistem": true, "system": true, "worker": true, "adapter": true, "penyedia": true, "perangkat": true,
	"server": true, "job": true, "scheduler": true, "webhook": true, "emit": true, "otomatis": true,
	"cron": true, "api": true, "gateway": true, "integrasi": true, "hitung": true, "tunggu": true, "cek": true,
	"edge": true, "cloud": true, "saat": true, "retry": true,
}

// SystemStep reports whether a step is done by the system, a device or a
// provider rather than by the user: it starts with such an actor or an
// automated check, is passive ("Sesi dibuat"), or names a technical
// identifier (ParkingSession, idempotency_key, payment.completed).
func SystemStep(text string) bool {
	f := strings.Fields(strings.TrimSpace(text))
	if len(f) == 0 {
		return false
	}
	if systemWords[strings.ToLower(strings.Trim(f[0], ",.:;"))] {
		return true
	}
	first := strings.ToLower(f[0])
	command := strings.HasSuffix(first, "kan") || imperatives[first]
	for i, w := range f {
		w = strings.Trim(w, ",.:;()")
		if strings.Contains(w, "_") || technicalName(w) || i > 0 && !command && passive(w) {
			return true
		}
	}
	return false
}

// imperatives are the base verbs a user step starts with.
var imperatives = map[string]bool{
	"buat": true, "isi": true, "pilih": true, "kirim": true, "cari": true, "catat": true, "tinjau": true,
	"periksa": true, "terima": true, "tutup": true, "ambil": true, "unggah": true, "unduh": true, "buka": true,
	"ubah": true, "setujui": true, "tolak": true, "kemas": true, "posting": true, "undang": true, "bayar": true,
}

// passive is an Indonesian passive verb (dibuat, diproses, disimpan).
func passive(w string) bool {
	w = strings.ToLower(w)
	return len(w) >= 6 && strings.HasPrefix(w, "di") && !strings.ContainsRune("aiueo", rune(w[2]))
}

// technicalName is a CamelCase identifier (VehicleEvent) or a dotted event
// name (payment.completed).
func technicalName(w string) bool {
	rs := []rune(w)
	if len(rs) < 4 {
		return false
	}
	if i := strings.IndexByte(w, '.'); i > 0 && i < len(w)-1 && strings.ToLower(w) == w {
		return true
	}
	if !unicode.IsUpper(rs[0]) {
		return false
	}
	lower := false
	for _, r := range rs[1:] {
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r) && lower:
			return true
		}
	}
	return false
}

var stopWords = map[string]bool{
	"dan": true, "atau": true, "dengan": true, "untuk": true, "dari": true, "yang": true, "pada": true,
	"per": true, "setiap": true, "bila": true, "jika": true, "lalu": true, "sesuai": true, "oleh": true,
	"dalam": true, "akan": true, "sudah": true, "belum": true, "tidak": true, "bisa": true, "dapat": true,
	"ada": true, "baru": true, "semua": true, "lain": true, "the": true, "and": true, "with": true, "for": true,
}

// stems are the content words of a text, reduced to their roots so
// "pembayaran" meets "bayar". Short all-caps words (PO, RFQ) stay whole.
func stems(text string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if n := len(w); n >= 2 && n <= 5 && strings.ToUpper(w) == w && unicode.IsLetter([]rune(w)[0]) {
			out = append(out, strings.ToLower(w))
			continue
		}
		w = strings.ToLower(w)
		if stopWords[w] {
			continue
		}
		if r := stem(w); r != "" && !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	return out
}

// stem strips the common Indonesian affixes (and an English plural). A
// prefix that swallows the first letter of the root (menyetujui ← setuju,
// memutuskan ← putus, menolak ← tolak, mengirim ← kirim) and one that may
// belong to the root (direktur) give several candidate roots, joined by
// "|"; stemMatch tries each.
func stem(w string) string {
	roots := []string{w}
	for _, p := range []string{"meng", "meny", "mem", "men", "me", "peng", "peny", "per", "pem", "pen", "pe", "di", "ber", "ter"} {
		if !strings.HasPrefix(w, p) || len(w)-len(p) < 4 {
			continue
		}
		r := w[len(p):]
		roots = []string{r}
		if strings.ContainsRune("aiueo", rune(r[0])) {
			for _, c := range map[string]string{"meny": "s", "peny": "s", "mem": "pm", "pem": "pm", "men": "tn", "pen": "tn", "meng": "k", "peng": "k"}[p] {
				roots = append(roots, string(c)+r)
			}
		}
		if p == "di" || p == "ter" || p == "ber" || p == "per" {
			roots = append(roots, w)
		}
		break
	}
	for i, r := range roots {
		for _, s := range []string{"kan", "nya", "an", "i", "s"} {
			if strings.HasSuffix(r, s) && len(r)-len(s) >= 4 {
				roots[i] = r[:len(r)-len(s)]
				break
			}
		}
	}
	roots = slices.DeleteFunc(roots, func(r string) bool { return len(r) < 4 })
	return strings.Join(slices.Compact(roots), "|")
}

// overlap counts the step's words the page names.
func overlap(step, page []string) int {
	n := 0
	for _, a := range step {
		if slices.ContainsFunc(page, func(b string) bool { return stemMatch(a, b) }) {
			n++
		}
	}
	return n
}

func stemMatch(a, b string) bool {
	for _, x := range strings.Split(a, "|") {
		for _, y := range strings.Split(b, "|") {
			if rootMatch(x, y) {
				return true
			}
		}
	}
	return false
}

func rootMatch(a, b string) bool {
	if a == b {
		return true
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	return len(a) >= 4 && strings.HasPrefix(b, a)
}

// CrowdedFlowSteps is how many user steps of one flow on one page call for
// splitting the task (a form or detail page, or a wizard).
const CrowdedFlowSteps = 5

// flowIssues checks that the screens follow the use cases: each flow names
// its pages, consecutive pages are linked, each user step has a place, a
// decision has a screen to decide on, a risky step is confirmed, and the
// alternative and failure scenarios can be followed. Word matching is only
// a guess, so a step or scenario it cannot place is a warning or a note,
// never an error; what the designer declared is checked strictly.
func flowIssues(p *Project, l *LayoutSpec, rules *DesignRules, format int) []Issue {
	level := IssueWarning
	if format >= UIDesignFormat {
		level = IssueError
	}
	var out []Issue
	add := func(level, where, msg string, a ...any) {
		out = append(out, Issue{Level: level, Where: where, Message: fmt.Sprintf(msg, a...)})
	}
	confirmAll := rules != nil && mentionsConfirm(strings.Join(append(slices.Clone(rules.UX), rules.Interaction...), " "))
	for _, fl := range p.Flows {
		if !p.ScreenFlow(fl) {
			continue
		}
		where := "flow:" + fl.ID
		for i, id := range fl.StepPages {
			if id = strings.TrimSpace(id); id != "" && layoutPage(l, id) == nil {
				add(IssueError, where, "langkah %d ditempatkan di halaman %q yang tidak ada", i+1, id)
			}
		}
		for i, id := range fl.BranchPages {
			if id = strings.TrimSpace(id); id != "" && layoutPage(l, id) == nil {
				add(IssueError, where, "skenario ke-%d ditempatkan di halaman %q yang tidak ada", i+1, id)
			}
		}
		if len(fl.StepPages) > 0 && !StepsDeclared(fl) {
			add(IssueWarning, where, "step_pages memuat %d halaman untuk %d langkah: penempatan langkah ditebak dari kata", len(fl.StepPages), len(fl.Steps))
		}
		pages := flowPages(l, fl)
		if len(pages) == 0 {
			add(IssueWarning, where, "use case %s %s belum dipetakan ke halaman (flows): layar tidak bisa ditelusuri per langkah", fl.ID, fl.Name)
			continue
		}
		steps := p.FlowSteps(l, fl)
		for i := 1; i < len(pages); i++ {
			from, to := pages[i-1], pages[i]
			if from == to || ParallelHop(steps, from, to) {
				continue
			}
			if pg := layoutPage(l, from); !slices.ContainsFunc(pg.Links, func(ln PageLink) bool { return ln.To == to }) {
				add(level, "page:"+from, "use case %s berlanjut ke %q tetapi halaman ini belum punya link ke sana", fl.ID, to)
			}
		}
		perPage := map[string]int{}
		decision := 0
		for _, st := range steps {
			if st.System {
				continue
			}
			perPage[st.Page]++
			if !st.Placed {
				add(IssueWarning, where, "langkah %d %q belum tergambar di halaman alurnya (%s): tambahkan section, field atau aksi untuk langkah ini",
					st.N, st.Text, strings.Join(pages, ", "))
			}
			if st.Decision && decision == 0 {
				decision = st.N
			}
			if st.Risky && st.Placed && !confirmAll && !mentionsConfirm(pageText(layoutPage(l, st.Page))) {
				add(IssueWarning, "page:"+st.Page, "langkah %d %q tidak dapat dibatalkan tetapi halaman ini belum menyebut konfirmasinya (dialog konfirmasi atau ringkasan sebelum %s)",
					st.N, st.Text, strings.ToLower(StepAction(st.Text)))
			}
		}
		if decision > 0 && !decisionScreen(l, pages) {
			add(IssueWarning, where, "langkah %d butuh keputusan (setujui/tolak) tetapi belum ada layar tempat pemberi keputusan menyetujui dan menolak atau mengembalikan", decision)
		}
		for _, id := range pages {
			if n := perPage[id]; n >= CrowdedFlowSteps && layoutPage(l, id).Pattern != "wizard" {
				add(IssueWarning, "page:"+id, "use case %s menjalankan %d langkah di halaman ini: pecah ke halaman form/detail atau pakai wizard/stepper", fl.ID, n)
				perPage[id] = 0
			}
		}
		for _, br := range MapFlowBranches(l, fl, steps) {
			label := "alternatif"
			if br.Kind == BranchFailure {
				label = "kegagalan"
			}
			if br.From > len(fl.Steps) {
				add(IssueWarning, where, "%s %q merujuk langkah %d yang tidak ada", label, br.Text, br.From)
			}
			if n, ok := strings.CutPrefix(br.Next, "step:"); ok {
				if k, _ := strconv.Atoi(n); k < 1 || k > len(fl.Steps) {
					add(IssueWarning, where, "%s %q kembali ke langkah %s yang tidak ada", label, br.Text, n)
				}
			}
			for _, id := range br.Flows {
				other := p.flow(id)
				switch {
				case other == nil:
					add(IssueWarning, where, "%s %q menyerahkan ke use case %s yang tidak ada", label, br.Text, id)
				case br.Page != "":
					if to := flowPages(l, *other); len(to) > 0 && to[0] != br.Page &&
						!slices.ContainsFunc(layoutPage(l, br.Page).Links, func(ln PageLink) bool { return ln.To == to[0] }) {
						add(IssueWarning, "page:"+br.Page, "%s %q berlanjut ke use case %s (%s) tetapi halaman ini belum punya link ke sana", label, br.Text, id, to[0])
					}
				}
			}
			if !br.Placed && !br.System {
				add(IssueInfo, where, "%s %q belum terlihat di layar alurnya: tampilkan sebagai pesan, state atau aksi di %s", label, br.Text, firstNonBlank(br.Page, pages[0]))
			}
		}
	}
	return out
}

// FlowByID is the use case with the id (any case), or nil.
func (p *Project) FlowByID(id string) *Flow { return p.flow(id) }

func (p *Project) flow(id string) *Flow {
	for i := range p.Flows {
		if strings.EqualFold(p.Flows[i].ID, id) {
			return &p.Flows[i]
		}
	}
	return nil
}

func firstNonBlank(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// Branch kinds.
const (
	BranchAlternative = "alternative"
	BranchFailure     = "failure"
)

var (
	branchFrom  = regexp.MustCompile(`(?i)^\s*(?:langkah|step)\s*(\d+)[a-z]?\b|^\s*(\d+)[a-z]?[.):]\s`)
	branchStep  = regexp.MustCompile(`(?i)\b(?:kembali|ulangi|lanjut(?:kan)?|ke)\s+(?:ke\s+)?(?:langkah|step)\s*(\d+)`)
	branchEnd   = regexp.MustCompile(`(?i)\b(?:selesai|berakhir|end)\b`)
	branchStop  = regexp.MustCompile(`(?i)\b(?:batal|dibatalkan|cancel(?:led)?)\b`)
	branchFlows = regexp.MustCompile(`\bFLOW-\d+\b`)
)

// MapFlowBranches reads where each alternative and failure leaves the main
// flow and where it goes ("Langkah 3: ditolak -> kembali ke langkah 2",
// "-> FLOW-004"), and places it on the page the designer named, else on
// the flow page whose text (including its states) shows it, else on the
// page of the step it branches from.
func MapFlowBranches(l *LayoutSpec, fl Flow, steps []FlowStep) []FlowBranch {
	var out []FlowBranch
	pages := flowPages(l, fl)
	declared := branchesDeclared(fl)
	pageWords := make([][]string, len(pages))
	for i, id := range pages {
		pageWords[i] = stems(pageText(layoutPage(l, id)))
	}
	texts := append(slices.Clone(fl.Alternatives), fl.Failures...)
	for i, text := range texts {
		br := FlowBranch{Kind: BranchAlternative, Text: strings.TrimSpace(text)}
		if i >= len(fl.Alternatives) {
			br.Kind = BranchFailure
		}
		if m := branchFrom.FindStringSubmatch(text); m != nil {
			br.From, _ = strconv.Atoi(m[1] + m[2])
		}
		if br.From == 0 {
			br.From = branchStepOf(br.Text, steps)
		}
		switch {
		case branchStep.MatchString(text):
			br.Next = "step:" + branchStep.FindStringSubmatch(text)[1]
		case branchStop.MatchString(text):
			br.Next = "cancel"
		case branchEnd.MatchString(text):
			br.Next = "end"
		}
		for _, id := range branchFlows.FindAllString(text, -1) {
			if !strings.EqualFold(id, fl.ID) && !slices.Contains(br.Flows, id) {
				br.Flows = append(br.Flows, id)
			}
		}
		if declared {
			switch id := strings.TrimSpace(fl.BranchPages[i]); {
			case id == "":
				br.System = true
			case layoutPage(l, id) != nil:
				br.Page, br.Placed = id, true
			}
			out = append(out, br)
			continue
		}
		words := stems(br.Text)
		best, score := -1, 0
		for j := range pages {
			if s := overlap(words, pageWords[j]); s > score {
				best, score = j, s
			}
		}
		switch {
		case best >= 0 && score*2 >= len(words):
			// at least half of the scenario's words are on the page
			br.Page, br.Placed = pages[best], true
		case br.From > 0 && br.From <= len(steps):
			br.Page = steps[br.From-1].Page
		case len(pages) > 0:
			br.Page = pages[0]
		}
		out = append(out, br)
	}
	return out
}

// branchStepOf is the step a scenario most likely leaves the main flow at:
// a rejection or approval leaves at the decision step, otherwise the step
// that shares the most words with it (0 when none does).
func branchStepOf(text string, steps []FlowStep) int {
	words := stems(text)
	if hasStem(words, approveStems...) || hasStem(words, rejectStems...) {
		for i := len(steps) - 1; i >= 0; i-- {
			if steps[i].Decision {
				return steps[i].N
			}
		}
	}
	best, score := 0, 0
	for _, st := range steps {
		if s := overlap(words, stems(st.Text)); s > score {
			best, score = st.N, s
		}
	}
	return best
}

var (
	approveStems = []string{"setuju", "approv"}
	rejectStems  = []string{"tolak", "reject", "revisi", "decline"}
	riskyStems   = []string{"hapus", "batal", "nonaktif", "void", "delete", "cancel", "posting", "reset", "musnah"}
	confirmStems = []string{"konfirmas", "confirm", "dialog", "yakin", "ringkas"}
)

func hasStem(words []string, want ...string) bool {
	return slices.ContainsFunc(words, func(w string) bool {
		return slices.ContainsFunc(strings.Split(w, "|"), func(r string) bool {
			return slices.ContainsFunc(want, func(x string) bool { return rootMatch(r, x) || strings.HasPrefix(r, x) })
		})
	})
}

// DecisionStep reports whether a user step decides: approves, rejects or
// returns a request.
func DecisionStep(text string) bool {
	if ApprovalRequest(text) {
		return false
	}
	// "Approver" names who decides, not the decision.
	w := slices.DeleteFunc(stems(text), func(x string) bool { return strings.HasPrefix(x, "approver") })
	return hasStem(w, "setuju", "approv", "tolak", "reject") || hasStem(w, "putus")
}

// ApprovalRequest reports whether a step submits something for approval
// ("Ajukan PO untuk persetujuan"): another role must then decide.
func ApprovalRequest(text string) bool {
	f := strings.Fields(strings.ToLower(text))
	return len(f) > 0 && slices.Contains([]string{"ajukan", "kirim", "minta", "mintakan", "submit", "teruskan"}, f[0]) &&
		hasStem(stems(text), "setuju", "approv")
}

// RiskyStep reports whether a user step cannot be undone (delete, cancel,
// deactivate, post) and so needs a confirmation.
func RiskyStep(text string) bool {
	f := strings.Fields(strings.ToLower(text))
	return len(f) > 0 && hasStem(stems(f[0]), riskyStems...)
}

// DangerAction reports whether an action label rejects or cannot be
// undone: it is shown as a destructive button with a confirmation.
func DangerAction(label string) bool {
	f := strings.Fields(strings.ToLower(label))
	return len(f) > 0 && (RiskyStep(label) || hasStem(stems(f[0]), "tolak", "reject", "decline"))
}

func mentionsConfirm(text string) bool { return hasStem(stems(text), confirmStems...) }

// decides reports whether page words offer both outcomes of a decision.
func decides(words []string) bool {
	return hasStem(words, approveStems...) && hasStem(words, rejectStems...)
}

// decisionScreen reports whether a page offers both outcomes of a decision:
// approve, and reject or return. The flow's own pages are looked at first;
// the decider may work on another page of the layout.
func decisionScreen(l *LayoutSpec, pages []string) bool {
	both := func(pg *LayoutPage) bool { return decides(stems(pageText(pg))) }
	for _, id := range pages {
		if both(layoutPage(l, id)) {
			return true
		}
	}
	for i := range l.Pages {
		if both(&l.Pages[i]) {
			return true
		}
	}
	return false
}

// CreateStep reports whether a step starts a record or a request.
func CreateStep(text string) bool {
	f := strings.Fields(strings.ToLower(text))
	return len(f) > 0 && slices.Contains([]string{"buat", "tambah", "tambahkan", "ajukan", "daftarkan", "catat",
		"input", "undang", "unggah", "impor", "rekam", "jadwalkan"}, f[0])
}

// StepAction turns a step into a short button label: its first clause, at
// most 40 characters, capitalized.
func StepAction(text string) string {
	t := strings.TrimSpace(text)
	for _, sep := range []string{", ", "; ", " lalu ", " sampai ", " atau ", " bila ", " jika "} {
		if i := strings.Index(t, sep); i > 0 {
			t = t[:i]
		}
	}
	if r := []rune(t); len(r) > 40 {
		t = string(r[:40])
		if i := strings.LastIndexByte(t, ' '); i > 0 {
			t = t[:i]
		}
	}
	// A label cut in the middle does not end on a joining word ("Buat
	// DeviceCommand dengan").
	for f := strings.Fields(t); len(f) > 1 && joinWords[strings.ToLower(f[len(f)-1])]; f = f[:len(f)-1] {
		t = strings.Join(f[:len(f)-1], " ")
	}
	if t == "" {
		return t
	}
	r := []rune(t)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// joinWords end a phrase that was cut short.
var joinWords = map[string]bool{"dan": true, "atau": true, "dengan": true, "untuk": true, "ke": true, "di": true, "dari": true,
	"yang": true, "pada": true, "and": true, "or": true, "with": true, "for": true, "to": true, "of": true, "the": true}

// ActionPatterns are the page patterns whose header carries the action
// that starts a task.
var ActionPatterns = []string{"table", "list", "master-detail", "cards", "detail", "kanban", "calendar"}

// FlowPrimaryAction is the action that starts a task on a page: the first
// user step of a flow that begins there, when it creates something.
func FlowPrimaryAction(walks []FlowWalk, pageID string) string {
	for _, w := range walks {
		if len(w.Pages) == 0 || w.Pages[0] != pageID {
			continue
		}
		for _, st := range w.Steps {
			if st.Page == pageID && !st.System && st.Placed {
				if CreateStep(st.Text) {
					return StepAction(st.Text)
				}
				break
			}
		}
	}
	return ""
}
