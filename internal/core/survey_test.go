package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSurveyMovesFirstSurveyAnswers(t *testing.T) {
	s := &Survey{Kinds: []string{" Web", "bot", "api"}, Users: "team", Stage: "MVP", Latency: "500", Webhook: "none",
		Services: "many", Stack: "Laravel, MySQL", Priorities: []string{"security", "security"}}
	s.FillDefaults()
	want := &Survey{Kinds: []string{"web", "api"}, Users: SurveyUsersInternal, Stage: SurveyStageMVP,
		Architecture: SurveyArchServices, Constraints: "Laravel, MySQL", Priorities: []string{"security"}}
	if !reflect.DeepEqual(s, want) {
		t.Fatalf("got %+v\nwant %+v", s, want)
	}
	// Moving again changes nothing; a chosen architecture is kept.
	s.FillDefaults()
	if !reflect.DeepEqual(s, want) {
		t.Fatalf("second fill %+v", s)
	}
	o := &Survey{Users: "self", Services: "many", Architecture: SurveyArchModular, Latency: "100"}
	o.FillDefaults()
	if o.Users != "" || o.Architecture != SurveyArchModular || o.Latency != "100" || o.Services != "" {
		t.Fatalf("got %+v", o)
	}
}

func TestSurveyLeavesOpenAnswersOpen(t *testing.T) {
	s := &Survey{}
	s.FillDefaults()
	if s.Stage != "" || s.Users != "" || s.Architecture != "" || len(s.Kinds) > 0 {
		t.Fatalf("defaults invented answers: %+v", s)
	}
	if s.Delivery() != SurveyStageMVP || (*Survey)(nil).Delivery() != SurveyStageMVP {
		t.Fatal("delivery default is MVP")
	}
}

func TestSurveyCheck(t *testing.T) {
	ok := &Survey{Kinds: []string{"web", "embedded", "other"}, Users: SurveyUsersMixed, Stage: SurveyStageProd,
		Architecture: SurveyArchExisting, Priorities: []string{SurveyPriorityTesting}}
	if err := ok.Check(); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []*Survey{{Kinds: []string{"tv"}}, {Users: "self"}, {Stage: "beta"}, {Architecture: "serverless"},
		{Priorities: []string{"murah"}}} {
		if bad.Check() == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
}

func TestLoadMovesFirstSurvey(t *testing.T) {
	st, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Init("test"); err != nil {
		t.Fatal(err)
	}
	raw := `{"version":1,"name":"x","stage":"TELL","story":"cerita","requirements":[],"features":[],"flows":[],"events":[],
"survey":{"what":"cerita","users":"team","stage":"prototype","latency":"500","webhook":"in","services":"many","stack":"Go"}}`
	if err := os.WriteFile(filepath.Join(st.Dir, StateDir, "project.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := &Survey{What: "cerita", Users: SurveyUsersInternal, Stage: SurveyStagePrototype, Architecture: SurveyArchServices,
		Constraints: "Go", Webhook: "in"}
	if !reflect.DeepEqual(p.Survey, want) {
		t.Fatalf("got %+v", p.Survey)
	}
}
