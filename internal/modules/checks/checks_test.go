package checks

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/handrawanw/foundry/internal/core"
)

// A background process left by a check keeps the output pipe open; the run
// must still return, and pass on the command's own exit code.
func TestRunNotHeldByBackgroundChild(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh tidak ada")
	}
	start := time.Now()
	b := Run(context.Background(), t.TempDir(), []core.Command{{Name: "test", Argv: []string{"sh", "-c", "echo ok; sleep 60 &"}}}, "")
	if d := time.Since(start); d > 20*time.Second {
		t.Fatalf("Run tertahan %s", d)
	}
	if !b.Passed || b.Steps[0].Output != "ok\n" {
		t.Fatalf("build = %+v", b)
	}
}
