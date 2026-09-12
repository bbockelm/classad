//go:build linux

package collections

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// reserve commits real disk blocks for the whole segment file up front (fallocate), instead of a
// sparse Truncate that reserves only the file length and defers block allocation to mmap writeback.
//
// This concentrates a disk-full (ENOSPC) at ALLOCATION -- where allocSeg records a sticky writeErr
// and the write fails cleanly -- rather than at a later msync, where an mmap-writeback failure on a
// sparse file can silently lose data (the per-commit msync error is not surfaced) or deliver SIGBUS
// and crash the process. With the blocks reserved, writeback of an allocated segment cannot ENOSPC.
//
// Falls back to a sparse Truncate on a filesystem that does not support fallocate (the pre-existing
// behavior), so those deployments are no worse off than before.
func reserve(f *os.File, size int64) error {
	err := unix.Fallocate(int(f.Fd()), 0, 0, size)
	if err == nil {
		return nil
	}
	if errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOSYS) {
		return f.Truncate(size) // filesystem/kernel without fallocate: best-effort sparse
	}
	return err // ENOSPC and friends propagate -> allocSeg fails cleanly
}
