package work

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func TestDetailPromptListsCoveredCriteria(t *testing.T) {
	p := &core.Project{
		Requirements: []core.Requirement{
			{ID: "REQ-001", Acceptance: []core.Criterion{{ID: "REQ-001-AC1", Kind: "negative", Text: "kriteria-tercakup"}}},
			{ID: "REQ-002", Acceptance: []core.Criterion{{ID: "REQ-002-AC1", Text: "kriteria-lain"}}},
		},
		Workflow: &core.Workflow{Commands: []core.Command{
			{ID: "C1", Instructions: "selesai", Covers: []string{"REQ-002"}},
			{ID: "C2", Covers: []string{"REQ-001"}},
		}},
	}
	prompt := WorkflowPrompt(p, nil)
	if !strings.Contains(prompt, "- REQ-001-AC1 (negative): kriteria-tercakup") {
		t.Errorf("kriteria batch tidak ada:\n%s", prompt)
	}
	if strings.Contains(prompt, "kriteria-lain") {
		t.Error("kriteria di luar batch ikut masuk")
	}

	// The criteria stay listed when the command reads the acceptance
	// artifact; its sections per requirement are not repeated, the rest
	// (Definition of Done) is.
	p.Workflow.Commands[1].Artifacts = []core.ArtifactRef{{ID: core.ArtifactAcceptance}}
	docs := []core.Artifact{{ID: core.ArtifactAcceptance, Body: "isi-acceptance\n## REQ-001 (MUST)\nsalinan-kriteria\n## REQ-002 (MUST)\nkriteria-lain\n## Definition of Done\nisi-dod\n"}}
	prompt = WorkflowPrompt(p, docs)
	if !strings.Contains(prompt, "isi-acceptance") || !strings.Contains(prompt, "isi-dod") ||
		strings.Count(prompt, "kriteria-tercakup") != 1 || strings.Contains(prompt, "salinan-kriteria") || strings.Contains(prompt, "kriteria-lain") {
		t.Errorf("kriteria digandakan atau hilang:\n%s", prompt)
	}
}

func staleProject(t *testing.T) (*core.Project, []core.Artifact) {
	t.Helper()
	docs := []core.Artifact{{ID: "requirements", Title: "R", Body: "v1"}}
	p := &core.Project{}
	raw := `{"commands": [
		{"id": "C1", "name": "satu", "objective": "o", "instructions": "i", "artifacts": ["requirements"]},
		{"id": "C2", "name": "dua", "objective": "o", "instructions": "i", "depends_on": ["C1"], "artifacts": ["requirements"]}]}`
	applyFullWorkflow(t, p, raw, docs)
	docs[0] = core.Artifact{ID: "requirements", Title: "R", Body: "v2"}
	return p, docs
}

// applyFullWorkflow lands a new workflow the way Foundry writes it: the
// outline first, then one detail round per command, in sequence.
func applyFullWorkflow(t *testing.T, p *core.Project, raw string, docs []core.Artifact) {
	t.Helper()
	if err := ApplyWorkflow(p, raw, docs); err != nil {
		t.Fatal(err)
	}
	var out workflowOut
	if err := ExtractJSON(raw, &out); err != nil {
		t.Fatal(err)
	}
	for p.Workflow.Unfilled() > 0 {
		next := nextFillCommand(p.Workflow.Commands)
		i := slices.IndexFunc(p.Workflow.Commands, func(c core.Command) bool { return c.ID == next.ID })
		d := out.Commands[i]
		d.ID = next.ID
		b, _ := json.Marshal(workflowOut{Commands: []commandOut{d}})
		if err := ApplyWorkflow(p, string(b), docs); err != nil {
			t.Fatal(err)
		}
	}
}

// A truncated answer must not decode to an empty workflow: the inner command
// object has no "commands" key, so it is not a workflow.
func TestExtractWorkflowRejectsInnerObject(t *testing.T) {
	var out workflowOut
	err := ExtractJSONFunc(`{"commands": [{"id": "C1", "name": "a"}, {"id": "C2"`,
		&out, func() bool { return out.Commands != nil || out.Remove != nil })
	if err == nil {
		t.Fatalf("truncated answer decoded as %+v", out)
	}
}

