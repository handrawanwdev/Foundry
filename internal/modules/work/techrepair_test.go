package work

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/techplan"
)

// parkOS is the project and the unified answer of a real Blueprint run whose
// technology section was refused with 41 problems (testdata/parkos): the
// schema example of the unified prompt showed alternatives without pros and
// cons, and the plan left out the mobile and desktop platforms.
func parkOS(t *testing.T) (*core.Project, string) {
	t.Helper()
	b, err := os.ReadFile("testdata/parkos/project.json")
	if err != nil {
		t.Fatal(err)
	}
	var p core.Project
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/parkos/plan.txt")
	if err != nil {
		t.Fatal(err)
	}
	p.Stack.PlanRequest = &core.TechPlanRequest{Action: core.TechPlanGenerate}
	return &p, string(raw)
}

// refuse lands the answer and returns the refusal, recorded on the request
// the way the pipeline records it.
func refuse(t *testing.T, p *core.Project, raw string) *techplan.RejectError {
	t.Helper()
	_, err := ApplyTechPlan(p, TechSection(raw))
	var rej *techplan.RejectError
	if !errors.As(err, &rej) {
		t.Fatalf("err = %v, want a validator refusal", err)
	}
	r := p.Stack.PlanRequest
	r.Draft, r.Issues, r.Rejected = rej.Plan, rej.Issues, rej.Error()
	return rej
}

func TestParkOSRefusalReproducesAll41Issues(t *testing.T) {
	p, raw := parkOS(t)
	rej := refuse(t, p, raw)
	if len(rej.Issues) != 41 {
		t.Fatalf("issues = %d, want 41", len(rej.Issues))
	}
	counts := techplan.CodeCounts(rej.Issues)
	if counts[techplan.CodePlatformMissing] != 2 || counts[techplan.CodeAltIncomplete] != 39 {
		t.Fatalf("codes = %v", counts)
	}
	var platforms []string
	paths := map[string]bool{}
	for _, is := range rej.Issues {
		if is.Code == "" || is.Path == "" || is.Hint == "" {
			t.Fatalf("issue without code, path or hint: %+v", is)
		}
		if paths[is.Path] {
			t.Fatalf("two issues on one path %s", is.Path)
		}
		paths[is.Path] = true
		switch is.Code {
		case techplan.CodePlatformMissing:
			platforms = append(platforms, is.Area)
		case techplan.CodeAltIncomplete:
			// Every alternative of this answer had why and difference, and
			// no pros or cons: the shape of the old schema example.
			if !strings.HasSuffix(is.Message, "yang kosong: pros, cons") || !strings.HasSuffix(is.Path, ".pros") {
				t.Fatalf("alternative issue = %+v", is)
			}
		}
	}
	if !slices.Equal(platforms, []string{core.TechMobile, core.TechDesktop}) {
		t.Fatalf("platform issues = %v", platforms)
	}
	for _, want := range []string{"41 masalah", "alternatif belum lengkap 39 di 35 area", "teknologi platform belum ditentukan 2: mobile, desktop"} {
		if !strings.Contains(rej.Error(), want) {
			t.Fatalf("error %q misses %q", rej.Error(), want)
		}
	}
}

func TestParkOSRetryIsTargetedRepairNotSyntaxRepair(t *testing.T) {
	p, raw := parkOS(t)
	p.Dir = t.TempDir()
	if err := os.MkdirAll(p.Dir+"/.foundry/ai-raw", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Dir+"/.foundry/ai-raw/plan.txt", []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	rej := refuse(t, p, raw)
	if got := PlanStrategy(p); got != core.TechAttemptRepair {
		t.Fatalf("strategy = %s", got)
	}
	got := UnifiedPlanPrompt(p)
	if strings.Contains(got, "Repair JSON syntax only") || strings.Contains(got, "Do not change any technology") {
		t.Fatal("a validation refusal was answered with the JSON syntax repair prompt")
	}
	for _, is := range rej.Issues {
		if !strings.Contains(got, is.Path) {
			t.Fatalf("repair prompt misses issue %s", is.Path)
		}
	}
	for _, want := range []string{
		"TARGETED REPAIR", "YOUR LAST ANSWER WAS REFUSED, fix this: 41 problems", "SECTIONS TO PRODUCE: technology",
		"ITEMS TO ADD (missing from the plan): mobile, desktop", "ALTERNATIVES TO COMPLETE",
		"MANDATORY AREAS from the selected target platform", "mobile (target android, ios)", "desktop (target desktop)",
		"- integrations: Port Go", // valid decisions are listed as kept, not asked again
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("repair prompt misses %q", want)
		}
	}
	if strings.Contains(got, `"area":"integrations"`) {
		t.Fatal("a valid decision is sent back for rewriting")
	}
	// The repair asks for far less than the plan it repairs.
	if len(got) > len(raw) {
		t.Fatalf("repair prompt %dB is larger than the refused answer %dB", len(got), len(raw))
	}
}

