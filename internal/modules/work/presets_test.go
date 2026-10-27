package work

import (
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func TestPresetByID(t *testing.T) {
	if PresetByID("does-not-exist") != nil {
		t.Error("unknown preset id should return nil")
	}
	for _, id := range []string{presetCleanSaaS, presetEnterprise, presetMinimalData, presetDeveloperTool} {
		if p := PresetByID(id); p == nil || p.ID != id {
			t.Errorf("PresetByID(%q) = %v, want preset with that id", id, p)
		}
	}
}

func TestDetectPreset(t *testing.T) {
	tests := []struct {
		name string
		p    *core.Project
		want string // "" means nil preset
	}{
		{"nil returns minimal-data", nil, presetMinimalData},
		{"saas story", &core.Project{Story: "Platform SaaS multi-tenant dengan subscription"}, presetCleanSaaS},
		{"dashboard story", &core.Project{Story: "Dashboard monitoring untuk sistem trading"}, presetCleanSaaS},
		{"analytics story", &core.Project{Story: "Tool analytics untuk monitoring performance"}, presetCleanSaaS},
		{"admin story", &core.Project{Story: "Sistem administrasi untuk manajemen user dan role"}, presetEnterprise},
		{"developer tool story", &core.Project{Story: "API console untuk developer, seperti playground"}, presetDeveloperTool},
		{"generic data app", &core.Project{Story: "Aplikasi sederhana untuk mengelola catatan"}, presetMinimalData},
		{"mobile app has no preset", &core.Project{Story: "Mobile app untuk Android dan iOS"}, ""},
		{"ecommerce has no preset", &core.Project{Story: "Toko online dengan product catalog"}, ""},
		{"workflow has no preset", &core.Project{Story: "Sistem workflow approval"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectPreset(tt.p)
			if tt.want == "" {
				if got != nil {
					t.Errorf("DetectPreset() = %s, want nil", got.ID)
				}
				return
			}
			if got == nil || got.ID != tt.want {
				t.Errorf("DetectPreset() = %v, want %s", got, tt.want)
			}
		})
	}
}

func TestPresetPromptConcrete(t *testing.T) {
	for _, pr := range Presets {
		t.Run(pr.ID, func(t *testing.T) {
			s := PresetPrompt(pr)
			if s == "" {
				t.Fatal("empty preset prompt")
			}
			for _, want := range []string{"VISUAL DIRECTION:", "LAYOUT BLUEPRINT", "PAGE PATTERN RULES:", "INTERACTION RULES:", "DESIGN TOKEN ROLES:", "ACCESSIBILITY:", "RESPONSIVE:", "QUALITY CHECKLIST"} {
				if !containsString(s, want) {
					t.Errorf("preset %s prompt missing %q", pr.ID, want)
				}
			}
			// Presets must not lock a framework or UI library.
			for _, banned := range []string{"React", "Vue", "Tailwind", "Material UI", "shadcn", "Angular", "Svelte"} {
				if containsString(s, banned) {
					t.Errorf("preset %s names framework/library %q; presets must be framework-agnostic", pr.ID, banned)
				}
			}
			if len(pr.Direction) == 0 || len(pr.Principles) == 0 || len(pr.AppShell) == 0 || len(pr.QualityChecklist) == 0 {
				t.Errorf("preset %s missing a concrete section", pr.ID)
			}
		})
	}
}

func TestPresetInjectedIntoDesignPrompt(t *testing.T) {
	p := &core.Project{
		Name:  "SaaS Dashboard",
		Story: "Platform SaaS dengan dashboard metrik dan subscription",
		Platform: &core.PlatformPlan{
			Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}},
		},
	}
	prompt := DesignPrompt(p)
	if !containsString(prompt, "UI DESIGN PRESET: Clean SaaS Dashboard") {
		t.Error("design prompt should include the detected UI design preset")
	}
}

func TestNoPresetForNonUIPlatforms(t *testing.T) {
	// A backend-only project has no design prompt: the design stage only runs
	// for products with screens. The UI design preset is therefore never
	// injected into a project that has no UI.
	p := &core.Project{
		Story: "Layanan API backend untuk pemrosesan data",
		Platform: &core.PlatformPlan{
			Targets: []core.PlatformTarget{{Kind: core.PlatformBackend}},
		},
	}
	if p.HasUI() {
		t.Fatal("backend-only project must not report HasUI")
	}
	// DetectPreset is deterministic even for a non-UI project (it never
	// returns a nil dereference path), but the pipeline never injects it.
	if got := DetectPreset(p); got == nil {
		t.Error("DetectPreset must always resolve for a non-nil project")
	}
}
