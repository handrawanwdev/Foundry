package pipeline_test

import (
	"strings"
	"testing"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
	"github.com/handrawanwdev/foundry/internal/modules/uikit"
)

// Every designed page renders as a mockup; the user approves it, a design
// change that alters the page makes the approval stale, and the export
// carries the mockups.
func TestMockupApprovalFollowsDesign(t *testing.T) {
	s := newService(t, fakeAI{}, "")
	designReady(t, s)
	list, err := s.Mockups()
	if err != nil || len(list) == 0 {
		t.Fatalf("mockups = %v, %v", list, err)
	}
	page := list[0].Page
	if list[0].Status != uikit.MockupPending {
		t.Fatalf("status awal = %s", list[0].Status)
	}
	html, err := s.Mockup(page, "", nil)
	if err != nil || !strings.Contains(html, `data-foundry-page="`+page+`"`) {
		t.Fatalf("mockup: %v", err)
	}
	if _, err := s.Mockup(page, "empty", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Mockup("tidak-ada", "", nil); err == nil {
		t.Fatal("halaman tak dikenal dirender")
	}
	list, err = s.ApproveMockups([]string{page})
	if err != nil || list[0].Status != uikit.MockupApproved {
		t.Fatalf("approve: %v %+v", err, list)
	}
	if _, err := s.ApproveMockups([]string{"tidak-ada"}); err == nil {
		t.Fatal("persetujuan halaman tak dikenal diterima")
	}
	if _, err := s.Store.Update(func(p *core.Project) error {
		p.UI.Layout.Pages[0].Purpose += " (diubah)"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	list, _ = s.Mockups()
	if list[0].Status != uikit.MockupChanged {
		t.Fatalf("status setelah berubah = %s", list[0].Status)
	}
	files := spec.Files(load(t, s))
	if !strings.Contains(files["design/mockups/"+page+".html"], "data-foundry-section") || files["design/mockups/index.html"] == "" {
		t.Fatal("ekspor tanpa mockup")
	}
	if !strings.Contains(files["README.md"], "design/mockups/index.html") {
		t.Fatal("README paket tidak menyebut mockup")
	}
}
