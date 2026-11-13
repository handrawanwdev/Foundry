package techplan

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/repoplan"
	"github.com/handrawanwdev/foundry/internal/modules/stack"
)

// finding is one problem of the plan. Refuse marks a defect of the plan
// itself (incomplete, malformed, breaking a binding choice): an answer that
// has one is sent back to the AI. The others are conflicts between
// decisions or with other artifacts: they land with the plan, are shown, and
// block approval until resolved.
type finding struct {
	core.TechIssue
	Refuse bool
	Check  string // the consistency area the finding belongs to
	Compat string // set when the finding is also an incompatibility, naming its compatibility area
}

type collector struct{ out []finding }

func (c *collector) add(level, area, check string, refuse bool, f string, a ...any) {
	code := CodeConflict
	if level != "error" {
		code = CodeWarning
	}
	c.issue(level, area, check, refuse, code, "", "", f, a...)
}

// issue is add with the code, the path in the plan and the fix hint that a
// targeted repair and the diagnosis need.
func (c *collector) issue(level, area, check string, refuse bool, code, path, hint string, f string, a ...any) {
	c.out = append(c.out, finding{TechIssue: core.TechIssue{Level: level, Area: area, Message: fmt.Sprintf(f, a...), Code: code, Path: path, Hint: hint},
		Refuse: refuse, Check: check})
}

func itemPath(area string, rest ...string) string {
	return strings.Join(append([]string{"items." + area}, rest...), ".")
}

// Refresh recomputes everything Foundry derives from the plan and the
// project: issues, checks, coverage, counts, staleness. It is called on
// every read and after every change; nothing in the analysis is trusted from
// the AI.
func Refresh(p *core.Project, plan *core.TechPlan) {
	if plan == nil {
		return
	}
	refresh(p, plan, p.Stack)
}

func refresh(p *core.Project, plan *core.TechPlan, st *core.TechStack) {
	a := &plan.Analysis
	*a = core.TechAnalysis{}
	a.StaleAreas = StaleAreas(p, plan)
	plan.Stale = len(a.StaleAreas) > 0
	a.Affected = Affected(a.StaleAreas)

	fs := analyze(p, plan, st)
	for _, f := range fs {
		a.Issues = append(a.Issues, f.TechIssue)
	}
	a.Items = len(plan.Items)
	for _, it := range plan.Items {
		if it.None() {
			continue
		}
		if it.Version != "" && !it.VersionVerified {
			a.Unverified++
		}
	}
	if a.Unverified > 0 {
		a.Findings = append(a.Findings, core.TechFinding{Category: "unverified", Severity: "info", Source: "computed",
			Message: fmt.Sprintf("%d versi belum terverifikasi terhadap sumber; periksa sebelum dipakai.", a.Unverified)})
	}
	a.Coverage = coverage(p, plan)
	a.Diverged = diverged(plan, st)
	plan.Evaluation.Outdated = len(a.Diverged) > 0
	for _, area := range core.TechConsistencyAreas {
		note, conflict := "", false
		warn := 0
		for _, f := range fs {
			if f.Check != area {
				continue
			}
			if f.Level == "error" {
				conflict = true
				if note == "" {
					note = f.Message
				}
			} else {
				warn++
			}
		}
		status := core.CheckConsistent
		if conflict {
			status = core.CheckConflict
		} else if warn > 0 {
			note = fmt.Sprintf("%d peringatan", warn)
		}
		a.Checks = append(a.Checks, core.TechCheck{Area: area, Status: status, Note: note, Source: "computed"})
	}
	for _, f := range fs {
		if f.Compat != "" {
			a.Compat = append(a.Compat, core.TechCompat{Area: f.Compat, Status: core.CompatConflict, Between: []string{f.Area}, Note: f.Message, Source: "computed"})
		}
	}
}

// Blocking are the problems that make an answer unusable as a plan.
func Blocking(p *core.Project, plan *core.TechPlan, st *core.TechStack) []core.TechIssue {
	var out []core.TechIssue
	for _, f := range analyze(p, plan, st) {
		if f.Refuse {
			out = append(out, f.TechIssue)
		}
	}
	return out
}

