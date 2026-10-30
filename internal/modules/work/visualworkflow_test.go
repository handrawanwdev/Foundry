package work

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

// TestHasUI tests the HasUI function with various project states.
func TestHasUI(t *testing.T) {
	tests := []struct {
		name     string
		project  *core.Project
		expected bool
	}{
		{
			name:     "nil project",
			project:  nil,
			expected: false,
		},
		{
			name:     "project without UI",
			project:  &core.Project{},
			expected: false,
		},
		{
			name: "project with nil UI",
			project: &core.Project{
				UI: nil,
			},
			expected: false,
		},
		{
			name: "project with UI but nil layout",
			project: &core.Project{
				UI: &core.UIDesign{},
			},
			expected: false,
		},
		{
			name: "project with UI and layout but no pages",
			project: &core.Project{
				UI: &core.UIDesign{
					Layout: &core.LayoutSpec{
						Pages: []core.LayoutPage{},
					},
				},
			},
			expected: false,
		},
		{
			name: "project with UI and pages",
			project: &core.Project{
				UI: &core.UIDesign{
					Layout: &core.LayoutSpec{
						Pages: []core.LayoutPage{
							{ID: "home", Name: "Home", Route: "/"},
						},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasUI(tt.project)
			if result != tt.expected {
				t.Errorf("HasUI() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestVisualVerificationCommands tests the generation of visual verification commands.
func TestVisualVerificationCommands(t *testing.T) {
	t.Run("returns nil for project without UI", func(t *testing.T) {
		p := &core.Project{}
		cmds := VisualVerificationCommands(p, nil)
		if cmds != nil {
			t.Errorf("expected nil for project without UI, got %v", cmds)
		}
	})

	t.Run("generates correct commands for project with UI", func(t *testing.T) {
		p := &core.Project{
			UI: &core.UIDesign{
				Layout: &core.LayoutSpec{
					Pages: []core.LayoutPage{
						{ID: "home", Name: "Home", Route: "/"},
						{ID: "about", Name: "About", Route: "/about"},
					},
				},
			},
		}

		existingCmds := []core.Command{
			{ID: "CMD-001", Name: "Implement backend"},
			{ID: "CMD-002", Name: "Build home page", Artifacts: []core.ArtifactRef{{ID: "design-rules"}}},
			{ID: "CMD-003", Name: "Build about page", Artifacts: []core.ArtifactRef{{ID: "layout-spec"}}},
		}

		cmds := VisualVerificationCommands(p, existingCmds)

		if len(cmds) != 6 {
			t.Fatalf("expected 6 commands, got %d", len(cmds))
		}

		// Verify command IDs (platform-specific for web)
		expectedIDs := []string{
			"CMD-START-APP-WEB",
			"CMD-ENUMERATE-ROUTES-WEB",
			"CMD-CAPTURE-SCREENSHOTS-WEB",
			"CMD-SUBMIT-VISUAL-REVIEWS",
			"CMD-CREATE-VISUAL-FIXES",
			"CMD-RE-VISUAL-VERIFY",
		}
		for i, expectedID := range expectedIDs {
			if cmds[i].ID != expectedID {
				t.Errorf("command %d: expected ID %s, got %s", i, expectedID, cmds[i].ID)
			}
		}

		// Verify dependencies
		if len(cmds[0].DependsOn) != 2 {
			t.Errorf("CMD-START-APP-WEB should depend on 2 commands, got %d", len(cmds[0].DependsOn))
		}
		if cmds[1].DependsOn[0] != "CMD-START-APP-WEB" {
			t.Errorf("CMD-ENUMERATE-ROUTES-WEB should depend on CMD-START-APP-WEB")
		}
		if cmds[2].DependsOn[0] != "CMD-ENUMERATE-ROUTES-WEB" {
			t.Errorf("CMD-CAPTURE-SCREENSHOTS-WEB should depend on CMD-ENUMERATE-ROUTES-WEB")
		}

		// Verify artifacts
		for _, cmd := range cmds {
			if len(cmd.Artifacts) == 0 {
				t.Errorf("command %s should have artifacts", cmd.ID)
			}
		}

		// Verify instructions are not empty
		for _, cmd := range cmds {
			if cmd.Instructions == "" {
				t.Errorf("command %s should have instructions", cmd.ID)
			}
		}
	})

	t.Run("generates platform-specific commands for multi-platform project", func(t *testing.T) {
		p := &core.Project{
			Platform: &core.PlatformPlan{
				Targets: []core.PlatformTarget{
					{Kind: core.PlatformWeb},
					{Kind: core.PlatformAndroid},
					{Kind: core.PlatformIOS},
				},
			},
			UI: &core.UIDesign{
				Layout: &core.LayoutSpec{
					Pages: []core.LayoutPage{
						{ID: "home", Name: "Home", Platform: core.PlatformWeb, Route: "/"},
						{ID: "profile", Name: "Profile", Platform: core.PlatformAndroid},
						{ID: "settings", Name: "Settings", Platform: core.PlatformIOS},
					},
				},
			},
		}

		existingCmds := []core.Command{
			{ID: "CMD-001", Name: "Implement backend"},
		}

		cmds := VisualVerificationCommands(p, existingCmds)

		// Should have: 3 start commands (web, android, ios) + 3 enumerate + 3 capture + 1 submit + 1 fix + 1 reverify = 12
		if len(cmds) != 12 {
			t.Fatalf("expected 12 commands for 3 platforms, got %d", len(cmds))
		}

		// Verify platform-specific command IDs exist
		expectedPlatformCmds := map[string]bool{
			"CMD-START-APP-WEB":               false,
			"CMD-START-APP-ANDROID":           false,
			"CMD-START-APP-IOS":               false,
			"CMD-ENUMERATE-ROUTES-WEB":        false,
			"CMD-ENUMERATE-ROUTES-ANDROID":    false,
			"CMD-ENUMERATE-ROUTES-IOS":        false,
			"CMD-CAPTURE-SCREENSHOTS-WEB":     false,
			"CMD-CAPTURE-SCREENSHOTS-ANDROID": false,
			"CMD-CAPTURE-SCREENSHOTS-IOS":     false,
			"CMD-SUBMIT-VISUAL-REVIEWS":       false,
			"CMD-CREATE-VISUAL-FIXES":         false,
			"CMD-RE-VISUAL-VERIFY":            false,
		}

		for _, cmd := range cmds {
			if _, exists := expectedPlatformCmds[cmd.ID]; exists {
				expectedPlatformCmds[cmd.ID] = true
			}
		}

		for cmdID, found := range expectedPlatformCmds {
			if !found {
				t.Errorf("expected command %s not found", cmdID)
			}
		}

		// Verify CMD-SUBMIT-VISUAL-REVIEWS depends on all capture commands
		var submitCmd *core.Command
		for i := range cmds {
			if cmds[i].ID == "CMD-SUBMIT-VISUAL-REVIEWS" {
				submitCmd = &cmds[i]
				break
			}
		}
		if submitCmd == nil {
			t.Fatal("CMD-SUBMIT-VISUAL-REVIEWS not found")
		}
		if len(submitCmd.DependsOn) != 3 {
			t.Errorf("CMD-SUBMIT-VISUAL-REVIEWS should depend on 3 capture commands, got %d", len(submitCmd.DependsOn))
		}
	})
}

// TestImplementationDependencies tests the implementationDependencies function.
func TestImplementationDependencies(t *testing.T) {
	cmds := []core.Command{
		{ID: "CMD-001", Name: "Backend API"},
		{ID: "CMD-002", Name: "Home page", Artifacts: []core.ArtifactRef{{ID: "design-rules"}}},
		{ID: "CMD-003", Name: "About page", Artifacts: []core.ArtifactRef{{ID: "layout-spec"}}},
		{ID: "CMD-004", Name: "Contact page", Artifacts: []core.ArtifactRef{{ID: "design-rules"}, {ID: "layout-spec"}}},
	}

	deps := implementationDependencies(cmds)

	if len(deps) != 3 {
		t.Fatalf("expected 3 dependencies, got %d", len(deps))
	}

	expected := map[string]bool{
		"CMD-002": true,
		"CMD-003": true,
		"CMD-004": true,
	}

	for _, dep := range deps {
		if !expected[dep] {
			t.Errorf("unexpected dependency: %s", dep)
		}
	}
}

// TestAppendVisualVerificationCommands tests the automatic appending of visual verification commands.
func TestAppendVisualVerificationCommands(t *testing.T) {
	t.Run("skips for project without UI", func(t *testing.T) {
		p := &core.Project{
			Workflow: &core.Workflow{
				Commands: []core.Command{
					{ID: "CMD-001", Name: "Test command"},
				},
			},
		}

		err := appendVisualVerificationCommands(p, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(p.Workflow.Commands) != 1 {
			t.Errorf("expected 1 command, got %d", len(p.Workflow.Commands))
		}
	})

	t.Run("appends commands for project with UI", func(t *testing.T) {
		p := &core.Project{
			Requirements: []core.Requirement{
				{ID: "REQ-001", Text: "Test requirement", Priority: "must"},
			},
			UI: &core.UIDesign{
				Layout: &core.LayoutSpec{
					Pages: []core.LayoutPage{
						{ID: "home", Name: "Home", Route: "/"},
					},
				},
			},
			Workflow: &core.Workflow{
				Commands: []core.Command{
					{
						ID:           "CMD-001",
						Name:         "Test command",
						Objective:    "Test objective",
						Instructions: "Test instructions",
						Artifacts:    []core.ArtifactRef{{ID: "requirements"}},
						Covers:       []string{"REQ-001"},
					},
				},
			},
		}

		docs := []core.Artifact{
			{ID: "requirements", Title: "Requirements", Body: "Test"},
			{ID: "design-rules", Title: "Design Rules", Body: "Test"},
			{ID: "layout-spec", Title: "Layout Spec", Body: "Test"},
		}

		err := appendVisualVerificationCommands(p, docs)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Should have original command + 6 visual verification commands
		if len(p.Workflow.Commands) != 7 {
			t.Errorf("expected 7 commands, got %d", len(p.Workflow.Commands))
		}

		// Verify version incremented
		if p.Workflow.Version != 1 {
			t.Errorf("expected version 1, got %d", p.Workflow.Version)
		}
	})

	t.Run("skips if visual commands already exist", func(t *testing.T) {
		p := &core.Project{
			UI: &core.UIDesign{
				Layout: &core.LayoutSpec{
					Pages: []core.LayoutPage{
						{ID: "home", Name: "Home", Route: "/"},
					},
				},
			},
			Workflow: &core.Workflow{
				Commands: []core.Command{
					{ID: "CMD-001", Name: "Test command"},
					{ID: "CMD-START-APP", Name: "Already exists"},
				},
			},
		}

		err := appendVisualVerificationCommands(p, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Should not add more commands
		if len(p.Workflow.Commands) != 2 {
			t.Errorf("expected 2 commands, got %d", len(p.Workflow.Commands))
		}
	})
}

// TestWorkflowPromptIncludesVisualRules tests that workflow prompts include visual verification rules when project has UI.
func TestWorkflowPromptIncludesVisualRules(t *testing.T) {
	docs := []core.Artifact{
		{ID: "requirements", Title: "Requirements", Category: "requirements", Body: "Test requirements"},
		{ID: "design-rules", Title: "Design Rules", Category: "design", Body: "Test design rules"},
		{ID: "layout-spec", Title: "Layout Spec", Category: "design", Body: "Test layout spec"},
	}

	t.Run("fresh prompt without UI", func(t *testing.T) {
		p := &core.Project{
			Name:  "Test Project",
			Story: "Test story",
		}

		prompt := WorkflowPrompt(p, docs)
		if strings.Contains(prompt, VisualVerificationRules) {
			t.Error("prompt should not include visual verification rules for project without UI")
		}
	})

	t.Run("fresh prompt with UI", func(t *testing.T) {
		p := &core.Project{
			Name:  "Test Project",
			Story: "Test story",
			UI: &core.UIDesign{
				Layout: &core.LayoutSpec{
					Pages: []core.LayoutPage{
						{ID: "home", Name: "Home", Route: "/"},
					},
				},
			},
		}

		prompt := WorkflowPrompt(p, docs)
		if !strings.Contains(prompt, VisualVerificationRules) {
			t.Error("prompt should include visual verification rules for project with UI")
		}
	})

	t.Run("sketch prompt with UI", func(t *testing.T) {
		// Create a large blueprint to trigger sketch mode
		largeDocs := make([]core.Artifact, 0)
		largeBody := strings.Repeat("x", 60000) // > 50KB threshold
		largeDocs = append(largeDocs, core.Artifact{
			ID:       "requirements",
			Title:    "Requirements",
			Category: "requirements",
			Body:     largeBody,
		})

		p := &core.Project{
			Name:  "Test Project",
			Story: "Test story",
			UI: &core.UIDesign{
				Layout: &core.LayoutSpec{
					Pages: []core.LayoutPage{
						{ID: "home", Name: "Home", Route: "/"},
					},
				},
			},
		}

		prompt := WorkflowPrompt(p, largeDocs)
		if !strings.Contains(prompt, "SKETCH PHASE") {
			t.Error("expected sketch phase prompt")
		}
		if !strings.Contains(prompt, VisualVerificationRules) {
			t.Error("sketch prompt should include visual verification rules for project with UI")
		}
	})

	t.Run("revision prompt with UI", func(t *testing.T) {
		p := &core.Project{
			Name:  "Test Project",
			Story: "Test story",
			UI: &core.UIDesign{
				Layout: &core.LayoutSpec{
					Pages: []core.LayoutPage{
						{ID: "home", Name: "Home", Route: "/"},
					},
				},
			},
			Workflow: &core.Workflow{
				Commands: []core.Command{
					{ID: "CMD-001", Name: "Test", Instructions: "Test instructions"},
				},
			},
		}

		prompt := WorkflowPrompt(p, docs)
		if !strings.Contains(prompt, VisualVerificationRules) {
			t.Error("revision prompt should include visual verification rules for project with UI")
		}
	})
}
