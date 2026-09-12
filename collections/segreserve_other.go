//go:build unix && !linux

package collections

import "os"

// reserve falls back to a sparse Truncate on non-Linux unix (macOS/BSD are dev platforms; the
// deployed store runs on Linux, where reserve uses fallocate to commit blocks up front). See
// segreserve_linux.go for why up-front reservation matters for ENOSPC safety.
func reserve(f *os.File, size int64) error { return f.Truncate(size) }