func analyze(p *core.Project, plan *core.TechPlan, st *core.TechStack) []finding {
	c := &collector{}
	if st == nil {
		st = &core.TechStack{}
	}
	validatePlan(c, plan, st)
	validateItems(c, plan)
	validateEvaluation(p, c, plan)
	validateConsistency(c, plan)
	crossChecks(p, c, plan, st)
	return c.out
}

func validatePlan(c *collector, plan *core.TechPlan, st *core.TechStack) {
	if len(plan.Items) == 0 {
		c.issue("error", "", "requirements", true, CodeEmptyPlan, "items", "tulis satu item per area yang dibutuhkan sistem", "rencana tidak memuat keputusan teknologi")
		return
	}
	if plan.Approach == "" || plan.Summary == "" {
		c.issue("error", "", "requirements", true, CodePlanMeta, "approach", "isi approach dan summary", "approach dan summary rencana wajib diisi")
	}
	if plan.Context.Summary == "" || len(plan.Context.Facts) == 0 {
		c.issue("error", "", "requirements", true, CodeContext, "context", "isi context.summary dan minimal satu context.facts {category, statement, source}",
			"context teknis wajib diisi: summary dan fakta kebutuhan yang dipahami (facts)")
	}
	for _, need := range PlatformAreas(st.Platform) {
		it := plan.Item(need.Area)
		switch {
		case it == nil || it.Technology == "":
			c.issue("error", need.Area, "requirements", true, CodePlatformMissing, itemPath(need.Area),
				fmt.Sprintf("tambahkan item %s dengan teknologi implementasi untuk target %s", need.Area, need.Targets),
				"platform %s dipilih, tetapi rencana tidak menentukan teknologi %s", need.Platform, need.Area)
		case it.None():
			// The plan states, with its reason, that the selected platform is
			// not needed. That is a conflict with the platform decision, not
			// a defect of the answer: it lands and blocks approval until the
			// user changes the platform or the decision.
			c.issue("error", need.Area, "requirements", false, CodePlatformDeclined, itemPath(need.Area, "technology"),
				"ubah platform target, atau minta revisi teknologi untuk platform ini",
				"platform %s dipilih, tetapi rencana menilai %s tidak perlu: %s", need.Platform, need.Area, orDash(it.Reasoning))
		}
	}
	for _, area := range StructureAreas {
		it := plan.Item(area)
		switch {
		case it == nil || it.None():
			c.issue("error", area, "architecture", true, CodeStructureValue, itemPath(area), "tambahkan item "+area+" yang menamai rancanganmu untuk sistem ini",
				"rencana harus menentukan %s", area)
		}
	}
	seen := map[string]bool{}
	for _, it := range plan.Items {
		if seen[it.Area] {
			c.issue("error", it.Area, "requirements", true, CodeDuplicateArea, itemPath(it.Area), "gabungkan menjadi satu keputusan",
				"area %s muncul lebih dari sekali; satu keputusan per area", it.Area)
		}
		seen[it.Area] = true
	}
}

