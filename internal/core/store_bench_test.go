package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func largeProject() *Project {
	p := &Project{
		Version: 42,
		Name:    "RBAC Management System",
		Stage:   StageDesign,
		Story:   "Sistem manajemen peran dan akses untuk perusahaan.",
		Discovery: &Discovery{
			Summary:  "RBAC untuk admin dan auditor.",
			Business: "Kontrol akses terpusat.",
			Roles:    []Role{{Name: "Admin", Description: "mengelola peran", Platform: "web"}},
		},
		UpdatedAt: time.Now().UTC(),
	}
	body := "Langkah implementasi yang panjang untuk command ini, termasuk konteks artefak, kriteria penerimaan, dan batasan teknis.\n"
	for i := 0; i < 80; i++ {
		id := "REQ-" + itoa3(i+1)
		p.Requirements = append(p.Requirements, Requirement{
			ID: id, Text: "Kebutuhan " + id + " tentang otorisasi dan jejak audit.", Priority: "MUST",
			Acceptance: []Criterion{{ID: id + "-AC1", Kind: "functional", Text: "Sistem menolak akses tanpa izin.", Evidence: "test"}},
		})
	}
	for i := 0; i < 40; i++ {
		id := "FEAT-" + itoa3(i+1)
		p.Features = append(p.Features, Feature{
			ID: id, Name: "Fitur " + id, Description: "Deskripsi fitur " + id,
			RequirementIDs: []string{"REQ-" + itoa3(i+1), "REQ-" + itoa3(i+2)},
		})
	}
	for i := 0; i < 30; i++ {
		id := "FLOW-" + itoa3(i+1)
		p.Flows = append(p.Flows, Flow{
			ID: id, Name: "Alur " + id, FeatureID: "FEAT-" + itoa3(i%40+1), Role: "Admin",
			Goal: "Menyelesaikan " + id, Steps: []string{"buka", "isi", "simpan"},
			Post: []string{"data tersimpan"}, Permissions: []string{"role.write"},
		})
	}
	cmds := make([]Command, 50)
	for i := range cmds {
		id := "CMD-" + itoa3(i+1)
		cmds[i] = Command{
			ID: id, Name: "Kerjakan " + id, Objective: "Implementasikan " + id,
			Instructions: body + body + body, Status: CommandDraft,
			Covers:    []string{"REQ-" + itoa3(i%80+1)},
			Artifacts: []ArtifactRef{{ID: "requirements", Hash: "abc123"}},
		}
		if i > 0 {
			cmds[i].DependsOn = []string{"CMD-" + itoa3(i)}
		}
	}
	p.Workflow = &Workflow{Status: "draft", Version: 3, Commands: cmds}
	for i := 0; i < 20; i++ {
		p.Artifacts = append(p.Artifacts, Artifact{
			ID: "extra-" + itoa3(i+1), Title: "Dokumen " + itoa3(i+1), Category: ArtifactOther,
			Body: body + body + body + body, Version: 1,
		})
	}
	for i := 0; i < 300; i++ {
		p.Log("system", "event "+itoa3(i))
	}
	return p
}

func itoa3(n int) string {
	s := itoaBench(n)
	for len(s) < 3 {
		s = "0" + s
	}
	return s
}

func itoaBench(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func writeLargeProject(tb testing.TB) (string, *Store) {
	tb.Helper()
	dir := tb.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		tb.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, StateDir), 0o755); err != nil {
		tb.Fatal(err)
	}
	p := largeProject()
	p.Dir = dir
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, StateDir, "project.json"), b, 0o644); err != nil {
		tb.Fatal(err)
	}
	return dir, s
}

func BenchmarkPeekName(b *testing.B) {
	dir, _ := writeLargeProject(b)
	st, err := os.Stat(filepath.Join(dir, StateDir, "project.json"))
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(st.Size())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if got := PeekName(dir); got != "RBAC Management System" {
			b.Fatalf("name = %q", got)
		}
	}
}

func BenchmarkStoreLoad(b *testing.B) {
	_, s := writeLargeProject(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p, err := s.Load()
		if err != nil || p.Name != "RBAC Management System" {
			b.Fatalf("load: %v name=%q", err, p.Name)
		}
	}
}

func BenchmarkPeekNameN20(b *testing.B) {
	dirs := make([]string, 20)
	for i := range dirs {
		dirs[i], _ = writeLargeProject(b)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, dir := range dirs {
			_ = PeekName(dir)
		}
	}
}