// OpenCode cuts long answers at the model's output limit. The command that
// arrived whole is applied, the request stays for the next round, and the
// next answer finishes it.
func TestApplyWorkflowTruncatedMakesProgress(t *testing.T) {
	p, docs := staleProject(t)
	p.Workflow.Request = &core.ChangeRequest{
		Text:  "Perbarui command yang terdampak.",
		Focus: []string{"CMD-001", "CMD-002"},
	}
	truncated := "```json\n{\"commands\": [\n" +
		`{"id": "CMD-001", "name": "satu baru", "objective": "o", "instructions": "i", "artifacts": ["requirements"]},` + "\n" +
		`{"id": "CMD-002", "name": "dua`
	if err := ApplyWorkflow(p, truncated, docs); err != nil {
		t.Fatalf("truncated answer: %v", err)
	}
	if p.Workflow.Commands[0].Name != "satu baru" {
		t.Fatalf("salvaged command = %+v", p.Workflow.Commands[0])
	}
	if p.Workflow.Request == nil {
		t.Fatal("request cleared while a stale command was still untouched")
	}

	full := `{"commands": [{"id": "CMD-002", "name": "dua baru", "objective": "o",
		"instructions": "i", "depends_on": ["CMD-001"], "artifacts": ["requirements"]}], "done": true}`
	if err := ApplyWorkflow(p, full, docs); err != nil {
		t.Fatal(err)
	}
	if p.Workflow.Request != nil {
		t.Fatalf("request survived a complete answer: %+v", p.Workflow.Request)
	}
	if p.Workflow.Commands[1].Name != "dua baru" {
		t.Fatalf("command = %+v", p.Workflow.Commands[1])
	}
}

// An empty answer to a sync request means the text still matches the changed
// artifacts: their new versions are recorded instead of failing.
func TestApplyWorkflowSyncEmptyAcceptsStale(t *testing.T) {
	p, docs := staleProject(t)
	p.Workflow.Request = &core.ChangeRequest{
		Text:  "Perbarui command yang terdampak.",
		Focus: []string{"CMD-001"},
		Sync:  true,
	}
	before := p.Workflow.Version
	if err := ApplyWorkflow(p, `{"commands": []}`, docs); err != nil {
		t.Fatal(err)
	}
	if p.Workflow.Request != nil || p.Workflow.Version != before+1 {
		t.Fatalf("workflow = %+v", p.Workflow)
	}
	if h := docs[0].Hash(); !slices.ContainsFunc(p.Workflow.Commands[0].Artifacts, func(r core.ArtifactRef) bool {
		return r.ID == "requirements" && r.Hash == h
	}) {
		t.Fatalf("artifact hash not refreshed: %+v", p.Workflow.Commands[0].Artifacts)
	}
	// A command outside the focus keeps its old hash.
	if slices.ContainsFunc(p.Workflow.Commands[1].Artifacts, func(r core.ArtifactRef) bool {
		return r.Hash == docs[0].Hash()
	}) {
		t.Fatal("unfocused command was refreshed")
	}
}

// A real request that the AI leaves unanswered is still refused.
func TestApplyWorkflowEmptyChangeStillFails(t *testing.T) {
	p, docs := staleProject(t)
	p.Workflow.Request = &core.ChangeRequest{Text: "pecah CMD-001", Focus: []string{"CMD-001"}}
	if err := ApplyWorkflow(p, `{"commands": []}`, docs); err == nil || !strings.Contains(err.Error(), "tidak menemukan perubahan") {
		t.Fatalf("err = %v", err)
	}
}

func TestWorkflowPromptListsFeatures(t *testing.T) {
	p := &core.Project{Name: "Klinik", Story: "Pasien membuat janji",
		Features: []core.Feature{{ID: "FEAT-001", Name: "Janji",
			SubFeatures: []core.SubFeature{{ID: "FEAT-001.1", Name: "Form janji"}}}}}
	prompt := WorkflowPrompt(p, nil)
	for _, want := range []string{"FEAT-001 Janji", "FEAT-001.1 Form janji", `"feature_ids"`} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt lacks %q", want)
		}
	}
}

