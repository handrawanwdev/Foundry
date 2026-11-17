package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// The layout spec, and the mockups drawn from it, are written from the
// features, the use cases and the blueprint's data and access. The design
// keeps a fingerprint of them from when the AI last wrote or synced it;
// what changed since is the design's drift, and syncing asks the AI to
// follow it. The pages the design owns on a use case (page ids, step
// pages) are no part of it: the design writes them itself.

// Drift kinds: what the layout is drawn from.
const (
	DriftFeature     = "feature"
	DriftFlow        = "flow"
	DriftData        = "data"
	DriftAccess      = "access"
	DriftRequirement = "requirements"
)

// DriftItem is one source that changed since the design was synced.
type DriftItem struct {
	Kind   string `json:"kind"`
	ID     string `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	Change string `json:"change"` // added | changed | removed
}

// DesignDrift is what changed since the design was synced. Known is false
// for a design written before Foundry kept the fingerprint.
type DesignDrift struct {
	Known bool        `json:"known"`
	Items []DriftItem `json:"items"`
}

func fingerprint(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

// DesignBasis fingerprints what the layout is drawn from: key → hash and
// name.
func DesignBasis(p *Project) map[string]string {
	out := map[string]string{}
	for _, f := range p.Features {
		out[DriftFeature+":"+f.ID] = fingerprint(f) + " " + f.Name
	}
	for _, fl := range p.Flows {
		c := fl
		c.PageIDs, c.StepPages, c.BranchPages = nil, nil, nil
		out[DriftFlow+":"+fl.ID] = fingerprint(c) + " " + fl.Name
	}
	if len(p.Requirements) > 0 {
		out[DriftRequirement] = fingerprint(p.Requirements)
	}
	if b := p.Blueprint; b != nil {
		out[DriftData] = fingerprint(b.Database.Entities)
		out[DriftAccess] = fingerprint([]any{b.Auth, b.Authz})
	}
	return out
}

// DesignDrift is what changed since the design was last synced.
func (p *Project) DesignDrift() DesignDrift {
	if p.UI == nil || len(p.UI.Basis) == 0 {
		return DesignDrift{Items: []DriftItem{}}
	}
	return DesignDrift{Known: true, Items: driftOf(p.UI.Basis, DesignBasis(p))}
}

func driftOf(was, now map[string]string) []DriftItem {
	split := func(k, v string) DriftItem {
		kind, id, _ := strings.Cut(k, ":")
		_, name, _ := strings.Cut(v, " ")
		return DriftItem{Kind: kind, ID: id, Name: name}
	}
	hash := func(v string) string { h, _, _ := strings.Cut(v, " "); return h }
	out := []DriftItem{}
	for k, v := range now {
		it := split(k, v)
		switch old, ok := was[k]; {
		case !ok:
			it.Change = "added"
		case hash(old) != hash(v):
			it.Change = "changed"
		default:
			continue
		}
		out = append(out, it)
	}
	for k, v := range was {
		if _, ok := now[k]; !ok {
			it := split(k, v)
			it.Change = "removed"
			out = append(out, it)
		}
	}
	order := map[string]int{DriftFeature: 0, DriftFlow: 1, DriftData: 2, DriftAccess: 3, DriftRequirement: 4}
	slices.SortFunc(out, func(a, b DriftItem) int {
		if order[a.Kind] != order[b.Kind] {
			return order[a.Kind] - order[b.Kind]
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

var driftNames = map[string]string{DriftFeature: "Fitur", DriftFlow: "Use case", DriftData: "Data (entity) blueprint",
	DriftAccess: "Autentikasi dan hak akses blueprint", DriftRequirement: "Requirement"}

var changeNames = map[string]string{"added": "baru", "changed": "berubah", "removed": "dihapus"}

// SyncRequest is the change the AI is asked for to follow the drift: the
// sources that changed, or every source when the drift is unknown.
func (d DesignDrift) SyncRequest() string {
	var b strings.Builder
	b.WriteString("Sinkronkan rancangan UI (layout spec) dengan fitur, use case, dan blueprint terbaru. ")
	b.WriteString("Tambah halaman, bagian, field, aksi, dan navigasi untuk yang baru; ubah yang terdampak perubahan; hapus halaman yang sumbernya sudah dihapus. ")
	b.WriteString("Pertahankan halaman yang tidak terdampak beserta perubahan manualnya.")
	if !d.Known {
		b.WriteString("\nPeriksa semua fitur, use case, entity, dan hak akses terhadap halaman yang ada.")
		return b.String()
	}
	if len(d.Items) == 0 {
		b.WriteString("\nTidak ada sumber yang tercatat berubah; periksa kesesuaian dan ubah hanya yang memang tidak sesuai.")
		return b.String()
	}
	b.WriteString("\nYang berubah sejak rancangan terakhir:")
	for _, it := range d.Items {
		label := driftNames[it.Kind]
		if it.ID != "" {
			label = fmt.Sprintf("%s %s", label, it.ID)
			if it.Name != "" {
				label += " (" + it.Name + ")"
			}
		}
		fmt.Fprintf(&b, "\n- %s: %s", label, changeNames[it.Change])
	}
	return b.String()
}
