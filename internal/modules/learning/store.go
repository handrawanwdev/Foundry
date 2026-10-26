package learning

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

const learningFileName = "learnings.json"

// Store manages persistent storage of learning records.
type Store struct {
	basePath string
}

// NewStore creates a new learning store at the specified base path.
func NewStore(basePath string) *Store {
	return &Store{basePath: basePath}
}

// Record captures learnings from a project's visual findings and saves them.
func (s *Store) Record(projectID string, productType string, findings []core.VisualFinding) error {
	if len(findings) == 0 {
		return nil
	}

	// Convert visual findings to learning findings
	learningFindings := make([]core.LearningFinding, 0, len(findings))
	for _, f := range findings {
		learningFindings = append(learningFindings, core.LearningFinding{
			Criterion: f.Criterion,
			Severity:  f.Severity,
			Message:   f.Message,
			Frequency: 1,
		})
	}

	record := core.LearningRecord{
		ProjectID:   projectID,
		ProductType: productType,
		Timestamp:   time.Now(),
		Findings:    learningFindings,
	}

	// Load existing records
	records, err := s.loadAll()
	if err != nil {
		records = []core.LearningRecord{}
	}

	// Append new record
	records = append(records, record)

	// Save back
	return s.saveAll(records)
}

// GetPatterns retrieves design patterns learned from all projects.
func (s *Store) GetPatterns(productType string) ([]core.DesignPattern, error) {
	records, err := s.loadAll()
	if err != nil {
		return nil, err
	}

	// Aggregate patterns
	patternMap := make(map[string]*core.DesignPattern)
	
	for _, record := range records {
		if productType != "" && record.ProductType != productType {
			continue
		}
		for _, pattern := range record.PatternsLearned {
			if existing, ok := patternMap[pattern.ID]; ok {
				// Merge usage stats
				existing.UsageCount += pattern.UsageCount
				existing.SuccessRate = (existing.SuccessRate*float64(existing.UsageCount) + pattern.SuccessRate*float64(pattern.UsageCount)) / float64(existing.UsageCount+pattern.UsageCount)
			} else {
				p := pattern
				patternMap[pattern.ID] = &p
			}
		}
	}

	// Convert to slice
	patterns := make([]core.DesignPattern, 0, len(patternMap))
	for _, p := range patternMap {
		patterns = append(patterns, *p)
	}

	// Sort by success rate
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].SuccessRate > patterns[j].SuccessRate
	})

	return patterns, nil
}

// GetSummary returns an aggregated summary of all learnings.
func (s *Store) GetSummary() (*core.LearningSummary, error) {
	records, err := s.loadAll()
	if err != nil {
		return nil, err
	}

	summary := &core.LearningSummary{
		TotalProjects:    len(records),
		ProductTypeStats: make(map[string]int),
	}

	// Count findings by criterion
	findingCounts := make(map[string]map[string]int) // criterion -> severity -> count
	for _, record := range records {
		summary.ProductTypeStats[record.ProductType]++
		
		for _, finding := range record.Findings {
			if _, ok := findingCounts[finding.Criterion]; !ok {
				findingCounts[finding.Criterion] = make(map[string]int)
			}
			findingCounts[finding.Criterion][finding.Severity]++
		}
	}

	// Build common findings
	totalFindings := 0
	for _, severityCounts := range findingCounts {
		for _, count := range severityCounts {
			totalFindings += count
		}
	}

	for criterion, severityCounts := range findingCounts {
		for severity, count := range severityCounts {
			pattern := core.FindingPattern{
				Criterion: criterion,
				Severity:  severity,
				Count:     count,
				Frequency: float64(count) / float64(totalFindings),
			}
			summary.CommonFindings = append(summary.CommonFindings, pattern)
		}
	}

	// Sort by frequency
	sort.Slice(summary.CommonFindings, func(i, j int) bool {
		return summary.CommonFindings[i].Frequency > summary.CommonFindings[j].Frequency
	})

	// Generate recommended rules based on common findings
	summary.RecommendedRules = generateRecommendedRules(summary.CommonFindings)

	return summary, nil
}

func (s *Store) loadAll() ([]core.LearningRecord, error) {
	path := filepath.Join(s.basePath, learningFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []core.LearningRecord{}, nil
		}
		return nil, err
	}

	var records []core.LearningRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("failed to parse learnings: %w", err)
	}

	return records, nil
}

func (s *Store) saveAll(records []core.LearningRecord) error {
	// Ensure directory exists
	if err := os.MkdirAll(s.basePath, 0755); err != nil {
		return err
	}

	path := filepath.Join(s.basePath, learningFileName)
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// generateRecommendedRules creates design rules based on common findings.
func generateRecommendedRules(findings []core.FindingPattern) []string {
	rules := []string{}

	for _, f := range findings {
		if f.Frequency < 0.1 {
			continue // Skip rare findings
		}

		switch f.Criterion {
		case "contrast":
			rules = append(rules, "Ensure all text meets WCAG AA contrast requirements (4.5:1 for normal text, 3:1 for large text)")
		case "spacing":
			rules = append(rules, "Use consistent spacing tokens throughout the design")
		case "touch":
			rules = append(rules, "Ensure all interactive elements have minimum 44x44px touch targets")
		case "overflow":
			rules = append(rules, "Test layouts at all breakpoints to prevent content overflow")
		case "hierarchy":
			rules = append(rules, "Use proper heading hierarchy (h1 -> h2 -> h3) without skipping levels")
		case "labels":
			rules = append(rules, "Provide clear, visible labels for all form inputs")
		}
	}

	return rules
}

// RecordDesignPattern saves a new design pattern to the store.
func (s *Store) RecordDesignPattern(pattern core.DesignPattern) error {
	records, err := s.loadAll()
	if err != nil {
		records = []core.LearningRecord{}
	}

	// Find or create a pattern record
	found := false
	for i, record := range records {
		for j, p := range record.PatternsLearned {
			if p.ID == pattern.ID {
				// Update existing pattern
				records[i].PatternsLearned[j] = pattern
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		// Add to first record or create new one
		if len(records) == 0 {
			records = append(records, core.LearningRecord{
				ProjectID: "system",
				Timestamp: time.Now(),
			})
		}
		records[0].PatternsLearned = append(records[0].PatternsLearned, pattern)
	}

	return s.saveAll(records)
}
