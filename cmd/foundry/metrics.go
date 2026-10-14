package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/work"
)

// runMetrics prints token-efficiency numbers of a project without changing it:
// prompt size before/after context scoping (exact chars) and the TaskMetrics
// recorded during builds (provider-reported vs estimated, kept apart).
func runMetrics(args []string) {
	fs := flag.NewFlagSet("metrics", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "keluaran JSON untuk perbandingan benchmark")
	fs.Parse(args)
	dir := fs.Arg(0)
	if dir == "" {
		dir = "."
	}
	dir, _ = filepath.Abs(dir)
	raw, err := os.ReadFile(filepath.Join(dir, core.StateDir, "project.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "baca project:", err)
		os.Exit(1)
	}
	var p core.Project
	if err := json.Unmarshal(raw, &p); err != nil {
		fmt.Fprintln(os.Stderr, "project.json tidak valid:", err)
		os.Exit(1)
	}
	p.Dir = dir
	if *asJSON {
		printJSON(&p)
		return
	}
	printMetrics(&p)
}

func printMetrics(p *core.Project) {
	var base, cur, mem, memSrc, memTasks int
	sizes := work.MeasurePrompts(p)
	for _, s := range sizes {
		base += s.Baseline
		cur += s.Current
		mem += s.Memory
		memSrc += s.MemorySource
		if s.Memory > 0 {
			memTasks++
		}
	}
	fmt.Printf("project: %s  tasks: %d\n", p.Name, len(sizes))
	fmt.Println("\nPROMPT (karakter persis, bukan token)")
	fmt.Printf("  baseline (blueprint + aturan penuh): %d\n  sekarang (blueprint + aturan sesuai task): %d\n", base, cur)
	if base > 0 {
		fmt.Printf("  selisih: %d (%.1f%%)\n", base-cur, 100*float64(base-cur)/float64(base))
	}
	fmt.Printf("  task memory: %d task, %d karakter di prompt, mewakili %d byte source (0 = tidak dipakai)\n", memTasks, mem, memSrc)
	fmt.Println("\nRUN AGENT per build (semua run: eksekusi, perbaikan, review, retry)")
	if p.BuildMetrics == nil || p.BuildMetrics.Runs == 0 {
		fmt.Println("  belum ada: build project ini belum berjalan dengan pencatatan metrik")
		return
	}
	printRuns("  build", p.BuildMetrics)
	fmt.Println("\nPER TASK")
	for _, t := range p.Tasks {
		if t.Metrics != nil {
			printRuns(fmt.Sprintf("  %s (percobaan %d)", t.ID, t.Attempts), t.Metrics)
		}
	}
}

// printRuns keeps measured, provider-reported and estimated numbers apart.
func printRuns(label string, m *core.TaskMetrics) {
	fmt.Printf("%s: run %d (gagal %d), %.1fs, prompt %d + output %d karakter\n", label, m.Runs, m.Failures, float64(m.Millis)/1000, m.PromptChars, m.OutputChars)
	if m.UsageRuns > 0 {
		fmt.Printf("    AKTUAL (%d/%d run, dilaporkan CLI): input %d, cache-write %d, cache-read %d, output %d, biaya $%.4f\n",
			m.UsageRuns, m.Runs, m.InputTokens, m.CacheWriteTokens, m.CachedTokens, m.OutputTokens, m.CostUSD)
	}
	if rest := m.Runs - m.UsageRuns; rest > 0 {
		fmt.Printf("    ESTIMASI (%d run tanpa data provider, karakter/4): %d token\n", rest, m.EstTokens)
	}
}

// printJSON is the same numbers in a stable shape, so two runs (old and new
// version, same project/model/config) can be diffed with jq.
func printJSON(p *core.Project) {
	var base, cur int
	for _, s := range work.MeasurePrompts(p) {
		base += s.Baseline
		cur += s.Current
	}
	tasks := map[string]*core.TaskMetrics{}
	for _, t := range p.Tasks {
		if t.Metrics != nil {
			tasks[t.ID] = t.Metrics
		}
	}
	out := map[string]any{
		"project":      p.Name,
		"tasks":        len(p.Tasks),
		"prompt_chars": map[string]int{"baseline": base, "current": cur},
		"build":        p.BuildMetrics,
		"per_task":     tasks,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}
