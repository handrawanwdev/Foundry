package pipeline

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/spec"
	"github.com/handrawanw/foundry/internal/modules/work"
	"github.com/handrawanw/foundry/internal/modules/workflow"
)

// Deleting a blueprint document: the document's data is removed and
// everything that was built on it is released at once, so the project stays
// valid; the AI then rewrites what still mentions the document. The impact
// shown before the user confirms comes from the same code on a copy.

// Fates of a command that referred to a deleted document.
const (
	FateRemoved = "removed" // it had no other document: it goes
	FateReview  = "review" // it keeps its other documents and is marked for review
)

// ImpactCommand is a command reached by a deletion.
type ImpactCommand struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Fate string `json:"fate"`
	// Why says what reached it, in plain words.
	Why string `json:"why"`
}

// ImpactRef is a blueprint document that refers to the deleted one.
type ImpactRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Impact is what deleting one document reaches.
type Impact struct {
	Doc   string `json:"doc"`
	Title string `json:"title"`
	// Erases is the data that goes with the document.
	Erases    string          `json:"erases"`
	Docs      []string        `json:"docs"` // every document that disappears
	Commands  []ImpactCommand `json:"commands"`
	Artifacts []ImpactRef     `json:"artifacts"` // written documents that refer to it
	Areas     []string        `json:"areas"`     // blueprint areas that become incomplete
	Uncovered []string        `json:"uncovered"` // requirements no command implements any more
	// AI is true when the AI rewrites the affected documents and commands
	// after the deletion.
	AI bool `json:"ai"`

	referrers []string
	review    []string
}

// deleteDoc removes the document and releases what depends on it. It changes
// p; callers that only preview pass a copy.
func deleteDoc(p *core.Project, id string) (*Impact, error) {
	docs := spec.Artifacts(p)
	i := slices.IndexFunc(docs, func(d core.Artifact) bool { return d.ID == id })
	if i < 0 {
		return nil, fmt.Errorf("dokumen %q tidak ada", id)
	}
	im := &Impact{Doc: id, Title: docs[i].Title, Docs: []string{}, Commands: []ImpactCommand{}, Artifacts: []ImpactRef{},
		Areas: []string{}, Uncovered: []string{}}
	before := areaStates(p, docs)
	var uncoveredBefore []string
	if p.Workflow != nil {
		_, uncoveredBefore = workflow.Covers(p.Workflow.Commands, p.Requirements)
	}

	switch {
	case slices.ContainsFunc(p.Artifacts, func(a core.Artifact) bool { return a.ID == id }):
		im.Erases = "Dokumen ini dihapus."
		im.Docs = []string{id}
		for _, a := range p.Artifacts {
			if a.ID != id && slices.Contains(a.Refs, id) {
				im.referrers = append(im.referrers, a.ID)
			}
		}
		if err := work.RemoveArtifact(p, id); err != nil {
			return nil, err
		}
	case id == core.ArtifactUseCases:
		im.Erases = fmt.Sprintf("%d use case dihapus.", len(p.Flows))
		im.Docs = []string{id}
		p.Flows = nil
	case id == core.ArtifactAcceptance:
		n := 0
		for k := range p.Requirements {
			n += len(p.Requirements[k].Acceptance)
			p.Requirements[k].Acceptance, p.Requirements[k].Untestable = nil, ""
		}
		im.Erases = fmt.Sprintf("%d kriteria penerimaan dan %d butir Definition of Done dihapus. Requirement tetap ada.", n, len(p.Done))
		im.Docs = []string{id}
		p.Done = nil
	case id == core.ArtifactDesignRules || id == core.ArtifactLayoutSpec:
		pages := 0
		if p.UI != nil && p.UI.Layout != nil {
			pages = len(p.UI.Layout.Pages)
		}
		im.Erases = fmt.Sprintf("Aturan desain dan spesifikasi layout (%d halaman) dihapus; keduanya saling bergantung.", pages)
		for _, d := range docs {
			if d.ID == core.ArtifactDesignRules || d.ID == core.ArtifactLayoutSpec {
				im.Docs = append(im.Docs, d.ID)
			}
		}
		p.UI = nil
	default:
		return nil, fmt.Errorf("dokumen %q dihitung dari data lain dan tidak bisa dihapus langsung", id)
	}

	for _, a := range p.Artifacts {
		if slices.ContainsFunc(im.Docs, func(d string) bool { return slices.Contains(a.Refs, d) }) && !slices.Contains(im.referrers, a.ID) {
			im.referrers = append(im.referrers, a.ID)
		}
	}
	for k := range p.Artifacts {
		a := &p.Artifacts[k]
		if !slices.Contains(im.referrers, a.ID) {
			continue
		}
		im.Artifacts = append(im.Artifacts, ImpactRef{ID: a.ID, Title: a.Title})
		a.Refs = slices.DeleteFunc(a.Refs, func(r string) bool { return slices.Contains(im.Docs, r) })
		if len(a.Refs) == 0 {
			a.Refs = nil
		}
		for _, d := range im.Docs {
			delete(a.Seen, d)
		}
	}

	releaseCommands(p, im)

	for _, a := range core.Coverage(p, spec.Artifacts(p)) {
		if (a.Status == core.AreaMissing || a.Status == core.AreaInvalid) && before[a.ID] != a.Status {
			im.Areas = append(im.Areas, a.Title)
		}
	}
	if p.Workflow != nil {
		_, uncovered := workflow.Covers(p.Workflow.Commands, p.Requirements)
		for _, r := range uncovered {
			if !slices.Contains(uncoveredBefore, r) {
				im.Uncovered = append(im.Uncovered, r)
			}
		}
	}
	im.AI = len(im.referrers) > 0 || len(im.review) > 0
	return im, nil
}

