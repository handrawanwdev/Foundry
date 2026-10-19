package work

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/handrawanw/foundry/internal/core"
)

// Version layer: every design state that lands (a generated design, a
// revision, a canvas edit, a theme change, ...) is snapshotted, so versions
// can be compared and an earlier one restored. The snapshot is the same
// Design IR the renderer and the final spec read; nothing is re-generated.

// MaxDesignVersions caps the version file; the oldest versions go first.
const MaxDesignVersions = 50

// VersionSummary is a version without its pages, for listing.
type VersionSummary struct {
	ID      int       `json:"id"`
	Source  string    `json:"source"`
	Note    string    `json:"note,omitempty"`
	Quality int       `json:"quality,omitempty"`
	Pages   int       `json:"pages"`
	Theme   string    `json:"theme,omitempty"`
	At      time.Time `json:"at"`
	Current bool      `json:"current,omitempty"` // the design as it is now
}

// VersionComparison is what changed from one version to another.
type VersionComparison struct {
	From     int                 `json:"from"`
	To       int                 `json:"to"` // 0 = the current design
	Changes  []core.DesignChange `json:"changes"`
	System   bool                `json:"system_changed,omitempty"`
	Theme    [2]string           `json:"theme,omitempty"` // before, after; empty when unchanged
	Template [2]string           `json:"template,omitempty"`
	Flows    []string            `json:"flows_changed,omitempty"`
}

func versionsPath(dir string) string {
	return filepath.Join(dir, core.StateDir, "design-versions.json")
}

