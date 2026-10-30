package pipeline_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

const surveyStory = "Saya ingin membuat aplikasi klinik untuk janji temu pasien"

func submitSurvey(t *testing.T, s *pipeline.Service, sv core.Survey) *core.Project {
	t.Helper()
	if _, err := s.SubmitStoryWith(surveyStory, &sv); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if err := s.StartBlueprint(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	mustJob(t, p, work.KindDesign, core.JobDone)
	return p
}

func surveyConflicts(p *core.Project) (critical []string) {
	for _, d := range p.Decisions {
		if d.Kind == core.DecisionConflict && d.Critical && d.Open() {
			critical = append(critical, d.ID)
		}
	}
	return critical
}

func docBody(p *core.Project, id string) string {
	for _, d := range spec.Artifacts(p) {
		if d.ID == id {
			return d.Body
		}
	}
	return ""
}

// The answers are stored, reloaded unchanged, and bind the stack, the
// decisions and the blueprint; new answers replace them.
func TestSurveyBindsBlueprint(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	p := submitSurvey(t, s, core.Survey{Kinds: []string{"web", "api"}, Users: core.SurveyUsersMixed, Stage: core.SurveyStagePrototype,
		Architecture: core.SurveyArchServices, Priorities: []string{core.SurveyPrioritySecurity}, Constraints: "Database wajib PostgreSQL"})

	want := &core.Survey{What: surveyStory, Kinds: []string{"web", "api"}, Users: core.SurveyUsersMixed, Stage: core.SurveyStagePrototype,
		Architecture: core.SurveyArchServices, Priorities: []string{core.SurveyPrioritySecurity}, Constraints: "Database wajib PostgreSQL"}
	if !reflect.DeepEqual(p.Survey, want) {
		t.Fatalf("survey after reload = %+v", p.Survey)
	}
	// The user's architecture wins over Foundry's prototype default.
	if d := p.Stack.Decision(core.TechArchitecture); d == nil || d.Choice() != core.ArchMicroservices || d.Source != core.SourcePreference {
		t.Fatalf("architecture = %+v", d)
	}
	if d := p.Stack.Decision(core.TechDatabase); d == nil || d.Choice() != "PostgreSQL" || d.Source != core.SourcePreference {
		t.Fatalf("database = %+v", d)
	}
	// Separate services for a prototype is a conflict the user decides before finalize.
	crit := surveyConflicts(p)
	if len(crit) != 1 || !slices.ContainsFunc(pipeline.Blockers(p), func(b string) bool { return strings.Contains(b, crit[0]) }) {
		t.Fatalf("conflicts %v blockers %v", crit, pipeline.Blockers(p))
	}
	doc := docBody(p, core.ArtifactSurvey)
	for _, w := range []string{"| Bagian | Keputusan | Sumber |", "| Arsitektur | Beberapa service terpisah | Pilihan pengguna |",
		"| Pengguna | Campuran pengguna dan admin | Pilihan pengguna |", "| Batasan wajib | Database wajib PostgreSQL | Pilihan pengguna |"} {
		if !strings.Contains(doc, w) {
			t.Errorf("survey doc misses %q", w)
		}
	}

	// Changed answers: the conflict goes, the architecture follows.
	p = submitSurvey(t, s, core.Survey{Kinds: []string{"web", "api"}, Users: core.SurveyUsersMixed, Stage: core.SurveyStageMVP,
		Architecture: core.SurveyArchModular})
	if p.Survey.Stage != core.SurveyStageMVP || p.Survey.Constraints != "" || len(surveyConflicts(p)) != 0 {
		t.Fatalf("survey %+v conflicts %v", p.Survey, surveyConflicts(p))
	}
	if d := p.Stack.Decision(core.TechArchitecture); d.Choice() != core.ArchModular || d.Source != core.SourcePreference {
		t.Fatalf("architecture after change = %+v", d)
	}
	if d := p.Stack.Decision(core.TechDatabase); d.Source == core.SourcePreference {
		t.Fatalf("removed constraint still binds: %+v", d)
	}
}

// Left to Foundry, nothing is presented as the user's choice.
func TestSurveyOpenAnswers(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	p := submitSurvey(t, s, core.Survey{})
	doc := docBody(p, core.ArtifactSurvey)
	if strings.Contains(doc, "Pilihan pengguna") || !strings.Contains(doc, "| Target delivery | MVP:") ||
		!strings.Contains(doc, "Rekomendasi Foundry") {
		t.Fatalf("survey doc:\n%s", doc)
	}
	if d := p.Stack.Decision(core.TechArchitecture); d.Source == core.SourcePreference {
		t.Fatalf("architecture = %+v", d)
	}
}

func TestSurveyRefusesUnknownAnswer(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	if _, err := s.SubmitStoryWith(surveyStory, &core.Survey{Users: "everyone"}); err == nil || !strings.Contains(err.Error(), "pengguna") {
		t.Fatalf("err = %v", err)
	}
	if p := load(t, s); p.Stage != core.StageTell || p.Survey != nil {
		t.Fatalf("refused survey changed the project: stage %s survey %+v", p.Stage, p.Survey)
	}
}

// The survey has its own document: the requirements, which commands and
// written artifacts of older projects refer to, keep their content and hash,
// so upgrading marks nothing for review.
func TestSurveyLeavesRequirementsHash(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	submitSurvey(t, s, core.Survey{Users: core.SurveyUsersInternal, Constraints: "Wajib PostgreSQL"})
	if err := workflowReady(s); err != nil {
		t.Fatal(err)
	}
	p := load(t, s)
	if r := pipeline.CheckWorkflow(p); len(r.Review) > 0 {
		t.Fatalf("fresh workflow needs review: %v", r.Review)
	}
	before := docBody(p, "requirements")
	p.Survey = nil // as an older project without survey decisions rendered
	if after := docBody(p, "requirements"); after != before || before == "" {
		t.Fatalf("requirements depend on the survey:\n%s\n---\n%s", before, after)
	}
	if docBody(p, core.ArtifactSurvey) != "" {
		t.Fatal("survey doc without survey")
	}
}
