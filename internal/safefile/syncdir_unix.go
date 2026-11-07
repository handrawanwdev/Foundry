//go:build !windows

package safefile

import "os"

// syncDir flushes a directory so a rename inside it survives power loss.
func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		d.Sync()
		d.Close()
	}
}
