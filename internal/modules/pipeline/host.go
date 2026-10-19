package pipeline

import (
	"fmt"

	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/work"
)

// WorkItem is what an MCP host must do next.
type WorkItem struct {
	Kind   string `json:"kind"` // discovery | design | design_qa | reference | stack | structure | none
	Prompt string `json:"prompt"`
	Submit string `json:"submit"` // how to hand the result back
}

// NextWork hands the pending job to the MCP host.
func (s *Service) NextWork() (*WorkItem, error) {
	p, err := s.Store.Load()
	if err != nil {
		return nil, err
	}
	if p.Job == nil || p.Job.Status != core.JobWaiting {
		return &WorkItem{Kind: "none", Prompt: nothingMessage(p)}, nil
	}
	switch p.Job.Kind {
	case work.KindDiscovery, work.KindDesign, work.KindDesignQA, work.KindReference, work.KindStack, work.KindStructure:
		return &WorkItem{Kind: p.Job.Kind, Prompt: s.promptFor(p, p.Job.Kind),
			Submit: fmt.Sprintf(`submit_work {"kind": %q, "result": "<the JSON object>"}`, p.Job.Kind)}, nil
	}
	return &WorkItem{Kind: "none", Prompt: nothingMessage(p)}, nil
}

func nothingMessage(p *core.Project) string {
	switch p.Stage {
	case core.StageTell:
		return "User belum menuliskan prompt di UI Foundry."
	case core.StageDesign:
		return "Rancangan menunggu tinjauan user di UI Foundry."
	case core.StageReady:
		return "Rancangan sudah difinalisasi; spesifikasi ada di docs/."
	}
	return "Tidak ada pekerjaan saat ini."
}

// SubmitWork applies a discovery/design answer from the MCP host.
func (s *Service) SubmitWork(kind, result string) error {
	p, err := s.Store.Load()
	if err != nil {
		return err
	}
	if p.Job == nil || p.Job.Status != core.JobWaiting || p.Job.Kind != kind {
		return fmt.Errorf("%w: Foundry tidak sedang menunggu %s", ErrWrongStage, kind)
	}
	err = s.applyAI(kind, result, "mcp")
	s.noteRejection(kind, err) // the host gets the error; next_work shows it too
	return err
}
