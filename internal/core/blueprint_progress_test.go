package core

import (
	"testing"
	"time"
)

func TestBlueprintProgressLifecycle(t *testing.T) {
	at := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	p := NewBlueprintProgress("blueprint-1", []BlueprintSection{
		{ID: "discovery", Name: "Penjelasan Sistem", Status: BPPending},
		{ID: "data", Name: "Arsitektur & Siklus Hidup Data", Status: BPPending},
		{ID: "tech", Name: "Teknologi", Status: BPPending},
	}, at)
	if p.Status != BPTaskRunning || p.Seq != 1 || p.Events[0].Event != EvTaskStarted {
		t.Fatalf("start: %+v", p)
	}
	p.StartSection("discovery", "Membaca kebutuhan sistem", at)
	p.CompleteSection("discovery", at)
	p.StartSection("data", "Menganalisis kepemilikan data", at)
	done, total := p.Counts()
	if done != 1 || total != 3 {
		t.Fatalf("counts %d/%d", done, total)
	}
	if got := p.section("data").Status; got != BPInProgress {
		t.Fatalf("data status %s", got)
	}
	// Output started is not completion.
	if p.section("data").Status == BPCompleted {
		t.Fatal("data completed before validation")
	}
	p.SetActivity("data", "Memvalidasi output Arsitektur & Siklus Hidup Data", at)
	p.CompleteSection("data", at)
	if p.section("data").Status != BPCompleted {
		t.Fatal("data not completed after validation")
	}
	p.StartSection("tech", "Merancang teknologi", at)
	p.FailSection("tech", "jawaban tidak valid", at)
	p.FailTask("jawaban tidak valid", at)
	if p.section("discovery").Status != BPCompleted || p.section("data").Status != BPCompleted {
		t.Fatal("failure dropped completed sections")
	}
	if p.section("tech").Status != BPFailed || p.Status != BPTaskFailed {
		t.Fatalf("fail: tech=%s task=%s", p.section("tech").Status, p.Status)
	}
	p.ResumeRunning()
	p.RetrySection("tech", "Memperbaiki output Teknologi", at)
	if p.Status != BPTaskRunning || p.section("tech").Status != BPRetrying {
		t.Fatalf("retry: task=%s tech=%s", p.Status, p.section("tech").Status)
	}
	if p.section("discovery").Status != BPCompleted {
		t.Fatal("retry dropped discovery")
	}
	p.CompleteSection("tech", at)
	p.CompleteTask(at)
	if p.Status != BPTaskCompleted {
		t.Fatalf("task %s", p.Status)
	}
	done, total = p.Counts()
	if done != 3 || total != 3 {
		t.Fatalf("final %d/%d", done, total)
	}
	seen := map[int64]bool{}
	for _, e := range p.Events {
		if seen[e.Seq] {
			t.Fatalf("duplicate seq %d", e.Seq)
		}
		seen[e.Seq] = true
		if e.TaskID != "blueprint-1" {
			t.Fatalf("taskId %s", e.TaskID)
		}
	}
	if len(p.After(p.Seq-1)) != 1 || len(p.After(p.Seq)) != 0 {
		t.Fatalf("After: last=%d n-1=%d n=%d", p.Seq, len(p.After(p.Seq-1)), len(p.After(p.Seq)))
	}
}

func TestBlueprintProgressSkipsDoNotCount(t *testing.T) {
	at := time.Now().UTC()
	p := NewBlueprintProgress("t", []BlueprintSection{
		{ID: "a", Name: "A", Status: BPPending},
		{ID: "b", Name: "B", Status: BPPending},
	}, at)
	p.CompleteSection("a", at)
	p.SkipSection("b", at)
	done, total := p.Counts()
	if done != 1 || total != 1 {
		t.Fatalf("counts %d/%d want 1/1", done, total)
	}
}

func TestBlueprintProgressDoesNotReopenCompleted(t *testing.T) {
	at := time.Now().UTC()
	p := NewBlueprintProgress("t", []BlueprintSection{{ID: "a", Name: "A", Status: BPPending}}, at)
	p.CompleteSection("a", at)
	p.StartSection("a", "again", at)
	p.FailSection("a", "x", at)
	p.RetrySection("a", "retry", at)
	if p.section("a").Status != BPCompleted {
		t.Fatalf("status %s", p.section("a").Status)
	}
}
