package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/standard"
	"github.com/handrawanw/foundry/internal/modules/workflow"
)

// ExportDir is where finalize writes the export package, inside the
// project folder. Foundry owns this folder; nothing else is written into
// the project.
const ExportDir = "docs/foundry"

// Format names the export package layout for the tools that read it.
const Format = "foundry-blueprint/1"

// Artifacts is the blueprint: the built-in artifacts rendered from the
// structured project data (only those with content), then the artifacts
// written for this project.
func Artifacts(p *core.Project) []core.Artifact {
	var out []core.Artifact
	add := func(id, title, category, summary, body string) {
		if strings.TrimSpace(body) == "" {
			return
		}
		out = append(out, core.Artifact{ID: id, Title: title, Category: category, Summary: summary, Body: body, Builtin: true})
	}
	add("requirements", "Requirements", core.ArtifactRequirements,
		"Masalah, pengguna, kebutuhan, fitur, dan alur.", requirementsMarkdown(p))
	arch := strings.TrimPrefix(architectureMarkdown(p), "## Architecture\n\n")
	add("architecture", "Arsitektur", core.ArtifactArchitecture,
		"Pola arsitektur, modul, data, keamanan, dan aturan teknis.", strings.ReplaceAll("\n"+arch, "\n### ", "\n## ")[1:])
	add("technology", "Teknologi & Struktur", core.ArtifactPlatform,
		"Keputusan teknologi dan struktur project.", techMarkdown(p))
	add("ui-design", "Desain UI/UX", core.ArtifactUI,
		"Halaman, design system, dan spesifikasi per halaman dari Design IR.", designMarkdown(p))
	return append(out, p.Artifacts...)
}

// BlueprintHash identifies the whole blueprint: the same artifacts give
// the same hash.
func BlueprintHash(docs []core.Artifact) string {
	h := sha256.New()
	for _, d := range docs {
		fmt.Fprintf(h, "%s=%s\n", d.ID, d.Hash())
	}
	return hex.EncodeToString(h.Sum(nil)[:8])
}

func artifactFile(id string) string { return "blueprint/" + id + ".md" }

// safeName holds for ids that are safe as file names (ids are normalized
// on input; this guards the file system anyway).
func safeName(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
func commandFile(id string) string { return "workflow/" + id + ".md" }

// ArtifactMarkdown is one artifact as a standalone document.
func ArtifactMarkdown(a core.Artifact) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", a.Title)
	if a.Summary != "" {
		fmt.Fprintf(&b, "> %s\n\n", a.Summary)
	}
	b.WriteString(strings.TrimSpace(a.Body))
	b.WriteString("\n")
	if len(a.Refs) > 0 {
		b.WriteString("\n## Terkait\n\n")
		for _, r := range a.Refs {
			fmt.Fprintf(&b, "- [%s](%s.md)\n", r, r)
		}
	}
	return b.String()
}

// CommandMarkdown is one command as a standalone instruction. waves places
// it in the order; nil leaves the order out.
func CommandMarkdown(c core.Command, waves [][]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s — %s\n\n", c.ID, c.Name)
	meta := []string{"Status: " + c.Status}
	for i, w := range waves {
		if slices.Contains(w, c.ID) {
			meta = append(meta, fmt.Sprintf("Gelombang %d", i+1))
		}
	}
	if len(c.DependsOn) > 0 {
		meta = append(meta, "Bergantung pada: "+strings.Join(c.DependsOn, ", "))
	} else {
		meta = append(meta, "Tanpa dependency")
	}
	b.WriteString(strings.Join(meta, " · ") + "\n\n")
	section := func(title, text string) {
		if text = strings.TrimSpace(text); text != "" {
			fmt.Fprintf(&b, "## %s\n\n%s\n\n", title, text)
		}
	}
	list := func(title, mark string, items []string) {
		if len(items) == 0 {
			return
		}
		fmt.Fprintf(&b, "## %s\n\n", title)
		for _, it := range items {
			fmt.Fprintf(&b, "- %s%s\n", mark, it)
		}
		b.WriteString("\n")
	}
	section("Tujuan", c.Objective)
	section("Konteks", c.Context)
	if len(c.Artifacts) > 0 {
		b.WriteString("## Acuan blueprint\n\n")
		for _, r := range c.Artifacts {
			fmt.Fprintf(&b, "- [%s](../%s)\n", r.ID, artifactFile(r.ID))
		}
		b.WriteString("\n")
	}
	list("Input", "", c.Inputs)
	section("Instruksi", c.Instructions)
	list("Output yang diharapkan", "", c.Outputs)
	list("Acceptance criteria", "[ ] ", c.Acceptance)
	list("Batasan", "", c.Constraints)
	return b.String()
}

// Manifest describes the export package for tools.
type Manifest struct {
	Format          string             `json:"format"`
	Project         string             `json:"project"`
	Status          string             `json:"status"` // draft | finalized
	FinalizedAt     string             `json:"finalized_at,omitempty"`
	BlueprintHash   string             `json:"blueprint_hash"`
	WorkflowVersion int                `json:"workflow_version"`
	Entry           string             `json:"entry"`
	Artifacts       []ManifestArtifact `json:"artifacts"`
	Commands        []ManifestCommand  `json:"commands"`
	Waves           [][]string         `json:"waves"`
}

type ManifestArtifact struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	File     string `json:"file"`
	Hash     string `json:"hash"`
}

type ManifestCommand struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	File      string   `json:"file"`
	DependsOn []string `json:"depends_on"`
	Artifacts []string `json:"artifacts"`
	Wave      int      `json:"wave"`
}

