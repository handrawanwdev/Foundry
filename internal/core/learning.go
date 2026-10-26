package core

import (
	"time"
)

// LearningRecord captures design patterns and visual findings from a project
// for continuous improvement across projects.
type LearningRecord struct {
	ProjectID      string             `json:"project_id"`
	ProductType    string             `json:"product_type"`
	Timestamp      time.Time          `json:"timestamp"`
	Findings       []LearningFinding  `json:"findings"`
	PatternsLearned []DesignPattern   `json:"patterns_learned"`
}

// LearningFinding represents a visual finding that can be learned from.
type LearningFinding struct {
	Criterion   string `json:"criterion"`
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	Pattern     string `json:"pattern,omitempty"` // Page pattern (dashboard, form, etc.)
	Frequency   int    `json:"frequency"`
	RootCause   string `json:"root_cause,omitempty"`
	Resolution  string `json:"resolution,omitempty"`
}

// DesignPattern represents a reusable design pattern learned from projects.
type DesignPattern struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	ProductType  string   `json:"product_type"`
	Pattern      string   `json:"pattern,omitempty"` // Page pattern
	Category     string   `json:"category"` // layout, interaction, accessibility, etc.
	BestPractice string   `json:"best_practice"`
	AntiPattern  string   `json:"anti_pattern,omitempty"`
	Examples     []string `json:"examples,omitempty"`
	SuccessRate  float64  `json:"success_rate"` // 0.0 to 1.0
	UsageCount   int      `json:"usage_count"`
}

// LearningSummary provides aggregated insights from learning records.
type LearningSummary struct {
	TotalProjects     int               `json:"total_projects"`
	CommonFindings    []FindingPattern  `json:"common_findings"`
	RecommendedRules  []string          `json:"recommended_rules"`
	ProductTypeStats  map[string]int    `json:"product_type_stats"`
}

// FindingPattern represents a recurring finding pattern across projects.
type FindingPattern struct {
	Criterion   string  `json:"criterion"`
	Severity    string  `json:"severity"`
	Count       int     `json:"count"`
	Frequency   float64 `json:"frequency"` // 0.0 to 1.0
	Description string  `json:"description"`
}