func TestDetailPromptCarriesSkeletonOnly(t *testing.T) {
	p := &core.Project{Workflow: &core.Workflow{Commands: []core.Command{
		{ID: "C1", Name: "selesai", Instructions: "teks-selesai", Outputs: []string{"keluaran-c1"}},
		{ID: "C2", Name: "lepas", Instructions: "teks-lepas", Outputs: []string{"keluaran-c2"}},
		{ID: "C3", Name: "giliran", DependsOn: []string{"C1"}},
	}}}
	prompt := WorkflowPrompt(p, nil)
	for _, want := range []string{"selesai", "lepas", "giliran", "keluaran-c1"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt tanpa %q", want)
		}
	}
	for _, gone := range []string{"teks-selesai", "teks-lepas", "keluaran-c2"} {
		if strings.Contains(prompt, gone) {
			t.Errorf("prompt masih memuat %q", gone)
		}
	}
	if p.Workflow.Commands[0].Instructions != "teks-selesai" {
		t.Fatal("prompt mengubah command tersimpan")
	}
}

func TestRemovedCommandIDIsNotReused(t *testing.T) {
	p := &core.Project{Workflow: &core.Workflow{Version: 1, Commands: []core.Command{
		{ID: "CMD-001", Name: "a", Objective: "o", Instructions: "i", Artifacts: []core.ArtifactRef{{ID: "requirements", Hash: "h"}}},
		{ID: "CMD-002", Name: "b", Objective: "o", Instructions: "i", Artifacts: []core.ArtifactRef{{ID: "requirements", Hash: "h"}}},
	}, Request: &core.ChangeRequest{Text: "ganti"}}}
	p.Progress = &core.Progress{Retired: map[string]core.CommandRun{"CMD-007": {CommandID: "CMD-007", Status: core.RunVerified}}}
	docs := []core.Artifact{{ID: "requirements"}}
	if err := ApplyWorkflow(p, `{"remove":["CMD-002"],"commands":[]}`, docs); err != nil {
		t.Fatal(err)
	}
	raw := `{"commands":[{"id":"NEW1","name":"c","objective":"o","instructions":"i","artifacts":["requirements"],"priority":"Tinggi","complexity":"bogus"}],"done":true}`
	if err := ApplyWorkflow(p, raw, docs); err != nil {
		t.Fatal(err)
	}
	c := p.Workflow.Commands[1]
	if c.ID != "CMD-008" || c.Priority != core.LevelHigh || c.Complexity != "" {
		t.Fatalf("command = %+v; a new command must not inherit a removed one's run", c)
	}
}

// Detail rounds are sequential: one command each, never before the
// commands it depends on, whatever order the outline lists them in.
func TestNextFillCommandFollowsDependencies(t *testing.T) {
	cmds := []core.Command{
		{ID: "CMD-001", DependsOn: []string{"CMD-002"}},
		{ID: "CMD-002"},
		{ID: "CMD-003", DependsOn: []string{"CMD-001"}},
	}
	var order []string
	for {
		c := nextFillCommand(cmds)
		if c == nil {
			break
		}
		order = append(order, c.ID)
		c.Instructions = "isi"
	}
	if !slices.Equal(order, []string{"CMD-002", "CMD-001", "CMD-003"}) {
		t.Fatalf("order = %v", order)
	}
}

// A detail answer lands only the command the round asked for.
func TestDetailRoundFillsOnlyRequestedCommand(t *testing.T) {
	docs := []core.Artifact{{ID: "requirements", Title: "R", Body: "v1"}}
	p := &core.Project{}
	if err := ApplyWorkflow(p, `{"commands": [
		{"id": "C1", "name": "a", "objective": "o", "instructions": "i", "artifacts": ["requirements"]},
		{"id": "C2", "name": "b", "objective": "o", "instructions": "i", "depends_on": ["C1"], "artifacts": ["requirements"]}]}`, docs); err != nil {
		t.Fatal(err)
	}
	if n := p.Workflow.Unfilled(); n != 2 {
		t.Fatalf("outline kept instructions: unfilled = %d", n)
	}
	both := `{"commands": [{"id": "CMD-001", "instructions": "satu"}, {"id": "CMD-002", "instructions": "dua"}]}`
	if err := ApplyWorkflow(p, both, docs); err != nil {
		t.Fatal(err)
	}
	if c := p.Workflow.Commands; c[0].Instructions != "satu" || c[1].Instructions != "" || p.Workflow.Request == nil {
		t.Fatalf("commands = %+v", c)
	}
	if err := ApplyWorkflow(p, `{"commands": [{"id": "CMD-001", "instructions": "x"}]}`, docs); err == nil {
		t.Fatal("answer for a command out of turn accepted")
	}
}