// parkOSRepair answers the repair the way a model following the prompt
// would: complete alternatives for every area that had incomplete ones, and
// the two missing platform decisions.
func parkOSRepair(t *testing.T, p *core.Project, mutate func(fix map[string]any)) string {
	t.Helper()
	r := p.Stack.PlanRequest
	alts := map[string][]core.TechAlternative{}
	sc := techplan.RepairScope(r.Draft, r.Issues)
	for a := range sc.AltOnly {
		var list []core.TechAlternative
		for _, x := range r.Draft.Item(a).Alternatives {
			x.Pros = []string{"Kelebihan " + x.Technology + " untuk ParkOS"}
			x.Cons = []string{"Kekurangan " + x.Technology + " untuk ParkOS"}
			list = append(list, x)
		}
		alts[a] = list
	}
	platform := func(area, tech, lang string) core.TechItem {
		return core.TechItem{Area: area, Group: core.TechPlanStack, Technology: tech, Language: lang, Purpose: "Aplikasi " + area + " untuk petugas",
			Fulfills: []string{"REQ-001"}, Driver: core.DriverRequirement, Reasoning: "Platform " + area + " dipilih user; peran belum pasti",
			Pros: []string{"Berbagi TypeScript dengan web"}, Cons: []string{"Perlu build per platform"}, Confidence: "low",
			Assumptions: []string{"Peran " + area + " sama dengan web"}}
	}
	fix := map[string]any{
		"alternatives": alts,
		"set":          []core.TechItem{platform(core.TechMobile, "React Native + TypeScript", "TypeScript"), platform(core.TechDesktop, "Tauri + TypeScript", "TypeScript")},
	}
	if mutate != nil {
		mutate(fix)
	}
	b, _ := json.Marshal(map[string]any{"technology": fix})
	return "```json\n" + string(b) + "\n```"
}

func TestParkOSTargetedRepairLandsAndKeepsValidDecisions(t *testing.T) {
	p, raw := parkOS(t)
	refuse(t, p, raw)
	draft := p.Stack.PlanRequest.Draft.Clone()
	if _, err := ApplyTechPlan(p, TechSection(parkOSRepair(t, p, nil))); err != nil {
		t.Fatalf("repair refused: %v", err)
	}
	plan := p.Stack.Plan
	if plan == nil || p.Stack.PlanRequest != nil {
		t.Fatalf("plan=%v request=%+v", plan != nil, p.Stack.PlanRequest)
	}
	if plan.Tech(core.TechMobile) != "React Native + TypeScript" || plan.Tech(core.TechDesktop) != "Tauri + TypeScript" {
		t.Fatalf("platform decisions = %q %q", plan.Tech(core.TechMobile), plan.Tech(core.TechDesktop))
	}
	for _, old := range draft.Items {
		it := plan.Item(old.Area)
		if it == nil || it.Technology != old.Technology || it.Reasoning != old.Reasoning || !slices.Equal(it.Pros, old.Pros) {
			t.Fatalf("decision %s changed by the repair: %+v", old.Area, it)
		}
		for i, a := range it.Alternatives {
			if len(techplan.MissingAlternative(a)) > 0 || a.Why != old.Alternatives[i].Why {
				t.Fatalf("%s alternative %d = %+v", old.Area, i, a)
			}
		}
	}
	// Untouched decisions are byte-identical apart from what Foundry computes.
	a, _ := json.Marshal(draft.Item("integrations"))
	b, _ := json.Marshal(plan.Item("integrations"))
	if string(a) != string(b) {
		t.Fatalf("valid decision changed:\n%s\n%s", a, b)
	}
}

