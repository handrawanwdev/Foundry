package learning

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

func TestStoreRecord(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	findings := []core.VisualFinding{
		{
			Page:      "dashboard",
			Criterion: "contrast",
			Severity:  "high",
			Message:   "Low contrast ratio",
		},
		{
			Page:      "dashboard",
			Criterion: "spacing",
			Severity:  "medium",
			Message:   "Inconsistent spacing",
		},
	}

	err := store.Record("test-project", "dashboard", findings)
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	// Verify file was created
	path := filepath.Join(tmpDir, learningFileName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("Learning file was not created")
	}

	// Verify we can load it back
	records, err := store.loadAll()
	if err != nil {
		t.Fatalf("loadAll failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}

	if records[0].ProjectID != "test-project" {
		t.Errorf("Expected project ID 'test-project', got %s", records[0].ProjectID)
	}

	if len(records[0].Findings) != 2 {
		t.Errorf("Expected 2 findings, got %d", len(records[0].Findings))
	}
}

func TestStoreGetPatterns(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Create some records with patterns
	records := []core.LearningRecord{
		{
			ProjectID:   "project1",
			ProductType: "dashboard",
			Timestamp:   time.Now(),
			PatternsLearned: []core.DesignPattern{
				{
					ID:          "pattern1",
					Name:        "Dashboard Layout",
					Description: "Standard dashboard layout pattern",
					ProductType: "dashboard",
					SuccessRate: 0.9,
					UsageCount:  10,
				},
			},
		},
		{
			ProjectID:   "project2",
			ProductType: "dashboard",
			Timestamp:   time.Now(),
			PatternsLearned: []core.DesignPattern{
				{
					ID:          "pattern1",
					Name:        "Dashboard Layout",
					Description: "Standard dashboard layout pattern",
					ProductType: "dashboard",
					SuccessRate: 0.95,
					UsageCount:  5,
				},
			},
		},
	}

	err := store.saveAll(records)
	if err != nil {
		t.Fatalf("saveAll failed: %v", err)
	}

	patterns, err := store.GetPatterns("dashboard")
	if err != nil {
		t.Fatalf("GetPatterns failed: %v", err)
	}

	if len(patterns) != 1 {
		t.Errorf("Expected 1 pattern, got %d", len(patterns))
	}

	// Check that patterns were merged
	if patterns[0].UsageCount != 15 {
		t.Errorf("Expected merged usage count 15, got %d", patterns[0].UsageCount)
	}

	// Check success rate was averaged
	expectedRate := (0.9*10 + 0.95*5) / 15.0
	if patterns[0].SuccessRate < expectedRate-0.01 || patterns[0].SuccessRate > expectedRate+0.01 {
		t.Errorf("Expected success rate ~%.2f, got %.2f", expectedRate, patterns[0].SuccessRate)
	}
}

func TestStoreGetSummary(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	records := []core.LearningRecord{
		{
			ProjectID:   "project1",
			ProductType: "dashboard",
			Timestamp:   time.Now(),
			Findings: []core.LearningFinding{
				{Criterion: "contrast", Severity: "high", Frequency: 1},
				{Criterion: "spacing", Severity: "medium", Frequency: 1},
			},
		},
		{
			ProjectID:   "project2",
			ProductType: "admin",
			Timestamp:   time.Now(),
			Findings: []core.LearningFinding{
				{Criterion: "contrast", Severity: "high", Frequency: 1},
				{Criterion: "labels", Severity: "low", Frequency: 1},
			},
		},
	}

	err := store.saveAll(records)
	if err != nil {
		t.Fatalf("saveAll failed: %v", err)
	}

	summary, err := store.GetSummary()
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}

	if summary.TotalProjects != 2 {
		t.Errorf("Expected 2 projects, got %d", summary.TotalProjects)
	}

	if len(summary.ProductTypeStats) != 2 {
		t.Errorf("Expected 2 product types, got %d", len(summary.ProductTypeStats))
	}

	if summary.ProductTypeStats["dashboard"] != 1 {
		t.Errorf("Expected 1 dashboard project, got %d", summary.ProductTypeStats["dashboard"])
	}

	// Check common findings
	if len(summary.CommonFindings) == 0 {
		t.Error("Expected some common findings")
	}

	// Find contrast finding
	foundContrast := false
	for _, f := range summary.CommonFindings {
		if f.Criterion == "contrast" {
			foundContrast = true
			if f.Count != 2 {
				t.Errorf("Expected contrast count 2, got %d", f.Count)
			}
			break
		}
	}
	if !foundContrast {
		t.Error("Expected to find contrast in common findings")
	}

	// Check recommended rules were generated
	if len(summary.RecommendedRules) == 0 {
		t.Error("Expected some recommended rules")
	}
}

func TestStoreRecordDesignPattern(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	pattern := core.DesignPattern{
		ID:           "test-pattern",
		Name:         "Test Pattern",
		Description:  "A test design pattern",
		ProductType:  "dashboard",
		Category:     "layout",
		BestPractice: "Use this approach",
		SuccessRate:  0.85,
		UsageCount:   5,
	}

	err := store.RecordDesignPattern(pattern)
	if err != nil {
		t.Fatalf("RecordDesignPattern failed: %v", err)
	}

	// Verify we can retrieve it
	patterns, err := store.GetPatterns("")
	if err != nil {
		t.Fatalf("GetPatterns failed: %v", err)
	}

	if len(patterns) != 1 {
		t.Errorf("Expected 1 pattern, got %d", len(patterns))
	}

	if patterns[0].ID != "test-pattern" {
		t.Errorf("Expected pattern ID 'test-pattern', got %s", patterns[0].ID)
	}
}

func TestStoreRecordEmptyFindings(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Record with empty findings should not create a record
	err := store.Record("test-project", "dashboard", []core.VisualFinding{})
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	records, err := store.loadAll()
	if err != nil {
		t.Fatalf("loadAll failed: %v", err)
	}

	if len(records) != 0 {
		t.Errorf("Expected 0 records for empty findings, got %d", len(records))
	}
}

func TestGenerateRecommendedRules(t *testing.T) {
	findings := []core.FindingPattern{
		{Criterion: "contrast", Severity: "high", Count: 10, Frequency: 0.5},
		{Criterion: "spacing", Severity: "medium", Count: 5, Frequency: 0.25},
		{Criterion: "rare", Severity: "low", Count: 1, Frequency: 0.05},
	}

	rules := generateRecommendedRules(findings)

	// Should include rules for contrast and spacing (frequency >= 0.1)
	// Should not include rule for rare (frequency < 0.1)
	if len(rules) < 2 {
		t.Errorf("Expected at least 2 rules, got %d", len(rules))
	}

	foundContrast := false
	foundSpacing := false
	for _, rule := range rules {
		if rule == "Ensure all text meets WCAG AA contrast requirements (4.5:1 for normal text, 3:1 for large text)" {
			foundContrast = true
		}
		if rule == "Use consistent spacing tokens throughout the design" {
			foundSpacing = true
		}
	}

	if !foundContrast {
		t.Error("Expected contrast rule to be generated")
	}
	if !foundSpacing {
		t.Error("Expected spacing rule to be generated")
	}
}
