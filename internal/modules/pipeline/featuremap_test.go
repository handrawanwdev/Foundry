package pipeline_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

func mapped(t *testing.T) (*pipeline.Service, *core.Project) {
	t.Helper()
	s := newService(t, fakeAI{}, "")
	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	mustJob(t, p, work.KindFeatureMap, core.JobDone)
	if p.FeatureMap.Empty() || p.FeatureMap.Find("buat-janji") == nil {
		t.Fatalf("feature map = %+v", p.FeatureMap)
	}
	return s, p
}

func TestSaveFeatureMap(t *testing.T) {
	s, _ := mapped(t)
	p, err := s.SaveFeatureMap(&core.FeatureMap{
		Summary: "Klinik",
		Modules: []core.FeatureModule{{
			Name: "Identitas",
			Features: []core.FeatureNode{{
				Name: "Login", Description: "Masuk akun", Purpose: "Akses",
				Benefit: "Data aman", Outcome: "Sesi aktif",
			}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.FeatureMap.Find("login") == nil || p.FeatureMap.FindModule("identitas") == nil {
		t.Fatalf("saved = %+v", p.FeatureMap)
	}
	if _, err := s.SaveFeatureMap(&core.FeatureMap{}); err != nil {
		t.Fatal(err)
	}
	if !load(t, s).FeatureMap.Empty() {
		t.Fatal("empty save should persist")
	}
}

func TestGenerateExpandGaps(t *testing.T) {
	s, p := mapped(t)
	id := p.FeatureMap.Find("buat-janji").ID
	if err := s.ExpandFeature(id, "tambah pilih jam"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	n := p.FeatureMap.Find("buat-janji")
	if n == nil || len(n.Children) < 3 {
		t.Fatalf("expand = %+v", n)
	}
	if p.FeatureMap.Find("pilih-jam") == nil {
		t.Fatalf("missing child: %+v", n.Children)
	}

	if err := s.FindFeatureGaps("yang belum ada"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	if p.FeatureMap.FindModule("tagihan") == nil || p.FeatureMap.Find("invoice") == nil {
		t.Fatalf("gaps = %+v", p.FeatureMap)
	}

	if err := s.GenerateFeatureMap("ulangi dari cerita"); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p = load(t, s)
	if p.FeatureMap.Find("buat-janji") == nil || p.FeatureMap.FindModule("tagihan") != nil {
		t.Fatalf("generate should replace: %+v", p.FeatureMap)
	}
}

func TestStartBlueprint(t *testing.T) {
	s, _ := mapped(t)
	if err := s.StartBlueprint(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	p := load(t, s)
	mustJob(t, p, work.KindDesign, core.JobDone)
	if p.Stage != core.StageDesign || p.Discovery == nil {
		t.Fatalf("stage=%s discovery=%v", p.Stage, p.Discovery)
	}
	if err := s.StartBlueprint(); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("second start err = %v", err)
	}
}

func TestFeatureMapGuards(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	if _, err := s.SaveFeatureMap(&core.FeatureMap{Modules: []core.FeatureModule{{Name: "X"}}}); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("save at tell err = %v", err)
	}
	if err := s.GenerateFeatureMap(""); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("generate at tell err = %v", err)
	}
	if err := s.StartBlueprint(); !errors.Is(err, pipeline.ErrWrongStage) {
		t.Fatalf("blueprint at tell err = %v", err)
	}

	if _, err := s.SubmitStory("Saya ingin membuat aplikasi klinik untuk janji temu pasien"); err != nil {
		t.Fatal(err)
	}
	if err := s.GenerateFeatureMap(""); !errors.Is(err, pipeline.ErrBusy) {
		t.Fatalf("generate while running err = %v", err)
	}
	if err := s.ExpandFeature("buat-janji", ""); err == nil {
		t.Fatal("expand while empty/running")
	}
	s.Wait()

	if err := s.ExpandFeature("", "x"); err == nil || !strings.Contains(err.Error(), "pilih fitur") {
		t.Fatalf("expand without id err = %v", err)
	}
	if err := s.ExpandFeature("tidak-ada", ""); err == nil || !strings.Contains(err.Error(), "tidak ditemukan") {
		t.Fatalf("expand missing err = %v", err)
	}
}
