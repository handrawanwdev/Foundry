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

// ContextualDesignRules returns design rules specific to the product type.
// These rules prevent generic patterns and enforce contextual decisions.
func ContextualDesignRules(productType ProductType) string {
	switch productType {
	case ProductDashboard:
		return `
PRODUCT CONTEXT: Dashboard/Analytics (data-heavy, monitoring focus)
DESIGN GUIDANCE:
- Prioritize information density: operators need to see many data points at once.
- Use compact tables with sortable columns, not cards, for data comparison.
- Dashboard should show real-time or recent data, not historical archives.
- KPI cards should show trend indicators (up/down arrows, sparklines) not just numbers.
- Avoid decorative elements; every pixel must serve a data insight purpose.
- Use color strategically: red for critical alerts, green for healthy, gray for neutral.
- Charts must have clear legends, axis labels, and tooltips. No chart without context.
- Dense layout is acceptable; operators spend hours monitoring, not browsing.
- Empty states should explain what data will appear, not just say "no data".
- Do not use card grids for data that belongs in tables.
`

	case ProductAdmin:
		return `
PRODUCT CONTEXT: Admin/Management System (operational, task-focused)
DESIGN GUIDANCE:
- Prioritize efficiency: minimize clicks to complete common tasks.
- Use sidebar navigation for quick access to management sections.
- Tables should support bulk actions (select multiple, bulk delete, bulk export).
- Forms should be scannable: group related fields, use clear labels, show validation inline.
- Status indicators must be prominent: badges, progress bars, timeline views.
- Audit trails and activity logs are essential; show who did what and when.
- Permission-based UI: hide or disable actions the user cannot perform.
- Search and filter are critical: admin systems often have large datasets.
- Avoid marketing-style layouts; this is a tool, not a showcase.
- Empty states should guide users to create their first item.
`

	case ProductSaaS:
		return `
PRODUCT CONTEXT: SaaS/Productivity Application (multi-tenant, subscription-based)
DESIGN GUIDANCE:
- Onboarding is critical: guide users through setup with wizards or checklists.
- Workspace/tenant switching must be obvious if multi-tenant.
- Billing and subscription UI should be accessible but not intrusive.
- Feature gating: clearly show what's available in the user's plan.
- Settings should be organized by category, not dumped in one page.
- Use progressive disclosure: show common options first, advanced on demand.
- Empty states should educate: explain the feature and how to get started.
- Notifications should be non-intrusive but discoverable.
- Avoid clutter; SaaS users expect clean, professional interfaces.
- Respect the user's time: every interaction should feel purposeful.
`

	case ProductMobile:
		return `
PRODUCT CONTEXT: Mobile Application (touch-first, small screen)
DESIGN GUIDANCE:
- Touch targets must be at least 44x44px; prefer 48x48px for primary actions.
- Use bottom navigation for primary destinations (thumb-friendly).
- Avoid wide tables; use lists or cards with key information, details on tap.
- Gestures should complement, not replace, visible buttons.
- Loading states must be visible; mobile networks are unreliable.
- Offline support: show cached data, indicate when offline, sync when back online.
- Forms should be short; break long forms into steps.
- Use native platform patterns (Material Design for Android, HIG for iOS).
- Avoid hover states; mobile has no hover.
- Prioritize vertical scrolling; horizontal scrolling is disorienting.
- Respect safe areas (notches, home indicator).
- Use platform-native navigation patterns (back swipe, bottom sheets).
`

	case ProductAnalytics:
		return `
PRODUCT CONTEXT: Analytics/Reporting Tool (data exploration, insight-focused)
DESIGN GUIDANCE:
- Charts are the primary content; give them space and clarity.
- Every chart must have: title, axis labels, legend, and data source.
- Use appropriate chart types: line for trends, bar for comparisons, pie only for parts-of-whole.
- Filters and date range pickers are critical; make them prominent.
- Export options (CSV, PDF, PNG) should be obvious.
- Drill-down: allow users to click chart elements to see details.
- Avoid decorative chart junk; data-ink ratio should be high.
- Use consistent color encoding across charts (same metric = same color).
- Tooltips should show exact values, not approximations.
- Dashboard layouts should be customizable if users need different views.
- Empty states should suggest what data to add or what insights to explore.
`

	case ProductECommerce:
		return `
PRODUCT CONTEXT: E-commerce/Product Catalog (shopping, conversion-focused)
DESIGN GUIDANCE:
- Product cards must show: image, title, price, and primary action (Add to Cart).
- Filters and sorting are critical for product discovery.
- Search should support autocomplete and suggestions.
- Product detail pages need: images (gallery), description, variants, reviews, related products.
- Cart should be accessible from every page (sticky header or floating button).
- Checkout flow should be linear: no distractions, clear progress indicator.
- Trust signals: security badges, return policy, customer reviews.
- Mobile: prioritize product images, simplify checkout.
- Empty cart should suggest popular products or categories.
- Out-of-stock items should be clearly marked with alternatives suggested.
- Price should be prominent; discounts should be visually distinct.
`

	case ProductContent:
		return `
PRODUCT CONTEXT: Content/Media Platform (reading, publishing focus)
DESIGN GUIDANCE:
- Typography is critical: readable fonts, appropriate line height, proper contrast.
- Content should be the hero; minimize UI chrome around it.
- Reading mode: allow users to focus on content without distractions.
- Article lists should show: title, excerpt, author, date, reading time.
- Images should be optimized and responsive; lazy-load below the fold.
- Social sharing should be accessible but not intrusive.
- Comments section should be threaded and moderated.
- Search should search within content, with filters for category/date.
- Empty states should encourage content creation or exploration.
- Avoid clutter; content platforms need whitespace for readability.
`

	case ProductCommunication:
		return `
PRODUCT CONTEXT: Communication/Messaging App (real-time, conversation-focused)
DESIGN GUIDANCE:
- Conversations should be the primary view; list on left, messages on right (desktop).
- Message bubbles should clearly distinguish sent vs received.
- Typing indicators and read receipts build trust.
- Notifications should be granular: per-conversation, per-type.
- File attachments should show previews and download options.
- Search should search across messages, contacts, and files.
- Group chats need member lists, roles, and permissions.
- Mobile: use full-screen conversations, list view for threads.
- Empty states should suggest starting a conversation or inviting contacts.
- Avoid clutter; communication apps need focus on the conversation.
`

	case ProductWorkflow:
		return `
PRODUCT CONTEXT: Workflow/Process Management (task progression, approval-focused)
DESIGN GUIDANCE:
- Workflow status should be prominent: progress bars, step indicators, timelines.
- Task lists should show: assignee, due date, priority, status.
- Approval flows need clear action buttons: Approve, Reject, Request Changes.
- Notifications for workflow events: assigned, due soon, overdue, completed.
- Audit trail: who did what, when, with what outcome.
- Filters by status, assignee, due date are critical.
- Kanban boards for visual workflow management (if appropriate).
- Empty states should guide users to create their first workflow.
- Avoid clutter; workflow apps need clarity on what needs attention.
`

	default: // ProductGeneric
		return `
PRODUCT CONTEXT: General Application
DESIGN GUIDANCE:
- Understand the primary user task before choosing layout patterns.
- Avoid generic dashboards unless the product is explicitly for monitoring/analytics.
- Choose patterns by task: tables for comparison, forms for input, lists for scanning.
- Every page should have a clear purpose and primary action.
- Respect the user's time; minimize unnecessary clicks and visual noise.
`
	}
}

// DetectProductTypeAndRules returns both the detected product type and contextual rules.
func DetectProductTypeAndRules(p *core.Project) (ProductType, string) {
	productType := DetectProductType(p)
	return productType, ContextualDesignRules(productType)
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
