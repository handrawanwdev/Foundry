//go:build !windows

package safefile

// transient is always false: rename and open never conflict here.
func transient(error) bool { return false }
