//go:build linux

package services

import "golang.org/x/sys/unix"

// Linux exposes both the preferred allocation unit (Frsize) and the
// filesystem block size (Bsize). virtiofs can report a large Bsize while
// keeping Frsize at the actual host allocation unit.
func filesystemBlockSize(stats unix.Statfs_t) uint64 {
	if stats.Frsize > 0 {
		return uint64(stats.Frsize)
	}
	return uint64(stats.Bsize)
}