func validateItems(c *collector, plan *core.TechPlan) {
	areas := map[string]bool{}
	for _, it := range plan.Items {
		areas[it.Area] = true
	}
	for _, it := range plan.Items {
		if it.Technology == "" {
			c.issue("error", it.Area, "requirements", true, CodeTechnologyEmpty, itemPath(it.Area, "technology"), "tulis teknologinya, atau \""+core.TechNone+"\" dengan alasan",
				"%s: technology kosong; tulis \"%s\" bila area ini tidak butuh teknologi", it.Area, core.TechNone)
			continue
		}
		if !slices.Contains(core.TechPlanGroups, it.Group) {
			c.issue("error", it.Area, "requirements", true, CodeInvalidGroup, itemPath(it.Area, "group"), "pakai salah satu group yang diizinkan",
				"%s: group harus salah satu dari %s", it.Area, strings.Join(core.TechPlanGroups, ", "))
		}
		if missing := MissingItem(it); len(missing) > 0 {
			first, _, _ := strings.Cut(missing[0], " ")
			c.issue("error", it.Area, "requirements", true, CodeRequiredField, itemPath(it.Area, first), "isi "+strings.Join(missing, ", ")+" untuk keputusan ini",
				"%s: field wajib belum diisi: %s", it.Area, strings.Join(missing, ", "))
		}
		for _, d := range it.DependsOn {
			if !areas[d] {
				c.issue("error", it.Area, "requirements", true, CodeUnknownDepend, itemPath(it.Area, "depends_on"), "hapus "+d+" dari depends_on atau tambahkan item "+d,
					"%s: depends_on mengacu ke area %q yang tidak ada di rencana", it.Area, d)
			}
		}
		for i, a := range it.Alternatives {
			if missing := MissingAlternative(a); len(missing) > 0 {
				c.issue("error", it.Area, "requirements", true, CodeAltIncomplete, itemPath(it.Area, fmt.Sprintf("alternatives[%d].%s", i, missing[0])),
					"lengkapi "+strings.Join(missing, ", ")+" khusus sistem ini, atau hapus alternatif yang tidak sungguh layak",
					"%s: alternatif %q harus menjelaskan why, difference, pros, dan cons; yang kosong: %s", it.Area, a.Technology, strings.Join(missing, ", "))
			}
		}
		if it.None() {
			continue
		}
		var soft []string
		if len(it.Tradeoffs) == 0 {
			soft = append(soft, "tradeoffs")
		}
		if it.Operational == "" {
			soft = append(soft, "operational")
		}
		if len(soft) > 0 {
			c.add("warning", it.Area, "requirements", false, "%s: belum menjelaskan %s", it.Area, strings.Join(soft, ", "))
		}
		if it.Confidence == "low" && len(it.Assumptions) == 0 {
			c.add("warning", it.Area, "requirements", false, "%s: kepercayaan rendah tanpa asumsi yang disebutkan; apa yang belum pasti?", it.Area)
		}
	}
}

// refs are everything an evaluation or a decision can point at.
func refs(p *core.Project, plan *core.TechPlan) (exact map[string]bool, names []string) {
	exact = map[string]bool{}
	add := func(s string) {
		if s = strings.ToLower(strings.TrimSpace(s)); s != "" {
			exact[s] = true
		}
	}
	for _, it := range plan.Items {
		add(it.Area)
		if len(it.Technology) >= 3 && !it.None() {
			names = append(names, strings.ToLower(it.Technology))
		}
	}
	for _, r := range p.Requirements {
		add(r.ID)
	}
	for _, f := range p.Features {
		add(f.ID)
		add(f.Name)
	}
	for _, m := range repoplan.Modules(p) {
		add(m.ID)
	}
	if st := p.Stack; st != nil {
		for _, n := range st.Needs {
			add(n)
		}
	}
	for _, g := range core.TechPlanGroups {
		add(g)
	}
	return exact, names
}

func resolves(ref string, exact map[string]bool, names []string) bool {
	r := strings.ToLower(strings.TrimSpace(ref))
	if exact[r] {
		return true
	}
	if exact[AreaID(r)] {
		return true
	}
	if slices.ContainsFunc(names, func(n string) bool { return strings.Contains(r, n) }) {
		return true
	}
	// Evidence written as a sentence ("REQ-001..REQ-034 on areas database,
	// orm") counts when it names a decision area, requirement, feature or
	// module.
	return slices.ContainsFunc(refWord.Split(r, -1), func(w string) bool { return w != "" && exact[w] })
}

var refWord = regexp.MustCompile(`[^a-z0-9_-]+`)