// A revision makes one command change per answer and keeps the request open,
// with what it did, until the answer says done.
func TestRevisionIsOneCommandPerRound(t *testing.T) {
	docs := []core.Artifact{{ID: "requirements"}}
	p := &core.Project{Workflow: &core.Workflow{Version: 1, Commands: []core.Command{
		{ID: "CMD-001", Name: "a", Objective: "o", Instructions: "i", Artifacts: []core.ArtifactRef{{ID: "requirements"}}},
		{ID: "CMD-002", Name: "b", Objective: "o", Instructions: "i", DependsOn: []string{"CMD-001"}, Artifacts: []core.ArtifactRef{{ID: "requirements"}}},
		{ID: "CMD-003", Name: "c", Objective: "o", Instructions: "i", DependsOn: []string{"CMD-002"}, Artifacts: []core.ArtifactRef{{ID: "requirements"}}},
	}, Request: &core.ChangeRequest{Text: "ganti"}}}
	two := `{"commands": [{"id": "CMD-001", "name": "x", "objective": "o", "instructions": "i", "artifacts": ["requirements"]},
		{"id": "CMD-002", "name": "y", "objective": "o", "instructions": "i", "artifacts": ["requirements"]}]}`
	if err := ApplyWorkflow(p, two, docs); err == nil || !strings.Contains(err.Error(), "satu command per jawaban") {
		t.Fatalf("two changes in one answer err = %v", err)
	}
	if err := ApplyWorkflow(p, `{"commands": [], "remove": ["CMD-002"]}`, docs); err != nil {
		t.Fatal(err)
	}
	if c := p.Workflow.Command("CMD-003"); c == nil || !slices.Equal(c.DependsOn, []string{"CMD-001"}) {
		t.Fatalf("dependent not rewired: %+v", c)
	}
	if r := p.Workflow.Request; r == nil || !slices.Equal(r.Handled, []string{"removed CMD-002"}) {
		t.Fatalf("request after round 1 = %+v", r)
	}
	if !strings.Contains(WorkflowPrompt(p, docs), "EARLIER ROUNDS of this request (already saved in CURRENT COMMANDS): removed CMD-002.") {
		t.Fatal("next round prompt misses the earlier rounds")
	}
	if err := ApplyWorkflow(p, `{"commands": [{"id": "CMD-003", "name": "z", "objective": "o", "instructions": "i", "depends_on": ["CMD-001"], "artifacts": ["requirements"]}], "done": true}`, docs); err != nil {
		t.Fatal(err)
	}
	if p.Workflow.Request != nil || p.Workflow.Command("CMD-003").Name != "z" {
		t.Fatalf("done answer left request %+v", p.Workflow.Request)
	}
}

// A sync revision that stops without the AI saying it is done (a repeated
// change, or the round limit) has not reviewed the remaining stale commands:
// they must stay marked for review, not be accepted as still valid.
func TestSyncRevisionStoppedWithoutDoneKeepsStale(t *testing.T) {
	p, docs := staleProject(t)
	p.Workflow.Request = &core.ChangeRequest{Text: "Ikuti keputusan baru.", Sync: true}
	one := `{"commands": [{"id": "CMD-001", "name": "satu baru", "objective": "o", "instructions": "i", "artifacts": ["requirements"]}], "done": false}`
	if err := ApplyWorkflow(p, one, docs); err != nil {
		t.Fatal(err)
	}
	if p.Workflow.Request == nil {
		t.Fatal("request closed while CMD-002 is still stale")
	}
	if err := ApplyWorkflow(p, one, docs); err != nil { // the same change again: no progress
		t.Fatal(err)
	}
	if p.Workflow.Request != nil {
		t.Fatalf("revision without progress not ended: %+v", p.Workflow.Request)
	}
	h := docs[0].Hash()
	if slices.ContainsFunc(p.Workflow.Command("CMD-002").Artifacts, func(r core.ArtifactRef) bool { return r.Hash == h }) {
		t.Fatal("unreviewed stale command was accepted as up to date")
	}
}

