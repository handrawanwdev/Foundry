package checks

import (
	"context"
	"math/rand"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/handrawanw/foundry/internal/core"
)

// tailWriter must read exactly like tail of everything written, whatever
// the write sizes.
func TestTailWriterMatchesTail(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for round := 0; round < 200; round++ {
		w := &tailWriter{max: maxOutput}
		var all strings.Builder
		for n := rng.Intn(40); n > 0; n-- {
			size := rng.Intn(3 * maxOutput)
			if rng.Intn(3) > 0 {
				size = rng.Intn(64)
			}
			chunk := make([]byte, size)
			for i := range chunk {
				chunk[i] = byte('a' + rng.Intn(26))
			}
			w.Write(chunk)
			all.Write(chunk)
		}
		if got, want := w.String(), tail(all.String()); got != want {
			t.Fatalf("round %d: tail mismatch (got %d bytes, want %d)", round, len(got), len(want))
		}
		if cap(w.buf) > 2*maxOutput {
			t.Fatalf("round %d: buffer grew to %d", round, cap(w.buf))
		}
	}
}

// A command printing far more than the limit keeps only the tail, without
// holding the whole output in memory.
func TestRunBoundsOutput(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh tidak ada")
	}
	const printed = 64 << 20
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	b := Run(context.Background(), t.TempDir(), []core.Command{{Name: "test",
		Argv: []string{"sh", "-c", "head -c 67108864 /dev/zero | tr '\\0' x; echo END"}}}, "")
	runtime.ReadMemStats(&after)
	if !b.Passed {
		t.Fatalf("build = %+v", b)
	}
	out := b.Steps[0].Output
	if len(out) != maxOutput+len("…") || !strings.HasPrefix(out, "…") || !strings.HasSuffix(out, "xxxxEND\n") {
		t.Fatalf("output: %d bytes, %q…%q", len(out), out[:8], out[len(out)-8:])
	}
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > printed/4 {
		t.Fatalf("allocated %d bytes for %d bytes of output", alloc, printed)
	}
}

// A server a check started in the background must not outlive the check
// and hold its port for the next one.
func TestRunKillsLeftoverProcess(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh tidak ada")
	}
	b := Run(context.Background(), t.TempDir(), []core.Command{{Name: "test",
		Argv: []string{"sh", "-c", "sleep 300 >/dev/null 2>&1 & echo $!"}}}, "")
	if !b.Passed {
		t.Fatalf("build = %+v", b)
	}
	waitExited(t, strings.TrimSpace(b.Steps[0].Output))
}
