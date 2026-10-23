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

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/standard"
	"github.com/handrawanwdev/foundry/internal/modules/workflow"
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
	add(core.ArtifactSurvey, "Keputusan Teknis Survey", core.ArtifactRequirements,
		"Platform, pengguna, target delivery, arsitektur, prioritas kualitas, dan batasan wajib beserta sumbernya.", surveyMarkdown(p))
	arch := strings.TrimPrefix(architectureMarkdown(p), "## Architecture\n\n")
	add("architecture", "Arsitektur", core.ArtifactArchitecture,
		"Pola arsitektur, modul, data, keamanan, dan aturan teknis.", strings.ReplaceAll("\n"+arch, "\n### ", "\n## ")[1:])
	add("technology", "Teknologi & Struktur", core.ArtifactPlatform,
		"Keputusan teknologi dan struktur project.", techMarkdown(p))
	add(core.ArtifactDesignRules, "Aturan Desain", core.ArtifactUI,
		"Prinsip, panduan UX, design tokens, komponen, aksesibilitas, responsif, interaksi, dan aturan platform.", designRulesMarkdown(p))
	add(core.ArtifactLayoutSpec, "Spesifikasi Layout", core.ArtifactUI,
		"Shell, halaman, section, hierarki, grid, responsif, navigasi, dan state UI per halaman.", layoutMarkdown(p))
	add(core.ArtifactUseCases, "Alur Bisnis & Use Case", core.ArtifactDomain,
		"Aktor, tujuan, kondisi, alur utama, alternatif, kegagalan, aturan bisnis, otorisasi, efek data dan integrasi.", useCasesMarkdown(p))
	add(core.ArtifactAcceptance, "Acceptance Criteria & Definition of Done", core.ArtifactQuality,
		"Kriteria penerimaan per requirement, bukti yang diharapkan, dan Definition of Done.", acceptanceMarkdown(p))
	add(core.ArtifactDecisions, "Keputusan, Asumsi & Risiko", core.ArtifactOther,
		"Area yang tidak dicakup, asumsi, pertanyaan terbuka, konflik, dan risiko.", decisionsMarkdown(p))
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
func ArtifactMarkdown(a core.Artifact) string { return artifactMarkdown(a, "") }