func TestRepairIgnoresChangedChoiceAndOutOfScopeItems(t *testing.T) {
	p, raw := parkOS(t)
	refuse(t, p, raw)
	answer := parkOSRepair(t, p, func(fix map[string]any) {
		set := fix["set"].([]core.TechItem)
		fe := *p.Stack.PlanRequest.Draft.Item(core.TechFrontend)
		fe.Technology = "Angular" // only its alternatives were incomplete
		in := *p.Stack.PlanRequest.Draft.Item("integrations")
		in.Technology = "Apache Camel" // nothing was wrong with it
		fix["set"] = append(set, fe, in)
		delete(fix["alternatives"].(map[string][]core.TechAlternative), core.TechFrontend)
	})
	_, err := ApplyTechPlan(p, TechSection(answer))
	var rej *techplan.RejectError
	if !errors.As(err, &rej) || rej.Merge == nil {
		t.Fatalf("err = %v", err)
	}
	if len(rej.Issues) != 1 || rej.Issues[0].Area != core.TechFrontend {
		t.Fatalf("remaining issues = %+v", rej.Issues)
	}
	if rej.Plan.Tech(core.TechFrontend) != "React + TypeScript + Vite" || !strings.HasPrefix(rej.Plan.Tech("integrations"), "Port Go") {
		t.Fatalf("frontend=%q integrations=%q", rej.Plan.Tech(core.TechFrontend), rej.Plan.Tech("integrations"))
	}
	ignored := strings.Join(rej.Merge.Ignored, "\n")
	if !strings.Contains(ignored, "frontend: teknologi") || !strings.Contains(ignored, "integrations: tidak bermasalah") {
		t.Fatalf("ignored = %v", rej.Merge.Ignored)
	}
	// Progress is kept: the next draft has the fixes of this attempt.
	if rej.Plan.Tech(core.TechMobile) == "" || len(techplan.MissingAlternative(rej.Plan.Item(core.TechBackend).Alternatives[0])) > 0 {
		t.Fatal("the fixes of a partly refused repair were lost")
	}
}

func TestRepairAnswerThatIsNotJSONKeepsTheDraft(t *testing.T) {
	p, raw := parkOS(t)
	refuse(t, p, raw)
	if _, err := ApplyTechPlan(p, TechSection("maaf, saya tidak bisa")); err == nil || !strings.HasPrefix(err.Error(), "output AI") {
		t.Fatalf("err = %v", err)
	}
	if PlanStrategy(p) != core.TechAttemptRepair || p.Stack.PlanRequest.Draft == nil {
		t.Fatal("an unreadable repair answer dropped the draft")
	}
}

// techItem is a complete decision for the contract tests.
func techItem(area, tech string) core.TechItem {
	return core.TechItem{Area: area, Technology: tech, Purpose: "p", Fulfills: []string{"REQ-001"}, Driver: core.DriverRequirement,
		Reasoning: "r", Pros: []string{"a"}, Cons: []string{"b"}, Confidence: "high"}
}

func TestContractNamesEveryRequirementTheValidatorChecks(t *testing.T) {
	st := &core.TechStack{Platform: core.TechPlatform{Web: true, Mobile: true, Android: true, Desktop: true, Backend: true}}
	got := techplan.Contract(st)
	for _, want := range append(append(append([]string{
		"frontend (target web)", "mobile (target android)", "desktop (target desktop)", "backend (target backend)",
		core.ArchModular, core.RepoMonorepo,
	}, techplan.AlternativeFields...), core.TechCriteria...), core.TechConsistencyAreas...) {
		if !strings.Contains(got, want) {
			t.Fatalf("contract misses %q:\n%s", want, got)
		}
	}
	for _, f := range techplan.ChosenFields {
		if !strings.Contains(got, f) {
			t.Fatalf("contract misses item field %s", f)
		}
	}
}

// The schema example is what a small model copies: every alternative object
// in it must carry every field the validator requires.
func TestUnifiedPromptExampleAlternativesAreComplete(t *testing.T) {
	p := unifiedProject()
	got := UnifiedPlanPrompt(p)
	objs := regexp.MustCompile(`"alternatives":\[\{"technology"[^}]*\}\]`).FindAllString(got, -1)
	var tech []string
	for _, o := range objs {
		if strings.Contains(o, `"why"`) {
			tech = append(tech, o)
		}
	}
	if len(tech) == 0 {
		t.Fatal("no technology alternative example in the prompt")
	}
	for _, o := range tech {
		for _, f := range techplan.AlternativeFields {
			if !strings.Contains(o, `"`+f+`"`) {
				t.Fatalf("example alternative %s misses %s", o, f)
			}
		}
	}
	if !strings.Contains(got, "CONTRACT (Foundry's validator refuses") {
		t.Fatal("unified prompt misses the contract")
	}
}