// releaseCommands takes the deleted documents off the commands. A command
// left with no document goes; one that depended on it loses that dependency;
// the others stay and are marked for review.
func releaseCommands(p *core.Project, im *Impact) {
	w := p.Workflow
	if w == nil {
		return
	}
	reason := fmt.Sprintf("dokumen %s dihapus", im.Title)
	removed := map[string]bool{}
	review := map[string]string{}
	var kept []core.Command
	for _, c := range w.Commands {
		refs := slices.DeleteFunc(slices.Clone(c.Artifacts), func(r core.ArtifactRef) bool { return slices.Contains(im.Docs, r.ID) })
		switch {
		case len(refs) == len(c.Artifacts):
			kept = append(kept, c)
		case len(refs) == 0:
			removed[c.ID] = true
			im.Commands = append(im.Commands, ImpactCommand{ID: c.ID, Name: c.Name, Fate: FateRemoved, Why: "hanya dibuat untuk dokumen ini"})
		default:
			c.Artifacts = refs
			review[c.ID] = "memakai dokumen ini bersama dokumen lain"
			kept = append(kept, c)
		}
	}
	for k := range kept {
		c := &kept[k]
		if deps := slices.DeleteFunc(slices.Clone(c.DependsOn), func(d string) bool { return removed[d] }); len(deps) != len(c.DependsOn) {
			c.DependsOn = deps
			if _, ok := review[c.ID]; !ok {
				review[c.ID] = "bergantung pada command yang dihapus"
			}
		}
		if why, ok := review[c.ID]; ok {
			c.Status, c.Review = core.CommandNeedsReview, strings.ToUpper(reason[:1])+reason[1:]
			im.Commands = append(im.Commands, ImpactCommand{ID: c.ID, Name: c.Name, Fate: FateReview, Why: why})
			im.review = append(im.review, c.ID)
		}
	}
	if len(im.Commands) == 0 {
		return
	}
	w.Commands = kept
	w.Version++
}

func areaStates(p *core.Project, docs []core.Artifact) map[string]string {
	out := map[string]string{}
	for _, a := range core.Coverage(p, docs) {
		out[a.ID] = a.Status
	}
	return out
}

// cleanupRequests sets the AI requests that follow a deletion and returns
// the job to start first ("" when nothing needs the AI). The artifact job
// runs first and hands over to the workflow job.
func cleanupRequests(p *core.Project, im *Impact) string {
	kind := ""
	if len(im.review) > 0 {
		p.Workflow.Request = &core.ChangeRequest{Focus: im.review, Cascade: true, Text: fmt.Sprintf(
			"Dokumen %q (%s) dihapus dari blueprint. Command yang tercantum sudah dilepas dari dokumen itu dan ditandai perlu ditinjau. "+
				"Tulis ulang instruksinya agar tidak lagi memakai atau menyebut dokumen itu, atau hapus command yang tidak punya tujuan lagi. "+
				"Jangan mengubah command lain dan jangan mengarang pengganti.", im.Title, strings.Join(im.Docs, ", "))}
		kind = work.KindWorkflow
	}
	if len(im.referrers) > 0 {
		p.ArtifactRequest = &core.ChangeRequest{Focus: im.referrers, Cascade: true, Text: fmt.Sprintf(
			"Dokumen %q (%s) dihapus dari blueprint. Artefak berikut masih merujuknya: %s. Hapus rujukan dan isi yang bergantung padanya; "+
				"hapus artefak yang sepenuhnya hanya ada karena dokumen itu. Jangan mengarang pengganti.",
			im.Title, strings.Join(im.Docs, ", "), strings.Join(im.referrers, ", "))}
		kind = work.KindArtifacts
	}
	return kind
}

// DeleteImpact previews deleting a document without changing anything.
func (s *Service) DeleteImpact(id string) (*Impact, error) {
	p, err := s.Store.Load()
	if err != nil {
		return nil, err
	}
	if err := blueprintEditable(p); err != nil {
		return nil, err
	}
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var c core.Project
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return deleteDoc(&c, id)
}

// DeleteDocument deletes a document with what depends on it; the AI then
// rewrites what still mentions it.
func (s *Service) DeleteDocument(id string) error {
	kind := ""
	_, err := s.Store.Update(func(p *core.Project) error {
		if err := blueprintEditable(p); err != nil {
			return err
		}
		im, err := deleteDoc(p, id)
		if err != nil {
			return err
		}
		stampRefs(p)
		kind = cleanupRequests(p, im)
		p.Log("ui", fmt.Sprintf("Dokumen dihapus: %s (%d command terdampak)", id, len(im.Commands)))
		return nil
	})
	if err != nil || kind == "" {
		return err
	}
	return s.startAI(kind)
}
