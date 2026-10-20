package standard

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// The rules adapt to the project: no stack or tool is mandatory.
func TestRulesForceNoStack(t *testing.T) {
	for _, w := range []string{"Redis required", "ORM required"} {
		if strings.Contains(Rules, w) {
			t.Errorf("Rules force %q", w)
		}
	}
	if !strings.Contains(AgentsMD, "docs/foundry/") || !strings.Contains(PackageGuide, "gelombang") {
		t.Fatal("agent guide does not point to the export package")
	}
}

// Rules is pinned: the text lands in discovery prompts. Update this hash on
// purpose when rules change.
func TestRulesPinned(t *testing.T) {
	sum := sha256.Sum256([]byte(Rules))
	if got := hex.EncodeToString(sum[:8]); got != "90db9be0a66c8cd2" {
		t.Fatalf("Rules changed (hash %s): update this test", got)
	}
}
