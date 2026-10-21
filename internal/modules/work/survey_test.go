package work

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func TestSurveyConflicts(t *testing.T) {
	newProject := func(s *core.Survey) *core.Project {
		return &core.Project{Survey: s, Structure: &core.Structure{Mode: core.StructureNew}}
	}
	cases := []struct {
		name     string
		s        *core.Survey
		critical int
		other    int
	}{
		{"none", &core.Survey{Stage: core.SurveyStageProd, Architecture: core.SurveyArchServices, Priorities: []string{core.SurveyPriorityOperations}}, 0, 0},
		{"open answers", &core.Survey{}, 0, 0},
		{"existing without code", &core.Survey{Architecture: core.SurveyArchExisting}, 1, 0},
		{"services prototype", &core.Survey{Stage: core.SurveyStagePrototype, Architecture: core.SurveyArchServices}, 1, 0},
		{"prototype operations", &core.Survey{Stage: core.SurveyStagePrototype, Priorities: []string{core.SurveyPriorityScalability, core.SurveyPriorityOperations}}, 0, 1},
		{"system with screens", &core.Survey{Users: core.SurveyUsersSystem, Kinds: []string{core.SurveyKindAPI, core.SurveyKindWeb, core.SurveyKindMobile}}, 0, 1},
		{"system api only", &core.Survey{Users: core.SurveyUsersSystem, Kinds: []string{core.SurveyKindAPI}}, 0, 0},
	}
	for _, c := range cases {
		ds := SurveyConflicts(newProject(c.s))
		crit := 0
		for _, d := range ds {
			if d.Kind != core.DecisionConflict || !d.Open() || strings.TrimSpace(d.Text) == "" || !strings.Contains(d.Text, "Pilihan:") {
				t.Errorf("%s: decision %+v must be an open conflict with options", c.name, d)
			}
			if d.Critical {
				crit++
			}
		}
		if crit != c.critical || len(ds)-crit != c.other {
			t.Errorf("%s: %d critical %d other, want %d %d: %+v", c.name, crit, len(ds)-crit, c.critical, c.other, ds)
		}
	}
	// The existing project has code to follow.
	p := newProject(&core.Survey{Architecture: core.SurveyArchExisting})
	p.Structure.Mode = core.StructureExisting
	if ds := SurveyConflicts(p); len(ds) != 0 {
		t.Fatalf("existing project: %+v", ds)
	}
}

func TestApplyDiscoveryRecordsSurveyDecisions(t *testing.T) {
	p := &core.Project{Story: "Aplikasi klinik", Structure: &core.Structure{Mode: core.StructureNew},
		Survey: &core.Survey{Kinds: []string{core.SurveyKindEmbedded}, Stage: core.SurveyStagePrototype, Architecture: core.SurveyArchServices}}
	if err := ApplyDiscovery(p, thinDiscovery); err != nil {
		t.Fatal(err)
	}
	var conflict *core.Decision
	for i := range p.Decisions {
		if p.Decisions[i].Critical && strings.Contains(p.Decisions[i].Text, "prototype") {
			conflict = &p.Decisions[i]
		}
	}
	if conflict == nil || conflict.ID == "" {
		t.Fatalf("decisions %+v", p.Decisions)
	}
	// A chosen platform is kept next to the ones the story gives.
	var kinds []string
	for _, t := range p.Platform.Targets {
		kinds = append(kinds, t.Kind)
	}
	if !strings.Contains(strings.Join(kinds, ","), core.PlatformEmbedded) || !strings.Contains(strings.Join(kinds, ","), core.PlatformWeb) {
		t.Fatalf("platform %v", kinds)
	}
}

func TestDiscoveryPromptSeparatesAnswers(t *testing.T) {
	p := &core.Project{Story: "Aplikasi klinik", Structure: &core.Structure{Mode: core.StructureNew},
		Survey: &core.Survey{Users: core.SurveyUsersMixed, Priorities: []string{core.SurveyPrioritySecurity},
			Constraints: "Wajib PostgreSQL\nTidak boleh cloud luar negeri", Architecture: core.SurveyArchServices, Stage: core.SurveyStagePrototype}}
	got := DiscoveryPrompt(p)
	for _, want := range []string{
		"- Platform: open", "- Users: user: Campuran pengguna dan admin", "- Delivery target: Prototype —",
		"- Architecture: user: Beberapa service terpisah", "- Quality priorities: user: Keamanan dan kontrol akses",
		"  Wajib PostgreSQL\n  Tidak boleh cloud luar negeri", "Foundry already records these conflicts", "Never turn an open answer into a constraint",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt misses %q", want)
		}
	}
	for _, old := range []string{"Latency budget", "Webhook", "Set performance.budget from the survey latency"} {
		if strings.Contains(got, old) {
			t.Errorf("prompt still forces %q", old)
		}
	}
	p.Survey = &core.Survey{}
	got = DiscoveryPrompt(p)
	if !strings.Contains(got, "- Delivery target: MVP (default)") || !strings.Contains(got, "- Mandatory constraints: none given") ||
		strings.Contains(got, ": user: ") || strings.Count(got, ": open\n") != 4 {
		t.Errorf("open survey prompt:\n%s", got)
	}
}

func TestWorkflowPromptFollowsDelivery(t *testing.T) {
	p := &core.Project{Name: "Klinik", Story: "Aplikasi klinik", Survey: &core.Survey{Stage: core.SurveyStageProd,
		Priorities: []string{core.SurveyPriorityReliability}, Constraints: "Wajib PostgreSQL"}}
	got := WorkflowPrompt(p, nil)
	for _, want := range []string{"DELIVERY TARGET: Production-ready", "security hardening", "observability",
		"QUALITY PRIORITIES (user): Reliabilitas dan konsistensi data", "MANDATORY CONSTRAINTS (user):\n  Wajib PostgreSQL"} {
		if !strings.Contains(got, want) {
			t.Errorf("workflow prompt misses %q", want)
		}
	}
	p.Survey = &core.Survey{Stage: core.SurveyStagePrototype}
	got = WorkflowPrompt(p, nil)
	if !strings.Contains(got, "no deployment, observability or hardening commands") || strings.Contains(got, "MANDATORY CONSTRAINTS") {
		t.Errorf("prototype workflow prompt:\n%s", got)
	}
}

func TestPerformanceBudgetNeedsAReason(t *testing.T) {
	for _, c := range []struct {
		s    *core.Survey
		want string
	}{
		{&core.Survey{}, "Tanpa target angka khusus"},
		{&core.Survey{Priorities: []string{core.SurveyPriorityPerformance}}, "Prioritas performa"},
		{&core.Survey{Latency: core.SurveyLatency200}, "< 200 ms p95"},
	} {
		b := &core.Blueprint{}
		completePerformance(b, &core.Project{Survey: c.s}, nil)
		if !strings.HasPrefix(b.Performance.Budget, c.want) {
			t.Errorf("%+v: budget %q", c.s, b.Performance.Budget)
		}
	}
}
