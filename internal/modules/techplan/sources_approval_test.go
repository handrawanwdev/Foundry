package techplan_test

import (
	"maps"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/repoplan"
	"github.com/handrawanwdev/foundry/internal/modules/structure"
	"github.com/handrawanwdev/foundry/internal/modules/techplan"
)

// Finalize approves the platform and Reopen clears it. Neither changes what
// the plans are written from, so neither may mark them stale: a stale plan
// changes its artifact and every command that recorded it needs review, and
// finalize could never succeed.
func TestPlatformApprovalIsNotASource(t *testing.T) {
	p := &core.Project{Name: "x", Story: "s", Platform: &core.PlatformPlan{
		Targets: []core.PlatformTarget{{Kind: core.PlatformWeb}}, Reason: "browser"}}
	tech, repo, st := techplan.SourceAreas(p), repoplan.SourceAreas(p), structure.SourceHash(p)
	p.Platform.Approved = true
	if !maps.Equal(tech, techplan.SourceAreas(p)) {
		t.Error("technology plan sources changed by platform approval")
	}
	if !maps.Equal(repo, repoplan.SourceAreas(p)) {
		t.Error("repository plan sources changed by platform approval")
	}
	if st != structure.SourceHash(p) {
		t.Error("structure analysis source changed by platform approval")
	}
	p.Platform.Reason = "changed"
	if maps.Equal(tech, techplan.SourceAreas(p)) || maps.Equal(repo, repoplan.SourceAreas(p)) {
		t.Error("platform content change must still be a source change")
	}
}
