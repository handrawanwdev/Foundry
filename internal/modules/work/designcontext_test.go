package work

import (
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

// TestDetectProductType tests product type detection from project story and features.
func TestDetectProductType(t *testing.T) {
	tests := []struct {
		name     string
		project  *core.Project
		expected ProductType
	}{
		{
			name:     "nil project returns generic",
			project:  nil,
			expected: ProductGeneric,
		},
		{
			name: "empty project returns generic",
			project: &core.Project{
				Story: "A simple application",
			},
			expected: ProductGeneric,
		},
		{
			name: "dashboard detection from story",
			project: &core.Project{
				Story: "Dashboard monitoring untuk sistem trading",
			},
			expected: ProductDashboard,
		},
		{
			name: "trading dashboard detection",
			project: &core.Project{
				Story: "Aplikasi trading dengan dashboard metrik keuangan",
			},
			expected: ProductDashboard,
		},
		{
			name: "admin system detection from story",
			project: &core.Project{
				Story: "Sistem administrasi untuk manajemen user dan role",
			},
			expected: ProductAdmin,
		},
		{
			name: "admin system detection from features",
			project: &core.Project{
				Story: "A management system",
				Features: []core.Feature{
					{Name: "User Management"},
					{Name: "Role Permissions"},
				},
			},
			expected: ProductAdmin,
		},
		{
			name: "SaaS detection",
			project: &core.Project{
				Story: "Platform SaaS multi-tenant dengan subscription",
			},
			expected: ProductSaaS,
		},
		{
			name: "mobile app detection from story",
			project: &core.Project{
				Story: "Mobile app untuk Android dan iOS",
			},
			expected: ProductMobile,
		},
		{
			name: "mobile app detection from platform",
			project: &core.Project{
				Story: "A fitness tracking app",
				Platform: &core.PlatformPlan{
					Targets: []core.PlatformTarget{
						{Kind: core.PlatformAndroid},
					},
				},
			},
			expected: ProductMobile,
		},
		{
			name: "e-commerce detection",
			project: &core.Project{
				Story: "Toko online dengan product catalog dan checkout",
			},
			expected: ProductECommerce,
		},
		{
			name: "content platform detection",
			project: &core.Project{
				Story: "Platform konten untuk blog dan artikel",
			},
			expected: ProductContent,
		},
		{
			name: "communication app detection",
			project: &core.Project{
				Story: "Aplikasi chat dan messaging real-time",
			},
			expected: ProductCommunication,
		},
		{
			name: "workflow system detection",
			project: &core.Project{
				Story: "Sistem workflow untuk approval process",
			},
			expected: ProductWorkflow,
		},
		{
			name: "analytics detection",
			project: &core.Project{
				Story: "Tool analytics untuk monitoring performance",
			},
			expected: ProductAnalytics,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectProductType(tt.project)
			if result != tt.expected {
				t.Errorf("DetectProductType() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestContextualDesignRules tests that contextual rules are returned for each product type.
func TestContextualDesignRules(t *testing.T) {
	productTypes := []ProductType{
		ProductDashboard,
		ProductAdmin,
		ProductSaaS,
		ProductMobile,
		ProductAnalytics,
		ProductECommerce,
		ProductContent,
		ProductCommunication,
		ProductWorkflow,
		ProductGeneric,
	}

	for _, pt := range productTypes {
		t.Run(string(pt), func(t *testing.T) {
			rules := ContextualDesignRules(pt)
			if rules == "" {
				t.Errorf("ContextualDesignRules(%s) returned empty string", pt)
			}
			if len(rules) < 100 {
				t.Errorf("ContextualDesignRules(%s) returned very short rules (%d chars)", pt, len(rules))
			}
		})
	}
}

// TestDetectProductTypeAndRules tests the combined function.
func TestDetectProductTypeAndRules(t *testing.T) {
	p := &core.Project{
		Story: "Dashboard monitoring untuk sistem trading",
	}

	productType, rules := DetectProductTypeAndRules(p)
	if productType != ProductDashboard {
		t.Errorf("Expected ProductDashboard, got %v", productType)
	}
	if rules == "" {
		t.Error("Expected non-empty rules")
	}
	if !containsString(rules, "PRODUCT CONTEXT: Dashboard/Analytics") {
		t.Error("Rules don't contain expected product context")
	}
}

// TestContextualDesignRulesIntegration tests that contextual rules are integrated into design prompt.
func TestContextualDesignRulesIntegration(t *testing.T) {
	p := &core.Project{
		Name:  "Trading Dashboard",
		Story: "Dashboard monitoring untuk sistem trading dengan metrik keuangan",
		Platform: &core.PlatformPlan{
			Targets: []core.PlatformTarget{
				{Kind: core.PlatformWeb},
			},
		},
		Features: []core.Feature{
			{
				ID:          "FEAT-001",
				Name:        "Real-time Metrics",
				Description: "Show real-time trading metrics",
				Platforms:   []string{core.PlatformWeb},
			},
		},
		Requirements: []core.Requirement{
			{ID: "REQ-001", Text: "Must show real-time data", Priority: "must"},
		},
	}

	prompt := DesignPrompt(p)

	// Check that product context is included
	if !containsString(prompt, "PRODUCT CONTEXT:") {
		t.Error("Design prompt should include product context")
	}

	// Check that dashboard-specific guidance is included
	if !containsString(prompt, "information density") {
		t.Error("Dashboard product context should mention information density")
	}

	// Check that generic warnings are not included
	if containsString(prompt, "Avoid generic dashboards") {
		t.Error("Dashboard product should not include generic dashboard warning")
	}
}

// TestGenericProductWarning tests that generic products get appropriate warnings.
func TestGenericProductWarning(t *testing.T) {
	p := &core.Project{
		Name:  "Simple App",
		Story: "A simple application with basic features",
		Platform: &core.PlatformPlan{
			Targets: []core.PlatformTarget{
				{Kind: core.PlatformWeb},
			},
		},
		Requirements: []core.Requirement{
			{ID: "REQ-001", Text: "Must work", Priority: "must"},
		},
	}

	prompt := DesignPrompt(p)

	// Generic product should get the generic guidance with anti-dashboard warning
	if !containsString(prompt, "PRODUCT CONTEXT: General Application") {
		t.Error("Generic product should have general application context")
	}
	if !containsString(prompt, "Avoid generic dashboards") {
		t.Error("Generic product should warn against generic dashboards")
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
