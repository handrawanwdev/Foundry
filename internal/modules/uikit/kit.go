// Package uikit is Foundry's curated UI kit: token presets (a complete,
// tested set of design tokens per kind of product) and page pattern
// templates (the regions — slots — a page of that pattern is made of). The
// AI picks a preset and fills slots instead of inventing values and
// layouts; Foundry materializes the tokens, renders every page as an HTML
// mockup from the templates, and exports the mockups as the reference the
// coding agent builds from.
//
// The kit is data, not code: the embedded files under data/ can be
// extended or replaced per user in $FOUNDRY_HOME/uikit/{presets,patterns}
// (same file format; a file with the same id wins).
package uikit

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/handrawanwdev/foundry/internal/config"
	"github.com/handrawanwdev/foundry/internal/core"
)

//go:embed data
var dataFS embed.FS

// DefaultPreset is used when a design names no preset.
const DefaultPreset = "saas-neutral"

// Preset is a complete token set for one kind of product.
type Preset struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Fit        string            `json:"fit"`
	Direction  string            `json:"direction"`
	Density    string            `json:"density"`
	Principles []string          `json:"principles,omitempty"`
	Tokens     core.DesignTokens `json:"tokens"`
}

// Slot is one region of a pattern template.
type Slot struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"` // renderer block: table, form, metrics, ...
	Guide    string `json:"guide"`
	Required bool   `json:"required,omitempty"`
	// Match are words that place a section in this slot when the designer
	// named none.
	Match []string `json:"match,omitempty"`
}

// Pattern is the template of a page pattern (or a shell when Shell).
type Pattern struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Use   string `json:"use"`
	Shell bool   `json:"shell,omitempty"`
	// Frame "centered" puts the page in a centered card (auth).
	Frame string `json:"frame,omitempty"`
	// Columns and Areas are the desktop grid: each row of Areas names the
	// slot of each column. Empty slots collapse; small screens stack the
	// slots in Slots order.
	Columns string     `json:"columns,omitempty"`
	Areas   [][]string `json:"areas,omitempty"`
	Slots   []Slot     `json:"slots"`
	// Panel are slots rendered as one data panel: the toolbar above, the
	// collection (the first table, list or cards slot), pagination below.
	Panel   []string         `json:"panel,omitempty"`
	Rules   []string         `json:"rules,omitempty"`
	Avoid   []string         `json:"avoid,omitempty"`
	Example *core.LayoutPage `json:"example,omitempty"`
}

// Slot returns the slot of the pattern by id.
func (p *Pattern) Slot(id string) (Slot, bool) {
	for _, s := range p.Slots {
		if s.ID == id {
			return s, true
		}
	}
	return Slot{}, false
}

// Kit is the loaded UI kit.
type Kit struct {
	Presets    []Preset             `json:"presets"`
	Patterns   []Pattern            `json:"patterns"`
	Components []core.ComponentRule `json:"components"`
	CSS        string               `json:"-"`
	// Version is a hash of the kit data; a mockup changes when it does.
	Version string `json:"version"`
}

var (
	once   sync.Once
	loaded *Kit
)

// Default is the kit: the embedded data plus the user's overrides.
func Default() *Kit {
	once.Do(func() {
		loaded = Load(filepath.Join(config.Home(), "uikit"))
	})
	return loaded
}

