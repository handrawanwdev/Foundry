package workflow

import (
	"slices"
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

func cmd(id string, deps ...string) core.Command {
	return core.Command{ID: id, Name: id, Objective: "o", Instructions: "i", DependsOn: deps,
		Artifacts: []core.ArtifactRef{{ID: "requirements", Hash: "h1"}}}
}

func TestValidate(t *testing.T) {
	docs := map[string]bool{"requirements": true}
	ok := []core.Command{cmd("A"), cmd("B", "A"), cmd("C", "A"), cmd("D", "B", "C")}
	if issues := Validate(ok, docs); len(issues) != 0 {
		t.Fatalf("valid workflow: %+v", issues)
	}
	bad := []core.Command{cmd("A", "A"), cmd("B", "Z"), cmd("B"), {ID: "E", Artifacts: []core.ArtifactRef{{ID: "ghost"}}}}
	var msgs []string
	for _, is := range Validate(bad, docs) {
		msgs = append(msgs, is.Command+": "+is.Message)
	}
	got := strings.Join(msgs, "\n")
	for _, want := range []string{"A: bergantung pada dirinya sendiri", "B: dependency Z tidak ada", "ID B dipakai lebih dari satu",
		"E: nama belum diisi", "E: instruksi belum diisi", `E: artefak blueprint "ghost" tidak ada`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
}

func TestCycleAndWaves(t *testing.T) {
	cyc := []core.Command{cmd("A", "C"), cmd("B", "A"), cmd("C", "B"), cmd("D")}
	if c := Cycle(cyc); !slices.Equal(c, []string{"A", "C", "B", "A"}) {
		t.Fatalf("cycle = %v", c)
	}
	if Waves(cyc) != nil {
		t.Fatal("waves of a cyclic workflow")
	}
	if is := Validate(cyc, map[string]bool{"requirements": true}); len(is) != 1 || !strings.Contains(is[0].Message, "melingkar") {
		t.Fatalf("issues = %+v", is)
	}
	// Independent commands share a wave; order follows the list.
	w := Waves([]core.Command{cmd("A"), cmd("B", "A"), cmd("C", "A"), cmd("D", "B", "C"), cmd("E")})
	want := [][]string{{"A", "E"}, {"B", "C"}, {"D"}}
	if len(w) != len(want) {
		t.Fatalf("waves = %v", w)
	}
	for i := range want {
		if !slices.Equal(w[i], want[i]) {
			t.Fatalf("waves = %v", w)
		}
	}
}

func TestStaleAndCheck(t *testing.T) {
	wf := &core.Workflow{Commands: []core.Command{cmd("A"), cmd("B", "A")}}
	wf.Commands[1].Artifacts = append(wf.Commands[1].Artifacts, core.ArtifactRef{ID: "api", Hash: "x"})
	docs := []core.Artifact{{ID: "requirements", Title: "R", Body: "b"}, {ID: "api", Title: "API", Body: "v2"}}
	for i := range wf.Commands {
		wf.Commands[i].Artifacts[0].Hash = docs[0].Hash()
	}
	r := Check(wf, docs, nil)
	if r.Ready || !slices.Equal(r.Stale["B"], []string{"api"}) || len(r.Stale["A"]) != 0 || !strings.Contains(r.Review["B"], "api") {
		t.Fatalf("report = %+v", r)
	}
	wf.Commands[1].Artifacts[1].Hash = docs[1].Hash()
	if r = Check(wf, docs, nil); !r.Ready || len(r.Waves) != 2 {
		t.Fatalf("report = %+v", r)
	}
	// A removed artifact is stale and a reference error.
	if r = Check(wf, docs[:1], nil); r.Ready || len(r.Issues) != 1 || !slices.Equal(r.Stale["B"], []string{"api"}) {
		t.Fatalf("removed artifact = %+v", r)
	}
	wf.Commands[0].Status = core.CommandNeedsReview
	if r = Check(wf, docs, nil); r.Ready || r.Review["A"] != "" {
		t.Fatalf("manual review = %+v", r)
	}
	if !slices.Equal(Dependents(wf.Commands, "A"), []string{"B"}) {
		t.Fatal("dependents")
	}
	if r = Check(nil, docs, nil); r.Ready {
		t.Fatal("empty workflow ready")
	}
}

// Every command refers to the blueprint, and the UI design is implemented
// by some command when the blueprint has one.
func TestValidateArtifactRefs(t *testing.T) {
	cmds := []core.Command{
		{ID: "CMD-001", Name: "A", Objective: "a", Instructions: "a", Artifacts: []core.ArtifactRef{{ID: "requirements"}}},
		{ID: "CMD-002", Name: "B", Objective: "b", Instructions: "b"},
	}
	msgs := func(issues []Issue) string {
		var out []string
		for _, is := range issues {
			out = append(out, is.Command+" "+is.Message)
		}
		return strings.Join(out, "; ")
	}
	got := msgs(Validate(cmds, map[string]bool{"requirements": true}))
	if got != "CMD-002 belum merujuk artefak blueprint" {
		t.Fatalf("issues = %s", got)
	}
	cmds[1].Artifacts = []core.ArtifactRef{{ID: core.ArtifactDesignRules}}
	got = msgs(Validate(cmds, map[string]bool{"requirements": true, core.ArtifactDesignRules: true, core.ArtifactLayoutSpec: true}))
	if got != " tidak ada command yang merujuk artefak layout-spec" {
		t.Fatalf("issues = %s", got)
	}
}

func TestCovers(t *testing.T) {
	reqs := []core.Requirement{{ID: "REQ-001", Priority: "MUST"}, {ID: "REQ-002", Priority: "MUST"}, {ID: "REQ-003", Priority: "COULD"}}
	cmds := []core.Command{{ID: "A", Covers: []string{"REQ-001", "REQ-404"}}}
	issues, uncovered := Covers(cmds, reqs)
	if len(issues) != 1 || issues[0].Command != "A" || !slices.Equal(uncovered, []string{"REQ-002"}) {
		t.Fatalf("issues %+v uncovered %v", issues, uncovered)
	}
	if _, uncovered := Covers(nil, reqs); uncovered != nil {
		t.Fatalf("no commands yet: uncovered = %v", uncovered)
	}
	// Uncovered requirements are reported but do not block; unknown ids do.
	r := Check(&core.Workflow{Commands: []core.Command{{ID: "A", Name: "a", Objective: "o", Instructions: "i",
		Artifacts: []core.ArtifactRef{{ID: "requirements"}}, Covers: []string{"REQ-001"}}}}, []core.Artifact{{ID: "requirements"}}, reqs)
	if len(r.Issues) != 0 || !slices.Equal(r.Uncovered, []string{"REQ-002"}) {
		t.Fatalf("report = %+v", r)
	}
}