func TestValidatorAlternativeAndPlatformCases(t *testing.T) {
	base := func() *core.TechPlan {
		alt := core.TechAlternative{Technology: "Go + Gin", Why: "w", Difference: "d", Pros: []string{"p"}, Cons: []string{"c"}}
		be := techItem(core.TechBackend, "Elixir + Phoenix")
		be.Alternatives = []core.TechAlternative{alt}
		return &core.TechPlan{Items: []core.TechItem{be, techItem(core.TechArchitecture, core.ArchModular), techItem(core.TechRepository, core.RepoSimple)}}
	}
	st := &core.TechStack{Platform: core.TechPlatform{Backend: true}}
	codes := func(plan *core.TechPlan, st *core.TechStack) (refused, kept []core.TechIssue) {
		techplan.Normalize(plan)
		p := &core.Project{Stack: st}
		refused = techplan.Blocking(p, plan, st)
		techplan.Refresh(p, plan)
		return refused, plan.Errors()
	}
	for _, field := range techplan.AlternativeFields {
		plan := base()
		a := &plan.Items[0].Alternatives[0]
		switch field {
		case "why":
			a.Why = ""
		case "difference":
			a.Difference = ""
		case "pros":
			a.Pros = nil
		case "cons":
			a.Cons = nil
		}
		refused, _ := codes(plan, st)
		i := slices.IndexFunc(refused, func(is core.TechIssue) bool { return is.Code == techplan.CodeAltIncomplete })
		if i < 0 || refused[i].Path != "items.backend.alternatives[0]."+field {
			t.Fatalf("missing %s: issues = %+v", field, refused)
		}
	}

	// A selected platform without a decision is refused; one the plan says
	// is not needed lands as a conflict that blocks approval.
	mob := &core.TechStack{Platform: core.TechPlatform{Backend: true, Mobile: true, IOS: true}}
	refused, _ := codes(base(), mob)
	if !slices.ContainsFunc(refused, func(is core.TechIssue) bool {
		return is.Code == techplan.CodePlatformMissing && is.Area == core.TechMobile
	}) {
		t.Fatalf("missing mobile not refused: %+v", refused)
	}
	plan := base()
	none := techItem(core.TechMobile, core.TechNone)
	none.Reasoning = "Requirement hanya menyebut UI responsif"
	plan.Items = append(plan.Items, none)
	refused, kept := codes(plan, mob)
	if slices.ContainsFunc(refused, func(is core.TechIssue) bool { return is.Area == core.TechMobile }) {
		t.Fatalf("declined platform refused: %+v", refused)
	}
	if !slices.ContainsFunc(kept, func(is core.TechIssue) bool { return is.Code == techplan.CodePlatformDeclined }) {
		t.Fatalf("declined platform does not block approval: %+v", kept)
	}

	// An area the system does not need is stated, with its reason.
	plan = base()
	cache := core.TechItem{Area: core.TechCache, Technology: core.TechNone, Driver: core.DriverEngineering, Reasoning: "Data kecil", Confidence: "high"}
	plan.Items = append(plan.Items, cache)
	if refused, _ := codes(plan, st); slices.ContainsFunc(refused, func(is core.TechIssue) bool { return is.Area == core.TechCache }) {
		t.Fatalf("an unneeded area with its reason is refused: %+v", refused)
	}
}

func TestValidatorFlagsIncompatibleORMWithoutRefusing(t *testing.T) {
	be := techItem(core.TechBackend, "Go + Gin")
	be.Language = "Go"
	orm := techItem("orm", "Prisma")
	orm.Language, orm.Group = "TypeScript", core.TechPlanData
	plan := &core.TechPlan{Items: []core.TechItem{be, orm, techItem(core.TechArchitecture, core.ArchSimple), techItem(core.TechRepository, core.RepoSimple)}}
	techplan.Normalize(plan)
	st := &core.TechStack{Platform: core.TechPlatform{Backend: true}}
	p := &core.Project{Stack: st}
	if refused := techplan.Blocking(p, plan, st); slices.ContainsFunc(refused, func(is core.TechIssue) bool { return is.Area == "orm" }) {
		t.Fatalf("a conflict between decisions refuses the answer: %+v", refused)
	}
	techplan.Refresh(p, plan)
	if !slices.ContainsFunc(plan.Errors(), func(is core.TechIssue) bool { return is.Area == "orm" && is.Code == techplan.CodeConflict }) {
		t.Fatalf("ORM incompatibility not reported: %+v", plan.Errors())
	}
}

func TestParserKeepsAlternativeFieldsAndDuplicates(t *testing.T) {
	raw := "```json\n" + `{"items":[{"area":"backend","technology":"Go","alternatives":[
	 {"technology":"Rust","why":"w","difference":"d","pros":"cepat","cons":["sulit"]},
	 {"technology":"rust","why":"w2","difference":"d2","pros":["x"],"cons":["y"]}]}]}` + "\n```"
	var plan core.TechPlan
	if err := ExtractJSONFunc(raw, &plan, func() bool { return len(plan.Items) > 0 }); err != nil {
		t.Fatal(err)
	}
	techplan.Normalize(&plan)
	alts := plan.Items[0].Alternatives
	if len(alts) != 1 || alts[0].Why != "w" || !slices.Equal(alts[0].Pros, []string{"cepat"}) || !slices.Equal(alts[0].Cons, []string{"sulit"}) {
		t.Fatalf("alternatives = %+v", alts)
	}
}