func validateEvaluation(p *core.Project, c *collector, plan *core.TechPlan) {
	ev := plan.Evaluation
	exact, names := refs(p, plan)
	byName := map[string]core.TechCriterion{}
	for _, k := range ev.Criteria {
		byName[k.Name] = k
	}
	var missing, badScore, shortReason, noEvidence []string
	for _, name := range core.TechCriteria {
		k, ok := byName[name]
		switch {
		case !ok:
			missing = append(missing, name)
			continue
		case k.Score < 1 || k.Score > 5:
			badScore = append(badScore, name)
		}
		if len([]rune(k.Reason)) < 20 {
			shortReason = append(shortReason, name)
		}
		if !slices.ContainsFunc(k.Evidence, func(e string) bool { return resolves(e, exact, names) }) {
			noEvidence = append(noEvidence, name)
		}
	}
	for _, g := range []struct {
		list []string
		msg  string
		hint string
	}{
		{missing, "evaluasi belum menilai: %s", "tambahkan kriteria yang belum dinilai"},
		{badScore, "skor evaluasi harus 1-5: %s", "pakai skor bulat 1-5"},
		{shortReason, "alasan skor terlalu singkat (jelaskan secara konkret): %s", "tulis alasan konkret minimal 20 karakter"},
		{noEvidence, "skor tanpa evidence: %s; evidence harus mengacu ke area keputusan, teknologi, requirement, fitur atau modul", "isi evidence dengan area item, id requirement, fitur atau modul"},
	} {
		if len(g.list) > 0 {
			c.issue("error", "", "requirements", true, CodeEvaluation, "evaluation.criteria", g.hint, g.msg, strings.Join(g.list, ", "))
		}
	}
	for _, f := range ev.Findings {
		if f.Severity == "error" {
			c.add("error", f.Area, "requirements", false, "Temuan AI: %s", f.Message)
		}
	}
	for _, x := range plan.Compatibility {
		if x.Status == core.CompatConflict {
			c.add("error", x.Area, "requirements", false, "Konflik kompatibilitas (%s): %s", x.Area, orDash(x.Note))
		}
	}
}

