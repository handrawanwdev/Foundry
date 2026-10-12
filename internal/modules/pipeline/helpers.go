package pipeline

import (
	"strings"

	"github.com/handrawanw/foundry/internal/core"
)

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// buildActive reports whether a build job is running or waits for an MCP host,
// so tasks put back in the queue are picked up without starting another build.
func buildActive(p *core.Project) bool {
	return p.Job != nil && p.Job.Kind == "build" && (p.Job.Status == core.JobRunning || p.Job.Status == core.JobWaiting)
}
