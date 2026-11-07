package core

import "testing"

func TestParseAndClampIntent(t *testing.T) {
	if ParseWorkIntent(" New_Feature ") != IntentNewFeature {
		t.Fatal("parse")
	}
	if ParseWorkIntent("nope") != "" {
		t.Fatal("unknown")
	}
	if !IntentNewFeature.ChangeScope() || IntentExisting.ChangeScope() || IntentNewProject.ChangeScope() {
		t.Fatal("change scope")
	}
	if got := ClampIntent(true, ""); got != IntentExisting {
		t.Fatalf("existing default = %s", got)
	}
	if got := ClampIntent(false, ""); got != IntentNewProject {
		t.Fatalf("new default = %s", got)
	}
	if got := ClampIntent(true, IntentNewProject); got != IntentExisting {
		t.Fatalf("cannot treat existing as new = %s", got)
	}
	if got := ClampIntent(false, IntentNewFeature); got != IntentNewProject {
		t.Fatalf("cannot treat empty folder as feature = %s", got)
	}
	if got := ClampIntent(true, IntentBugFix); got != IntentBugFix {
		t.Fatalf("keep bug fix = %s", got)
	}
}

func TestIntentOrDefault(t *testing.T) {
	p := &Project{Structure: &Structure{Mode: StructureExisting}}
	if p.IntentOrDefault() != IntentExisting {
		t.Fatal(p.IntentOrDefault())
	}
	p.Intent = IntentNewFeature
	if p.IntentOrDefault() != IntentNewFeature {
		t.Fatal(p.IntentOrDefault())
	}
}

func TestNormalizeChangePlan(t *testing.T) {
	got := NormalizeChangePlan(&ChangePlan{
		Scope: "CHANGE", Summary: "  notifikasi  ",
		Existing: []string{" ", "internal/notify"},
		Items: []ChangeItem{
			{Action: "ADD", Target: "Notification", Module: "notify", Files: []string{"", "internal/notify/service.go"}},
			{Action: "delete", Target: "x"},
			{Action: "reuse", Target: ""},
		},
		Impact: []ChangeImpact{
			{Area: "API", Message: "endpoint baru", Risk: "MEDIUM"},
			{Area: "", Message: "skip"},
		},
		Files: ChangeFiles{
			Add:    []ChangeFile{{Path: " internal/notify/service.go ", Responsibility: "kirim"}},
			Remove: []ChangeFile{{Path: ""}},
		},
	})
	if got == nil || got.Scope != BlueprintScopeChange || got.Summary != "notifikasi" {
		t.Fatalf("got %+v", got)
	}
	if len(got.Existing) != 1 || got.Existing[0] != "internal/notify" {
		t.Fatalf("existing %+v", got.Existing)
	}
	if len(got.Items) != 1 || got.Items[0].Action != ChangeAdd || len(got.Items[0].Files) != 1 {
		t.Fatalf("items %+v", got.Items)
	}
	if len(got.Impact) != 1 || got.Impact[0].Risk != "medium" {
		t.Fatalf("impact %+v", got.Impact)
	}
	if len(got.Files.Add) != 1 || len(got.Files.Remove) != 0 {
		t.Fatalf("files %+v", got.Files)
	}
	if NormalizeChangePlan(&ChangePlan{}) != nil {
		t.Fatal("empty")
	}
}