func validateConsistency(c *collector, plan *core.TechPlan) {
	have := map[string]core.TechCheck{}
	for _, k := range plan.Consistency {
		have[k.Area] = k
	}
	var missing []string
	for _, a := range core.TechConsistencyAreas {
		k, ok := have[a]
		if !ok {
			missing = append(missing, a)
			continue
		}
		if k.Status == core.CheckConflict {
			c.add("error", "", a, false, "Konflik konsistensi (%s): %s", a, orDash(k.Note))
		}
	}
	if len(missing) > 0 {
		c.issue("error", "", "requirements", true, CodeConsistency, "consistency", "tambahkan satu entri per area yang belum diperiksa",
			"pemeriksaan konsistensi belum lengkap: %s", strings.Join(missing, ", "))
	}
	if len(plan.Compatibility) == 0 {
		c.issue("error", "", "requirements", true, CodeCompatibility, "compatibility", "periksa kompatibilitas seluruh stack",
			"pemeriksaan kompatibilitas stack belum ada (compatibility)")
	}
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

// background mechanisms need something that runs them.
var jobAreas = []string{"background_jobs", "job_queue", "message_broker"}

// crossChecks finds the conflicts between decisions, and between the
// decisions and the other artifacts, that a plan must not leave in place.
func crossChecks(p *core.Project, c *collector, plan *core.TechPlan, st *core.TechStack) {
	tech := plan.Tech
	has := func(n string) bool { return slices.Contains(st.Needs, n) }
	pl := st.Platform

	if has("transactions") && tech(core.TechDatabase) == "" && tech("data_access") == "" && tech("orm") == "" {
		c.add("error", core.TechDatabase, "data_architecture", false,
			"Ada kebutuhan transaksi, tetapi tidak ada database atau persistence yang menjamin konsistensinya.")
	}
	if has("files") && tech(core.TechStorage) == "" && tech("object_storage") == "" {
		if plan.Item(core.TechStorage) == nil && plan.Item("object_storage") == nil {
			c.add("error", core.TechStorage, "data_architecture", false, "Ada kebutuhan upload file, tetapi rencana tidak punya strategi penyimpanan file.")
		} else {
			c.add("warning", core.TechStorage, "data_architecture", false, "Ada kebutuhan upload file, tetapi file storage diputuskan tidak perlu; pastikan alasannya cukup.")
		}
	}
	if pl.Backend && tech(core.TechDeployment) == "" && tech("hosting") == "" {
		c.add("error", core.TechDeployment, "deployment", false, "Backend ada di platform, tetapi rencana tidak menentukan deployment atau hosting yang menjalankannya.")
	}
	if be := plan.Item(core.TechBackend); be != nil && !be.None() {
		bl := CanonLanguage(be.Language)
		for _, area := range []string{"orm", "data_access", "query_layer"} {
			it := plan.Item(area)
			if it == nil || it.None() || it.Language == "" || bl == "" {
				continue
			}
			if il := CanonLanguage(it.Language); il != bl && !slices.Contains([]string{"sql", "any", "agnostic", "polyglot", "language-agnostic"}, il) {
				c.add("error", area, "data_architecture", false,
					"%s (%s) tidak kompatibel dengan backend %s (%s): ORM atau query layer harus berjalan di runtime backend.", it.Technology, it.Language, be.Technology, be.Language)
				c.out[len(c.out)-1].Compat = "language"
			}
		}
	}
	for _, area := range jobAreas {
		if it := plan.Item(area); it != nil && !it.None() && tech(core.TechWorkers) == "" && !slices.Contains(it.DependsOn, core.TechBackend) {
			c.add("error", area, "architecture", false,
				"Ada %s (%s), tetapi tidak ada worker atau proses yang menjalankan job; tentukan worker atau jelaskan di depends_on bahwa backend yang menjalankannya.", area, it.Technology)
		}
	}
	if it := plan.Item("scheduler"); it != nil && !it.None() && tech(core.TechWorkers) == "" && tech(core.TechBackend) == "" {
		c.add("error", "scheduler", "architecture", false, "Ada scheduler (%s), tetapi tidak ada proses backend atau worker yang menjalankannya.", it.Technology)
	}
	// What Foundry checks on any stack (a decision without a choice, a
	// migration of the code's technology).
	cand := *st
	cand.Plan = plan
	cand.Decisions = Decisions(plan, st)
	for _, is := range stack.Validate(&cand) {
		check := "requirements"
		switch is.Area {
		case core.TechDatabase, core.TechStorage, core.TechCache, core.TechSearch:
			check = "data_architecture"
		case core.TechDeployment:
			check = "deployment"
		case core.TechArchitecture, core.TechRepository, core.TechMessaging, core.TechWorkers:
			check = "architecture"
		}
		c.add(is.Level, is.Area, check, false, "%s", is.Message)
	}
	// The folder plan and the structure decision are other artifacts.
	if s := p.Structure; s != nil && s.Mode != core.StructureExisting {
		if repo := s.Repo; repo != nil {
			stackLangs := map[string]bool{}
			for _, it := range plan.Items {
				if !it.None() && it.Language != "" {
					stackLangs[CanonLanguage(it.Language)] = true
				}
			}
			if len(stackLangs) > 0 {
				for _, l := range FileLanguages(repo) {
					if !stackLangs[l] {
						c.add("warning", "repository", "folder_structure", false,
							"Rencana struktur folder memuat file %s, tetapi teknologi yang dipilih tidak memakai bahasa itu.", l)
					}
				}
			}
		}
	}
}

// coverage maps the requirements, features, modules and needs the decisions
// say they serve to the areas that serve them.
func coverage(p *core.Project, plan *core.TechPlan) []core.TechCoverage {
	exact, _ := refs(p, plan)
	byRef := map[string][]string{}
	var order []string
	for _, it := range plan.Items {
		if it.None() {
			continue
		}
		for _, f := range it.Fulfills {
			k := strings.ToLower(strings.TrimSpace(f))
			if !exact[k] || slices.Contains(byRef[k], it.Area) {
				continue
			}
			if _, ok := byRef[k]; !ok {
				order = append(order, k)
			}
			byRef[k] = append(byRef[k], it.Area)
		}
	}
	slices.Sort(order)
	var out []core.TechCoverage
	for _, k := range order {
		out = append(out, core.TechCoverage{Ref: k, Areas: byRef[k]})
	}
	return out
}

// diverged are the areas where the stack's current choice is not what the
// plan reasoned about: the user changed it after the plan was written.
func diverged(plan *core.TechPlan, st *core.TechStack) []string {
	var out []string
	if st == nil {
		return nil
	}
	for _, it := range plan.Items {
		d := st.Decision(it.Area)
		if d == nil || d.Override == nil {
			continue
		}
		if !strings.EqualFold(d.Choice(), it.Technology) {
			out = append(out, it.Area)
		}
	}
	return out
}