// BuildManifest indexes the package of p.
func BuildManifest(p *core.Project) Manifest {
	docs := Artifacts(p)
	m := Manifest{Format: Format, Project: p.Name, Status: "draft", BlueprintHash: BlueprintHash(docs), Entry: "README.md",
		Artifacts: []ManifestArtifact{}, Commands: []ManifestCommand{}, Waves: [][]string{}}
	if p.Finalized() {
		m.Status = "finalized"
	}
	for _, d := range docs {
		m.Artifacts = append(m.Artifacts, ManifestArtifact{ID: d.ID, Title: d.Title, Category: d.Category, File: artifactFile(d.ID), Hash: d.Hash()})
	}
	w := p.Workflow
	if w == nil {
		return m
	}
	m.WorkflowVersion = w.Version
	if w.FinalizedAt != nil && p.Finalized() {
		m.FinalizedAt = w.FinalizedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	if waves := workflow.Waves(w.Commands); waves != nil {
		m.Waves = waves
	}
	for _, c := range w.Commands {
		mc := ManifestCommand{ID: c.ID, Name: c.Name, File: commandFile(c.ID), DependsOn: c.DependsOn, Artifacts: []string{}}
		if mc.DependsOn == nil {
			mc.DependsOn = []string{}
		}
		for _, r := range c.Artifacts {
			mc.Artifacts = append(mc.Artifacts, r.ID)
		}
		for i, wave := range m.Waves {
			if slices.Contains(wave, c.ID) {
				mc.Wave = i + 1
			}
		}
		m.Commands = append(m.Commands, mc)
	}
	return m
}

// Files is the export package: relative path → content. The same set is
// written into ExportDir on finalize and downloaded as a zip.
func Files(p *core.Project) map[string]string {
	m := BuildManifest(p)
	docs := Artifacts(p)
	out := map[string]string{"README.md": packageReadme(p, m, docs)}
	if b, err := json.MarshalIndent(m, "", "  "); err == nil {
		out["manifest.json"] = string(b) + "\n"
	}
	for _, d := range docs {
		if safeName(d.ID) {
			out[artifactFile(d.ID)] = ArtifactMarkdown(d)
		}
	}
	if w := p.Workflow; w != nil {
		for _, c := range w.Commands {
			if safeName(c.ID) {
				out[commandFile(c.ID)] = CommandMarkdown(c, m.Waves)
			}
		}
		if b, err := json.MarshalIndent(struct {
			Version  int            `json:"version"`
			Status   string         `json:"status"`
			Waves    [][]string     `json:"waves"`
			Commands []core.Command `json:"commands"`
		}{w.Version, m.Status, m.Waves, w.Commands}, "", "  "); err == nil {
			out["workflow/workflow.json"] = string(b) + "\n"
		}
	}
	if b, err := json.MarshalIndent(Export(p), "", "  "); err == nil {
		out["spec.json"] = string(b) + "\n"
	}
	if p.Stack != nil && len(p.Stack.Decisions) > 0 {
		out["adr/0001-stack.md"] = adrStack(p)
	}
	if p.HasUI() && p.Design != nil && len(p.Design.Pages) > 0 {
		out["design-brief.md"] = standard.DesignBrief(p)
	}
	return out
}

// packageReadme tells an external coding agent how to read the package.
func packageReadme(p *core.Project, m Manifest, docs []core.Artifact) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s — Blueprint & Command Workflow\n\n", p.Name)
	fmt.Fprintf(&b, "Status: **%s** · Blueprint `%s` · Workflow v%d\n\n", m.Status, m.BlueprintHash, m.WorkflowVersion)
	if p.Story != "" {
		fmt.Fprintf(&b, "%s\n\n", strings.TrimSpace(p.Story))
	}
	b.WriteString(standard.PackageGuide)
	b.WriteString("\n## Blueprint\n\n| Artefak | Kategori | File |\n|---|---|---|\n")
	for _, d := range docs {
		fmt.Fprintf(&b, "| %s | %s | [%s](%s) |\n", cell(d.Title), d.Category, artifactFile(d.ID), artifactFile(d.ID))
	}
	b.WriteString("\n## Urutan pengerjaan\n\n")
	if len(m.Commands) == 0 {
		b.WriteString("Belum ada command.\n")
		return b.String()
	}
	names := map[string]string{}
	for _, c := range m.Commands {
		names[c.ID] = c.Name
	}
	for i, wave := range m.Waves {
		fmt.Fprintf(&b, "**Gelombang %d**", i+1)
		if len(wave) > 1 {
			b.WriteString(" (boleh paralel)")
		}
		b.WriteString("\n\n")
		for _, id := range wave {
			fmt.Fprintf(&b, "- [%s — %s](%s)\n", id, names[id], commandFile(id))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// WriteDocs writes the export package into ExportDir, replacing the
// package files of an earlier finalize. Only files Foundry wrote there
// (.md and .json) are removed; AGENTS.md is added at the project root when
// the project has none.
func WriteDocs(dir string, p *core.Project) error {
	if dir == "" || p == nil {
		return nil
	}
	root := filepath.Join(dir, filepath.FromSlash(ExportDir))
	files := Files(p)
	for _, sub := range []string{"blueprint", "workflow", "adr"} {
		entries, _ := os.ReadDir(filepath.Join(root, sub))
		for _, e := range entries {
			name := sub + "/" + e.Name()
			if e.Type().IsRegular() && (strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".json")) && files[name] == "" {
				if err := os.Remove(filepath.Join(root, filepath.FromSlash(name))); err != nil {
					return err
				}
			}
		}
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
	}
	agents := filepath.Join(dir, "AGENTS.md")
	if _, err := os.Stat(agents); os.IsNotExist(err) {
		return os.WriteFile(agents, []byte(standard.AgentsMD), 0o644)
	}
	return nil
}