// The AI's done on a sync revision says the rest still matches: the
// remaining stale commands get the new artifact versions.
func TestSyncRevisionDoneAcceptsRest(t *testing.T) {
	p, docs := staleProject(t)
	p.Workflow.Request = &core.ChangeRequest{Text: "Ikuti keputusan baru.", Sync: true}
	one := `{"commands": [{"id": "CMD-001", "name": "satu baru", "objective": "o", "instructions": "i", "artifacts": ["requirements"]}], "done": true}`
	if err := ApplyWorkflow(p, one, docs); err != nil {
		t.Fatal(err)
	}
	h := docs[0].Hash()
	if p.Workflow.Request != nil || !slices.ContainsFunc(p.Workflow.Command("CMD-002").Artifacts, func(r core.ArtifactRef) bool { return r.Hash == h }) {
		t.Fatalf("done sync revision did not accept the rest: %+v", p.Workflow)
	}
}

// A focused revision reads the focus and its dependents in full, the rest as
// an outline, and only the artifacts the full commands reference by body.
func TestFocusedRevisionReadsOnlyAffectedCommands(t *testing.T) {
	cmd := func(id, text string, deps []string, art string) core.Command {
		return core.Command{ID: id, Name: "nama-" + id, Instructions: text, Outputs: []string{"out-" + id},
			DependsOn: deps, Artifacts: []core.ArtifactRef{{ID: art}}}
	}
	p := &core.Project{Workflow: &core.Workflow{Commands: []core.Command{
		cmd("C1", "teks-c1", nil, "data"),
		cmd("C2", "teks-c2", []string{"C1"}, "api"),
		cmd("C3", "teks-c3", []string{"C2"}, "ui"),
		cmd("C4", "teks-c4", nil, "ops"),
	}, Request: &core.ChangeRequest{Text: "ubah", Focus: []string{"C2"}}}}
	docs := []core.Artifact{
		{ID: "data", Body: "isi-data"}, {ID: "api", Body: "isi-api"}, {ID: "ui", Body: "isi-ui"},
		{ID: "ops", Body: "isi-ops"}, {ID: "requirements", Body: "isi-req"},
	}
	prompt := WorkflowPrompt(p, docs)
	for _, want := range []string{"teks-c2", "teks-c3", "nama-C1", "out-C1", "nama-C4", "isi-api", "isi-ui", "isi-req"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt tanpa %q", want)
		}
	}
	for _, gone := range []string{"teks-c1", "teks-c4", "out-C4", "isi-data", "isi-ops"} {
		if strings.Contains(prompt, gone) {
			t.Errorf("prompt masih memuat %q", gone)
		}
	}

	p.Workflow.Request.Focus = nil // unfocused: every command in full
	prompt = WorkflowPrompt(p, docs)
	for _, want := range []string{"teks-c1", "teks-c4", "isi-data", "isi-ops"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("unfocused prompt tanpa %q", want)
		}
	}
}

func TestRevisionScope(t *testing.T) {
	cmd := func(id string, deps ...string) core.Command { return core.Command{ID: id, DependsOn: deps} }
	cases := []struct {
		name    string
		cmds    []core.Command
		focus   []string
		handled []string
		want    []string // nil: every command in full
	}{
		{"direct dependent", []core.Command{cmd("A"), cmd("B", "A"), cmd("X")}, []string{"A"}, nil, []string{"A", "B"}},
		{"transitive, multi-level", []core.Command{cmd("A"), cmd("B", "A"), cmd("C", "B"), cmd("D", "C"), cmd("X")}, []string{"A"}, nil, []string{"A", "B", "C", "D"}},
		{"order does not matter", []core.Command{cmd("D", "C"), cmd("C", "B"), cmd("B", "A"), cmd("A"), cmd("X")}, []string{"A"}, nil, []string{"A", "B", "C", "D"}},
		{"branching", []core.Command{cmd("A"), cmd("B", "A"), cmd("C", "A"), cmd("D", "B", "C"), cmd("X"), cmd("Y", "X")}, []string{"A"}, nil, []string{"A", "B", "C", "D"}},
		{"cycle", []core.Command{cmd("A", "C"), cmd("B", "A"), cmd("C", "B"), cmd("X")}, []string{"B"}, nil, []string{"A", "B", "C"}},
		{"upstream stays outline", []core.Command{cmd("A"), cmd("B", "A"), cmd("C", "B")}, []string{"C"}, nil, []string{"C"}},
		{"no dependencies", []core.Command{cmd("A"), cmd("B"), cmd("C")}, []string{"B"}, nil, []string{"B"}},
		{"several focus, several dependents", []core.Command{cmd("A"), cmd("B"), cmd("C", "A"), cmd("D", "A"), cmd("E", "B"), cmd("X")}, []string{"A", "B"}, nil, []string{"A", "B", "C", "D", "E"}},
		{"earlier rounds join", []core.Command{cmd("A"), cmd("B", "A"), cmd("N"), cmd("M", "N"), cmd("X")}, []string{"A"}, []string{"added N", "removed Z"}, []string{"A", "B", "M", "N"}},
		{"focus removed by an earlier round", []core.Command{cmd("B"), cmd("X")}, []string{"A"}, []string{"removed A"}, nil},
		{"scope is everything", []core.Command{cmd("A"), cmd("B", "A")}, []string{"A"}, nil, nil},
		{"unfocused", []core.Command{cmd("A"), cmd("B")}, nil, nil, nil},
	}
	for _, tc := range cases {
		w := &core.Workflow{Commands: tc.cmds, Request: &core.ChangeRequest{Text: "x", Focus: tc.focus, Handled: tc.handled}}
		got := revisionScope(w)
		if tc.want == nil {
			if got != nil {
				t.Errorf("%s: scope = %v, want every command", tc.name, got)
			}
			continue
		}
		ids := slices.Sorted(maps.Keys(got))
		if !slices.Equal(ids, tc.want) {
			t.Errorf("%s: scope = %v, want %v", tc.name, ids, tc.want)
		}
	}
}

