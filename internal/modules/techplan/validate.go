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
	c.out = append(c.out, finding{TechIssue: core.TechIssue{Level: level, Area: area, Message: fmt.Sprintf(f, a...)}, Refuse: refuse, Check: check})
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
		if !stack.Known(it.Area, it.Technology) {
			a.Outside++
		}
		if it.Version != "" && !it.VersionVerified {
			a.Unverified++
		}
	}
	if a.Outside > 0 {
		a.Findings = append(a.Findings, core.TechFinding{Category: "unverified", Severity: "info", Source: "computed",
			Message: fmt.Sprintf("%d teknologi tidak ada di katalog Foundry; kompatibilitasnya bergantung pada tinjauan AI, bukan data katalog.", a.Outside)})
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
		c.add("error", "", "requirements", true, "rencana tidak memuat keputusan teknologi")
		return
	}
	if plan.Approach == "" || plan.Summary == "" {
		c.add("error", "", "requirements", true, "approach dan summary rencana wajib diisi")
	}
	if plan.Context.Summary == "" || len(plan.Context.Facts) == 0 {
		c.add("error", "", "requirements", true, "context teknis wajib diisi: summary dan fakta kebutuhan yang dipahami (facts)")
	}
	pl := st.Platform
	for _, need := range []struct {
		on   bool
		area string
		name string
	}{
		{pl.Web, core.TechFrontend, "web"}, {pl.Mobile, core.TechMobile, "mobile"}, {pl.Desktop, core.TechDesktop, "desktop"},
		{pl.Backend, core.TechBackend, "backend"}, {pl.CLI, core.TechCLI, "CLI"},
	} {
		if it := plan.Item(need.area); need.on && (it == nil || it.None()) {
			c.add("error", need.area, "requirements", true, "platform %s dipilih, tetapi rencana tidak menentukan teknologi %s", need.name, need.area)
		}
	}
	for _, area := range []string{core.TechArchitecture, core.TechRepository} {
		it := plan.Item(area)
		switch {
		case it == nil || it.None():
			c.add("error", area, "architecture", true, "rencana harus menentukan %s", area)
		case st.Mode == core.StructureExisting:
		case area == core.TechArchitecture && !slices.Contains([]string{core.ArchSimple, core.ArchModular, core.ArchMicroservices}, it.Technology):
			c.add("error", area, "architecture", true, "arsitektur harus salah satu dari: %s, %s, %s (bukan %q)", core.ArchSimple, core.ArchModular, core.ArchMicroservices, it.Technology)
		case area == core.TechRepository && !slices.Contains([]string{core.RepoSimple, core.RepoMonorepo}, it.Technology):
			c.add("error", area, "architecture", true, "repository harus salah satu dari: %s, %s (bukan %q)", core.RepoSimple, core.RepoMonorepo, it.Technology)
		}
	}
	seen := map[string]bool{}
	for _, it := range plan.Items {
		if seen[it.Area] {
			c.add("error", it.Area, "requirements", true, "area %s muncul lebih dari sekali; satu keputusan per area", it.Area)
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
			c.add("error", it.Area, "requirements", true, "%s: technology kosong; tulis \"%s\" bila area ini tidak butuh teknologi", it.Area, core.TechNone)
			continue
		}
		if !slices.Contains(core.TechPlanGroups, it.Group) {
			c.add("error", it.Area, "requirements", true, "%s: group harus salah satu dari %s", it.Area, strings.Join(core.TechPlanGroups, ", "))
		}
		var missing []string
		need := func(ok bool, field string) {
			if !ok {
				missing = append(missing, field)
			}
		}
		need(it.Reasoning != "", "reasoning")
		need(slices.Contains(core.TechDrivers, it.Driver), "driver ("+strings.Join(core.TechDrivers, "|")+")")
		need(slices.Contains(core.TechConfidence, it.Confidence), "confidence (high|medium|low)")
		if !it.None() {
			need(it.Purpose != "", "purpose")
			need(len(it.Fulfills) > 0, "fulfills")
			need(len(it.Pros) > 0, "pros")
			need(len(it.Cons) > 0, "cons")
		}
		if len(missing) > 0 {
			c.add("error", it.Area, "requirements", true, "%s: field wajib belum diisi: %s", it.Area, strings.Join(missing, ", "))
		}
		for _, d := range it.DependsOn {
			if !areas[d] {
				c.add("error", it.Area, "requirements", true, "%s: depends_on mengacu ke area %q yang tidak ada di rencana", it.Area, d)
			}
		}
		for _, a := range it.Alternatives {
			if a.Why == "" || a.Difference == "" || len(a.Pros) == 0 || len(a.Cons) == 0 {
				c.add("error", it.Area, "requirements", true, "%s: alternatif %q harus menjelaskan why, difference, pros, dan cons", it.Area, a.Technology)
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
		if it.Driver == core.DriverProfile && len(it.Fulfills) == 0 {
			c.add("warning", it.Area, "requirements", false, "%s: alasannya hanya profil proyek; profil bukan kebutuhan", it.Area)
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
	for _, n := range stack.KnownNeeds {
		add(n)
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
	}{
		{missing, "evaluasi belum menilai: %s"},
		{badScore, "skor evaluasi harus 1-5: %s"},
		{shortReason, "alasan skor terlalu singkat (jelaskan secara konkret): %s"},
		{noEvidence, "skor tanpa evidence: %s; evidence harus mengacu ke area keputusan, teknologi, requirement, fitur atau modul"},
	} {
		if len(g.list) > 0 {
			c.add("error", "", "requirements", true, g.msg, strings.Join(g.list, ", "))
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
		c.add("error", "", "requirements", true, "pemeriksaan konsistensi belum lengkap: %s", strings.Join(missing, ", "))
	}
	if len(plan.Compatibility) == 0 {
		c.add("error", "", "requirements", true, "pemeriksaan kompatibilitas stack belum ada (compatibility)")
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

var staticHost = []string{"static", "cdn", "github pages", "netlify", "cloudflare pages", "s3 bucket"}
var runsCode = []string{"server", "container", "docker", "function", "lambda", "worker", "kubernetes", "k8s", "vm", "ecs", "fly", "render", "railway", "heroku", "app engine", "cloud run", "paas", "compose"}

var sessionWords = []string{"session", "token", "jwt", "cookie", "oauth", "oidc", "openid", "passkey", "saml", "webauthn", "api key", "bearer"}

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
	if pl.Backend {
		dep := tech(core.TechDeployment)
		if dep == "" && tech("hosting") == "" {
			c.add("error", core.TechDeployment, "deployment", false, "Backend ada di platform, tetapi rencana tidak menentukan deployment atau hosting yang menjalankannya.")
		} else {
			d := strings.ToLower(dep + " " + tech("hosting"))
			if slices.ContainsFunc(staticHost, func(w string) bool { return strings.Contains(d, w) }) &&
				!slices.ContainsFunc(runsCode, func(w string) bool { return strings.Contains(d, w) }) {
				c.add("error", core.TechDeployment, "deployment", false,
					"Deployment %q tidak menjalankan kode server, padahal ada backend %s dan arsitektur %s.", orDash(strings.TrimSpace(dep+" "+tech("hosting"))), orDash(tech(core.TechBackend)), orDash(tech(core.TechArchitecture)))
			}
		}
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
	for _, area := range []string{"authentication", "auth"} {
		it := plan.Item(area)
		if it == nil || it.None() {
			continue
		}
		strategy := plan.Item("session") != nil || plan.Item("token") != nil
		text := strings.ToLower(it.Technology + " " + it.Purpose + " " + it.Reasoning + " " + it.Integration)
		if !strategy && !slices.ContainsFunc(sessionWords, func(w string) bool { return strings.Contains(text, w) }) {
			c.add("warning", area, "security", false, "Authentication %s belum menentukan strategi session atau token.", it.Technology)
		}
	}
	if fe := strings.ToLower(tech(core.TechFrontend)); has("seo") && fe != "" &&
		slices.ContainsFunc([]string{"vite", "create-react-app", "cra", "spa"}, func(w string) bool { return strings.Contains(fe, w) }) &&
		!slices.ContainsFunc([]string{"next", "nuxt", "remix", "astro", "sveltekit", "ssr", "ssg", "prerender", "gatsby", "qwik"}, func(w string) bool { return strings.Contains(fe, w) }) {
		c.add("warning", core.TechFrontend, "requirements", false, "Ada kebutuhan SEO, tetapi %s dirender di browser; tambahkan SSR/prerender atau pilih framework lain.", tech(core.TechFrontend))
	}
	// The catalog's own compatibility rules apply to the areas it knows;
	// outside the catalog the plan's review is what counts.
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
