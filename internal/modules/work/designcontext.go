package work

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/learning"
)

// ProductType represents the category of product being designed.
type ProductType string

const (
	ProductDashboard     ProductType = "dashboard"
	ProductAdmin         ProductType = "admin"
	ProductSaaS          ProductType = "saas"
	ProductMobile        ProductType = "mobile"
	ProductAnalytics     ProductType = "analytics"
	ProductECommerce     ProductType = "ecommerce"
	ProductContent       ProductType = "content"
	ProductCommunication ProductType = "communication"
	ProductWorkflow      ProductType = "workflow"
	ProductGeneric       ProductType = "generic"
)

// DetectProductType analyzes the project story and features to determine the product type.
// This enables contextual design guidance instead of generic patterns.
func DetectProductType(p *core.Project) ProductType {
	if p == nil {
		return ProductGeneric
	}

	story := strings.ToLower(p.Story)
	features := strings.ToLower(joinFeatures(p))

	// Dashboard/Analytics indicators
	if containsAny(story, []string{"dashboard", "analitik", "analytics", "monitoring", "metrik", "metrics", "laporan", "report"}) ||
		containsAny(features, []string{"dashboard", "chart", "grafik", "report", "laporan", "statistik"}) {
		if containsAny(story, []string{"trading", "keuangan", "financial", "investasi"}) {
			return ProductDashboard // Trading/financial dashboard needs dense, data-heavy layout
		}
		if containsAny(story, []string{"admin", "administrasi", "management"}) {
			return ProductAdmin // Admin dashboard needs operational focus
		}
		return ProductAnalytics
	}

	// Admin/Management system indicators
	if containsAny(story, []string{"admin", "administrasi", "management", "manajemen", "backend", "operator"}) ||
		containsAny(features, []string{"user management", "role", "permission", "approval", "workflow"}) {
		return ProductAdmin
	}

	// SaaS/Productivity indicators
	if containsAny(story, []string{"saas", "subscription", "langganan", "multi-tenant", "workspace"}) ||
		containsAny(features, []string{"subscription", "billing", "tenant", "workspace"}) {
		return ProductSaaS
	}

	// Mobile-first indicators
	if containsAny(story, []string{"mobile app", "aplikasi mobile", "android", "ios"}) ||
		(p.Platform != nil && hasMobilePlatform(p.Platform.Targets)) {
		return ProductMobile
	}

	// E-commerce indicators
	if containsAny(story, []string{"e-commerce", "ecommerce", "toko online", "shop", "belanja", "product catalog"}) ||
		containsAny(features, []string{"cart", "checkout", "product", "catalog", "order"}) {
		return ProductECommerce
	}

	// Content/Media indicators
	if containsAny(story, []string{"content", "konten", "blog", "article", "media", "publishing"}) ||
		containsAny(features, []string{"content", "article", "blog", "media", "publish"}) {
		return ProductContent
	}

	// Communication indicators
	if containsAny(story, []string{"chat", "messaging", "komunikasi", "communication", "social"}) ||
		containsAny(features, []string{"chat", "message", "conversation", "notification"}) {
		return ProductCommunication
	}

	// Workflow/Process indicators
	if containsAny(story, []string{"workflow", "proses", "process", "approval", "task management"}) ||
		containsAny(features, []string{"workflow", "task", "approval", "process"}) {
		return ProductWorkflow
	}

	return ProductGeneric
}

// GetLearnedDesignRules loads learned patterns from the learning store and returns
// them as design rules to be included in the design prompt.
func GetLearnedDesignRules(p *core.Project, learningDir string) string {
	if learningDir == "" {
		// Default to .foundry/learnings in project directory
		if p != nil && p.Dir != "" {
			learningDir = filepath.Join(p.Dir, ".foundry", "learnings")
		} else {
			return ""
		}
	}

	// Check if learning directory exists
	if _, err := os.Stat(learningDir); os.IsNotExist(err) {
		return ""
	}

	store := learning.NewStore(learningDir)
	productType := ""
	if p != nil {
		productType = string(DetectProductType(p))
	}

	// Get patterns for this product type
	patterns, err := store.GetPatterns(productType)
	if err != nil || len(patterns) == 0 {
		return ""
	}

	// Build learned rules text
	var rules strings.Builder
	rules.WriteString("\nLEARNED DESIGN PATTERNS (from previous projects):\n")
	rules.WriteString("These patterns have been validated across multiple projects and should be considered:\n\n")

	for i, pattern := range patterns {
		if i >= 5 {
			// Limit to top 5 patterns to avoid overwhelming the prompt
			break
		}

		rules.WriteString(fmt.Sprintf("- %s: %s", pattern.Name, pattern.Description))
		if pattern.BestPractice != "" {
			rules.WriteString(fmt.Sprintf(" Best practice: %s", pattern.BestPractice))
		}
		if pattern.AntiPattern != "" {
			rules.WriteString(fmt.Sprintf(" Avoid: %s", pattern.AntiPattern))
		}
		if pattern.SuccessRate > 0 {
			rules.WriteString(fmt.Sprintf(" (success rate: %.0f%%)", pattern.SuccessRate*100))
		}
		rules.WriteString("\n")
	}

	return rules.String()
}

// GetLearnedRecommendations loads recommended rules from learning summary.
func GetLearnedRecommendations(p *core.Project, learningDir string) string {
	if learningDir == "" {
		if p != nil && p.Dir != "" {
			learningDir = filepath.Join(p.Dir, ".foundry", "learnings")
		} else {
			return ""
		}
	}

	// Check if learning directory exists
	if _, err := os.Stat(learningDir); os.IsNotExist(err) {
		return ""
	}

	store := learning.NewStore(learningDir)
	summary, err := store.GetSummary()
	if err != nil || len(summary.RecommendedRules) == 0 {
		return ""
	}

	// Build recommendations text
	var recs strings.Builder
	recs.WriteString("\nRECOMMENDED DESIGN RULES (learned from project history):\n")
	for _, rule := range summary.RecommendedRules {
		recs.WriteString(fmt.Sprintf("- %s\n", rule))
	}

	return recs.String()
}

// Helper functions

func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

func joinFeatures(p *core.Project) string {
	var parts []string
	for _, f := range p.Features {
		parts = append(parts, f.Name)
		for _, sub := range f.SubFeatures {
			parts = append(parts, sub.Name)
		}
	}
	return strings.Join(parts, " ")
}

func hasMobilePlatform(targets []core.PlatformTarget) bool {
	for _, t := range targets {
		if t.Kind == core.PlatformAndroid || t.Kind == core.PlatformIOS {
			return true
		}
	}
	return false
}
