package progress

import (
	"fmt"
	"testing"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

// BenchmarkView covers a workflow far larger than a usual project, where a
// per-feature scan of every command would show up as quadratic growth.
func BenchmarkView(b *testing.B) {
	for _, size := range []int{20, 500} {
		b.Run(fmt.Sprintf("commands=%d", size), func(b *testing.B) {
			p := &core.Project{Workflow: &core.Workflow{}, Progress: &core.Progress{Commands: map[string]core.CommandRun{}}}
			for i := 0; i < size/4; i++ {
				f := core.Feature{ID: fmt.Sprintf("FEAT-%03d", i), Name: fmt.Sprintf("Fitur %d", i)}
				for j := 0; j < 4; j++ {
					id := fmt.Sprintf("%s-S%d", f.ID, j)
					f.SubFeatures = append(f.SubFeatures, core.SubFeature{ID: id, FeatureID: f.ID, Name: id})
					cmd := core.Command{ID: fmt.Sprintf("CMD-%03d", i*4+j), FeatureIDs: []string{f.ID}, SubFeatureIDs: []string{id}}
					if i > 0 {
						cmd.DependsOn = []string{fmt.Sprintf("CMD-%03d", (i-1)*4+j)}
					}
					p.Workflow.Commands = append(p.Workflow.Commands, cmd)
					p.Progress.Commands[cmd.ID] = core.CommandRun{CommandID: cmd.ID, Status: core.RunVerified}
				}
				p.Features = append(p.Features, f)
			}
			now := time.Now()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				View(p, now, Agent{})
			}
		})
	}
}
