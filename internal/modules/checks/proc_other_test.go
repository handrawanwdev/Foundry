//go:build !unix

package checks

import "testing"

func waitExited(t *testing.T, pid string) { t.Skip("process groups are Unix only") }
