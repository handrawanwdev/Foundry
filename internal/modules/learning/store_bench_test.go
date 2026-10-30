package learning

import (
	"fmt"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
)

func BenchmarkLearningReads(b *testing.B) {
	dir := b.TempDir()
	s := NewStore(dir)
	for i := 0; i < 200; i++ {
		findings := []core.VisualFinding{
			{Page: "p", Criterion: "contrast", Severity: "high", Message: "low contrast"},
			{Page: "p", Criterion: "spacing", Severity: "medium", Message: "gap"},
			{Page: "p", Criterion: "touch", Severity: "low", Message: "small target"},
		}
		if err := s.Record(fmt.Sprintf("p%d", i), "dashboard", findings); err != nil {
			b.Fatal(err)
		}
	}
	_ = s.RecordDesignPattern(core.DesignPattern{
		ID: "pattern1", Name: "Dashboard Layout", Description: "grid", ProductType: "dashboard",
		SuccessRate: 0.9, UsageCount: 10,
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.GetPatterns("dashboard"); err != nil {
			b.Fatal(err)
		}
		if _, err := s.GetSummary(); err != nil {
			b.Fatal(err)
		}
	}
}