func artifactMarkdown(a core.Artifact, binding string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", a.Title)
	if a.Summary != "" {
		fmt.Fprintf(&b, "> %s\n\n", a.Summary)
	}
	if binding != "" && a.ID != core.ArtifactDecisions {
		b.WriteString(bindBanner(binding, "decisions.md"))
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

// bindBanner inlines decided decisions into a document so an agent that
// reads only this file still follows them.
func bindBanner(binding, href string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "> **Keputusan mengikat** menimpa isi di bawah bila bertentangan. Sumber: [decisions](%s).\n>\n", href)
	for _, line := range strings.Split(strings.TrimSuffix(binding, "\n"), "\n") {
		b.WriteString("> " + line + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

// CommandMarkdown is one command as a standalone instruction. waves places
// it in the order; nil leaves the order out.
func CommandMarkdown(c core.Command, waves [][]string) string {
	return commandMarkdown(c, waves, "")
}

func commandMarkdown(c core.Command, waves [][]string, binding string) string {
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
	if binding != "" {
		b.WriteString(bindBanner(binding, "../blueprint/decisions.md"))
	}
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
	if len(c.Covers) > 0 {
		fmt.Fprintf(&b, "## Mencakup requirement\n\n%s — kriteria penerimaannya di [acceptance](../%s).\n\n", strings.Join(c.Covers, ", "), artifactFile(core.ArtifactAcceptance))
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
	// Areas is how far the blueprint covers each area.
	Areas    []core.AreaStatus `json:"areas"`
	Commands []ManifestCommand `json:"commands"`
	Waves    [][]string        `json:"waves"`
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
	Covers    []string `json:"covers"`
	Wave      int      `json:"wave"`
}

// BuildManifest indexes the package of p.
func BuildManifest(p *core.Project) Manifest {
	docs := Artifacts(p)
	m := Manifest{Format: Format, Project: p.Name, Status: "draft", BlueprintHash: BlueprintHash(docs), Entry: "README.md",
		Artifacts: []ManifestArtifact{}, Areas: core.Coverage(p, docs), Commands: []ManifestCommand{}, Waves: [][]string{}}
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
		mc := ManifestCommand{ID: c.ID, Name: c.Name, File: commandFile(c.ID), DependsOn: c.DependsOn, Artifacts: []string{}, Covers: c.Covers}
		if mc.Covers == nil {
			mc.Covers = []string{}
		}
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
	binding := core.BindingText(p.Decisions)
	out := map[string]string{"README.md": packageReadme(p, m, docs)}
	if b, err := json.MarshalIndent(m, "", "  "); err == nil {
		out["manifest.json"] = string(b) + "\n"
	}
	for _, d := range docs {
		if safeName(d.ID) {
			out[artifactFile(d.ID)] = artifactMarkdown(d, binding)
		}
	}
	if w := p.Workflow; w != nil {
		for _, c := range w.Commands {
			if safeName(c.ID) {
				out[commandFile(c.ID)] = commandMarkdown(c, m.Waves, binding)
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
	if len(p.Requirements) > 0 {
		out["traceability.md"] = traceabilityMarkdown(p)
	}
	if d := p.UI; d != nil {
		if d.Rules != nil {
			out[designFile(core.ArtifactDesignRules)] = jsonFile(d.Rules)
		}
		if d.Layout != nil && len(d.Layout.Pages) > 0 {
			out[designFile(core.ArtifactLayoutSpec)] = jsonFile(struct {
				*core.LayoutSpec
				Flows []core.Flow `json:"flows,omitempty"`
			}{d.Layout, pageFlowList(p.Flows)})
		}
	}
	return out
}

func designFile(id string) string { return "design/" + id + ".json" }

func jsonFile(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}
	return string(b) + "\n"
}

// pageFlowList keeps the flows that pass through pages.
func pageFlowList(flows []core.Flow) []core.Flow {
	var out []core.Flow
	for _, f := range flows {
		if len(f.PageIDs) > 0 {
			out = append(out, f)
		}
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
	if bind := core.BindingText(p.Decisions); bind != "" {
		b.WriteString("## Keputusan mengikat\n\n")
		b.WriteString("Keputusan di bawah menimpa artefak dan command lain bila isinya bertentangan. Baca ini sebelum artefak mana pun.\n\n")
		b.WriteString(bind)
		b.WriteString("\n")
	}
	b.WriteString(standard.PackageGuide)
	b.WriteString("\n## Blueprint\n\n| Artefak | Kategori | File |\n|---|---|---|\n")
	for _, d := range docs {
		fmt.Fprintf(&b, "| %s | %s | [%s](%s) |\n", cell(d.Title), d.Category, artifactFile(d.ID), artifactFile(d.ID))
	}
	b.WriteString("\n## Cakupan blueprint\n\n| Area | Status | Keterangan |\n|---|---|---|\n")
	for _, a := range m.Areas {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", a.Title, areaLabel[a.Status], cell(areaNote(a)))
	}
	b.WriteString("\nKeterlacakan requirement ke command: [traceability.md](traceability.md).\n")
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

var areaLabel = map[string]string{
	core.AreaCovered: "dicakup", core.AreaInvalid: "belum valid", core.AreaMissing: "belum ditulis",
	core.AreaSkipped: "dilewati", core.AreaNotApplicable: "tidak relevan",
}

func areaNote(a core.AreaStatus) string {
	switch {
	case a.Reason != "":
		return a.Reason
	case a.Errors > 0:
		return fmt.Sprintf("%d hal wajib diperbaiki", a.Errors)
	case len(a.Artifacts) > 0:
		return strings.Join(a.Artifacts, ", ")
	}
	return "-"
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
	// design-brief.md was written by older versions; the design rules
	// replace it.
	if err := os.Remove(filepath.Join(root, "design-brief.md")); err != nil && !os.IsNotExist(err) {
		return err
	}
	if files["traceability.md"] == "" {
		if err := os.Remove(filepath.Join(root, "traceability.md")); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	for _, sub := range []string{"blueprint", "workflow", "adr", "design"} {
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
