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