// A command referring to an artifact that was deleted or renamed still
// gets its full text; the artifact is simply not there to send.
func TestFocusedRevisionWithMissingArtifact(t *testing.T) {
	p := &core.Project{Workflow: &core.Workflow{Commands: []core.Command{
		{ID: "C1", Instructions: "teks-c1", Artifacts: []core.ArtifactRef{{ID: "gone", Hash: "h"}}},
		{ID: "C2", Instructions: "teks-c2", Artifacts: []core.ArtifactRef{{ID: "api"}}},
	}, Request: &core.ChangeRequest{Text: "ubah", Focus: []string{"C1"}}}}
	docs := []core.Artifact{{ID: "api", Body: "isi-api"}, {ID: "requirements", Body: "isi-req"}}
	prompt := WorkflowPrompt(p, docs)
	if !strings.Contains(prompt, "teks-c1") || strings.Contains(prompt, "teks-c2") || strings.Contains(prompt, "isi-api") || !strings.Contains(prompt, "isi-req") {
		t.Fatalf("prompt:\n%s", prompt)
	}
}

// A focused revision never takes back a command it only saw as an outline.
func TestFocusedRevisionRefusesOutlineCommand(t *testing.T) {
	p := &core.Project{Workflow: &core.Workflow{Commands: []core.Command{
		{ID: "C1", Name: "a", Instructions: "teks-c1"},
		{ID: "C2", Name: "b", Instructions: "teks-c2", DependsOn: []string{"C1"}},
		{ID: "C3", Name: "c", Instructions: "teks-c3"},
	}, Request: &core.ChangeRequest{Text: "ubah", Focus: []string{"C1"}}}}
	err := applyWorkflowRevisionFromParsed(p, workflowOut{Commands: []commandOut{{ID: "C3"}}}, nil)
	if err == nil || !strings.Contains(err.Error(), "di luar cakupan") {
		t.Fatalf("err = %v", err)
	}
	if p.Workflow.Commands[2].Instructions != "teks-c3" {
		t.Fatal("outline command changed")
	}
}

// A command answered with the {"id","hash"} artifact objects the prompt
// shows is read as ids, not refused (that refusal cost a whole extra round).
func TestCommandArtifactsAsObjects(t *testing.T) {
	var out workflowOut
	if err := json.Unmarshal([]byte(`{"commands":[{"id":"C1","artifacts":[{"id":"api","hash":"x"},"ui"]}]}`), &out); err != nil {
		t.Fatal(err)
	}
	if got := []string(out.Commands[0].Artifacts); !slices.Equal(got, []string{"api", "ui"}) {
		t.Fatalf("artifacts = %v", got)
	}
	for _, bad := range []string{`{"commands":[{"id":"C1","artifacts":[{"hash":"x"}]}]}`, `{"commands":[{"id":"C1","artifacts":[1]}]}`, `{"commands":[{"id":"C1","artifacts":"api"}]}`} {
		if err := json.Unmarshal([]byte(bad), &out); err == nil {
			t.Errorf("%s accepted", bad)
		}
	}
}