// Load reads the embedded kit and overlays dir (presets/*.json,
// patterns/*.json); a missing dir is the embedded kit alone.
func Load(dir string) *Kit {
	k := &Kit{}
	sum := sha256.New()
	readAll := func(fsys fs.FS, sub string, each func(name string, b []byte)) {
		entries, err := fs.ReadDir(fsys, sub)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			b, err := fs.ReadFile(fsys, sub+"/"+e.Name())
			if err != nil {
				log.Printf("uikit: %s/%s: %v", sub, e.Name(), err)
				continue
			}
			sum.Write(b)
			each(sub+"/"+e.Name(), b)
		}
	}
	addPreset := func(name string, b []byte) {
		var p Preset
		if err := json.Unmarshal(b, &p); err != nil || p.ID == "" {
			log.Printf("uikit: preset %s tidak valid: %v", name, err)
			return
		}
		k.Presets = slices.DeleteFunc(k.Presets, func(x Preset) bool { return x.ID == p.ID })
		k.Presets = append(k.Presets, p)
	}
	addPattern := func(name string, b []byte) {
		var p Pattern
		if err := json.Unmarshal(b, &p); err != nil || p.ID == "" || len(p.Slots) == 0 {
			log.Printf("uikit: pattern %s tidak valid: %v", name, err)
			return
		}
		if !slices.Contains(core.LayoutPatterns, p.ID) {
			log.Printf("uikit: pattern %s: %q bukan pattern layout Foundry", name, p.ID)
			return
		}
		k.Patterns = slices.DeleteFunc(k.Patterns, func(x Pattern) bool { return x.ID == p.ID })
		k.Patterns = append(k.Patterns, p)
	}
	embedded, _ := fs.Sub(dataFS, "data")
	readAll(embedded, "presets", addPreset)
	readAll(embedded, "patterns", addPattern)
	if b, err := fs.ReadFile(embedded, "components.json"); err == nil {
		sum.Write(b)
		json.Unmarshal(b, &k.Components)
	}
	if b, err := fs.ReadFile(embedded, "kit.css"); err == nil {
		sum.Write(b)
		k.CSS = string(b)
	}
	if dir != "" {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			user := os.DirFS(dir)
			readAll(user, "presets", addPreset)
			readAll(user, "patterns", addPattern)
		}
	}
	sort.SliceStable(k.Presets, func(i, j int) bool { return presetOrder(k.Presets[i].ID) < presetOrder(k.Presets[j].ID) })
	sort.SliceStable(k.Patterns, func(i, j int) bool {
		return slices.Index(core.LayoutPatterns, k.Patterns[i].ID) < slices.Index(core.LayoutPatterns, k.Patterns[j].ID)
	})
	k.Version = hex.EncodeToString(sum.Sum(nil))[:12]
	return k
}

// presetOrder keeps the curated presets first, densest to calmest.
func presetOrder(id string) int {
	if i := slices.Index([]string{"ops-dense", "saas-neutral", "consumer-warm", "editorial-calm"}, id); i >= 0 {
		return i
	}
	return 100
}

// Preset returns a preset by id.
func (k *Kit) Preset(id string) (*Preset, bool) {
	for i := range k.Presets {
		if k.Presets[i].ID == id {
			return &k.Presets[i], true
		}
	}
	return nil, false
}

// Pattern returns a pattern template by id.
func (k *Kit) Pattern(id string) (*Pattern, bool) {
	for i := range k.Patterns {
		if k.Patterns[i].ID == id {
			return &k.Patterns[i], true
		}
	}
	return nil, false
}

// Component returns a kit component by name (case-insensitive).
func (k *Kit) Component(name string) (core.ComponentRule, bool) {
	for _, c := range k.Components {
		if strings.EqualFold(c.Name, name) {
			return c, true
		}
	}
	return core.ComponentRule{}, false
}

func init() {
	core.KitPreset = func(id string) (core.DesignTokens, bool) {
		p, ok := Default().Preset(id)
		if !ok {
			return core.DesignTokens{}, false
		}
		return p.Tokens, true
	}
	core.KitSlots = func(pattern string) ([]core.KitSlot, bool) {
		p, ok := Default().Pattern(pattern)
		if !ok {
			return nil, false
		}
		out := make([]core.KitSlot, 0, len(p.Slots))
		for _, s := range p.Slots {
			out = append(out, core.KitSlot{ID: s.ID, Kind: s.Kind, Required: s.Required})
		}
		return out, true
	}
}
