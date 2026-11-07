package safefile

// syncDir is a no-op: Windows cannot fsync a directory handle.
func syncDir(string) {}
