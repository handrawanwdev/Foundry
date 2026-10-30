package pipeline

import (
	"errors"
	"fmt"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

func (s *Service) GenerateFeatureMap(text string) error {
	return s.startFeatureMap(core.FeatureMapGenerate, "", text)
}

func (s *Service) ExpandFeature(id, text string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("pilih fitur yang akan dikembangkan")
	}
	return s.startFeatureMap(core.FeatureMapExpand, id, text)
}

func (s *Service) FindFeatureGaps(text string) error {
	return s.startFeatureMap(core.FeatureMapGaps, "", text)
}

func (s *Service) startFeatureMap(action, nodeID, text string) error {
	text = strings.TrimSpace(text)
	return s.startAIIf(work.KindFeatureMap, func(p *core.Project) error {
		if err := featureMapEditable(p); err != nil {
			return err
		}
		if p.Job != nil && p.Job.Status == core.JobWaiting {
			return ErrBusy
		}
		if action == core.FeatureMapExpand {
			if p.FeatureMap.Empty() {
				return errors.New("belum ada daftar fitur")
			}
			if mod, node := p.FeatureMap.Locate(nodeID); mod == nil && node == nil {
				return errors.New("fitur tidak ditemukan")
			}
		}
		if p.FeatureMap == nil {
			p.FeatureMap = &core.FeatureMap{}
		}
		p.FeatureMap.Request = &core.FeatureMapRequest{Action: action, NodeID: nodeID, Text: text}
		return nil
	})
}

func (s *Service) SaveFeatureMap(m *core.FeatureMap) (*core.Project, error) {
	return s.Store.Update(func(p *core.Project) error {
		if err := featureMapEditable(p); err != nil {
			return err
		}
		if p.Job != nil && p.Job.Status == core.JobWaiting {
			return ErrBusy
		}
		if m == nil {
			m = &core.FeatureMap{}
		}
		m.Normalize()
		if err := m.Validate(); err != nil {
			return err
		}
		p.FeatureMap = m
		p.Log("ui", "Daftar fitur disimpan")
		return nil
	})
}

// StartBlueprint starts the existing discovery job using the feature map as
// a reference. Stage stays DISCOVERY until discovery lands.
func (s *Service) StartBlueprint() error {
	return s.startAIIf(work.KindDiscovery, func(p *core.Project) error {
		if p.Busy() || (p.Job != nil && p.Job.Status == core.JobWaiting) {
			return ErrBusy
		}
		if p.Finalized() {
			return ErrFinalized
		}
		if p.Stage != core.StageDiscovery {
			return fmt.Errorf("%w: daftar fitur sudah dilanjutkan ke blueprint", ErrWrongStage)
		}
		if strings.TrimSpace(p.Story) == "" {
			return fmt.Errorf("%w: tulis prompt dulu", ErrWrongStage)
		}
		return nil
	})
}

func featureMapEditable(p *core.Project) error {
	switch {
	case p.Busy():
		return ErrBusy
	case p.Finalized():
		return ErrFinalized
	case p.Stage == core.StageTell:
		return fmt.Errorf("%w: tulis prompt dulu", ErrWrongStage)
	}
	return nil
}
