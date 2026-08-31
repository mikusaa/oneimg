//go:build !linux

package services

import "golang.org/x/sys/unix"

func filesystemBlockSize(stats unix.Statfs_t) uint64 {
	return uint64(stats.Bsize)
}