// LoadVersions reads the saved versions, oldest first.
func LoadVersions(dir string) ([]core.DesignVersion, error) {
	b, err := os.ReadFile(versionsPath(dir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []core.DesignVersion
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("riwayat versi rusak: %w", err)
	}
	return out, nil
}

func saveVersions(dir string, vs []core.DesignVersion) error {
	b, err := json.MarshalIndent(vs, "", "  ")
	if err != nil {
		return err
	}
	path := versionsPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// snapshot is the current design as a version (no id yet).
func snapshot(p *core.Project) core.DesignVersion {
	d := p.Design
	v := core.DesignVersion{Pages: d.Pages, System: d.System, Theme: d.Theme, Template: d.Template, Flows: map[string][]string{}}
	for _, fl := range p.Flows {
		v.Flows[fl.ID] = fl.PageIDs
	}
	if d.Quality != nil {
		v.Quality = d.Quality.Overall
	}
	v.Hash = versionHash(v)
	return v
}

// versionHash identifies the design content; the same design gives the
// same hash whatever its source.
func versionHash(v core.DesignVersion) string {
	b, _ := json.Marshal(struct {
		Pages    []core.Page
		System   *core.DesignSystem
		Theme    string
		Flows    map[string][]string
		Template string `json:",omitempty"`
	}{v.Pages, v.System, themeID(v.Theme), v.Flows, templateID(v.Template)})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

func templateID(t *core.TemplateChoice) string {
	if t == nil {
		return ""
	}
	return t.ID
}

func templateName(t *core.TemplateChoice) string {
	if t == nil {
		return ""
	}
	return t.Name
}

func themeID(t *core.ThemeContext) string {
	if t == nil {
		return ""
	}
	return t.ID
}

// RecordVersion snapshots the design when it differs from the last saved
// version. It returns the new version, or nil when nothing changed or there
// is no design yet.
func RecordVersion(p *core.Project, source, note string) (*core.DesignVersion, error) {
	if p.Design == nil || len(p.Design.Pages) == 0 {
		return nil, nil
	}
	vs, err := LoadVersions(p.Dir)
	if err != nil {
		return nil, err
	}
	v := snapshot(p)
	if n := len(vs); n > 0 && vs[n-1].Hash == v.Hash {
		return nil, nil
	}
	v.ID, v.Source, v.Note, v.At = 1, source, note, time.Now().UTC()
	if n := len(vs); n > 0 {
		v.ID = vs[n-1].ID + 1
	}
	vs = append(vs, v)
	if len(vs) > MaxDesignVersions {
		vs = vs[len(vs)-MaxDesignVersions:]
	}
	return &v, saveVersions(p.Dir, vs)
}

// Versions lists the saved versions, newest first; the one matching the
// current design is marked.
func Versions(p *core.Project) ([]VersionSummary, error) {
	vs, err := LoadVersions(p.Dir)
	if err != nil {
		return nil, err
	}
	cur := ""
	if p.Design != nil && len(p.Design.Pages) > 0 {
		cur = snapshot(p).Hash
	}
	out := make([]VersionSummary, 0, len(vs))
	for i := len(vs) - 1; i >= 0; i-- {
		v := vs[i]
		out = append(out, VersionSummary{ID: v.ID, Source: v.Source, Note: v.Note, Quality: v.Quality, Pages: len(v.Pages),
			Theme: themeName(v.Theme), At: v.At, Current: v.Hash == cur})
	}
	return out, nil
}

func themeName(t *core.ThemeContext) string {
	if t == nil {
		return ""
	}
	return t.Name
}

// FindVersion returns a saved version; id 0 is the current design.
func FindVersion(p *core.Project, id int) (*core.DesignVersion, error) {
	if id == 0 {
		if p.Design == nil || len(p.Design.Pages) == 0 {
			return nil, fmt.Errorf("belum ada rancangan")
		}
		v := snapshot(p)
		return &v, nil
	}
	vs, err := LoadVersions(p.Dir)
	if err != nil {
		return nil, err
	}
	for i := range vs {
		if vs[i].ID == id {
			return &vs[i], nil
		}
	}
	return nil, fmt.Errorf("versi %d tidak ada", id)
}

// CompareVersions lists what changed from a to b.
func CompareVersions(a, b core.DesignVersion) VersionComparison {
	out := VersionComparison{From: a.ID, To: b.ID, Changes: DiffPages(a.Pages, b.Pages)}
	sa, _ := json.Marshal(a.System)
	sb, _ := json.Marshal(b.System)
	out.System = string(sa) != string(sb)
	if themeID(a.Theme) != themeID(b.Theme) {
		out.Theme = [2]string{themeName(a.Theme), themeName(b.Theme)}
	}
	if templateID(a.Template) != templateID(b.Template) {
		out.Template = [2]string{templateName(a.Template), templateName(b.Template)}
	}
	for id, ids := range b.Flows {
		if fmt.Sprint(a.Flows[id]) != fmt.Sprint(ids) {
			out.Flows = append(out.Flows, id)
		}
	}
	for id := range a.Flows {
		if _, ok := b.Flows[id]; !ok {
			out.Flows = append(out.Flows, id)
		}
	}
	sort.Strings(out.Flows)
	return out
}

// RestoreVersion makes a saved version the current design. The version's
// Design IR is validated again; a pending revision must be kept or undone
// first. The design returns to draft.
func RestoreVersion(p *core.Project, v core.DesignVersion) error {
	d := p.Design
	if d == nil {
		return fmt.Errorf("belum ada rancangan")
	}
	if d.Revision != nil {
		return fmt.Errorf("simpan atau batalkan perubahan rancangan dulu")
	}
	pages := clonePages(v.Pages)
	core.EnsureIR(pages, p.Name)
	if err := core.ValidateDesign(pages, pages); err != nil {
		return err
	}
	d.Pages, d.System, d.Theme, d.Template = pages, v.System, v.Theme, v.Template
	d.Status, d.Screens, d.Scope, d.Rejected, d.QA = core.DesignDraft, nil, nil, "", nil
	for i := range p.Flows {
		p.Flows[i].PageIDs = v.Flows[p.Flows[i].ID]
	}
	// The picked theme follows the version, so a later generation keeps it.
	p.Theme = ""
	if v.Theme != nil && v.Theme.Source == "user" {
		p.Theme = v.Theme.ID
	}
	return nil
}

// clonePages deep-copies pages so a restored version shares nothing with
// the saved one.
func clonePages(in []core.Page) []core.Page {
	raw, _ := json.Marshal(in)
	var out []core.Page
	_ = json.Unmarshal(raw, &out)
	return out
}
