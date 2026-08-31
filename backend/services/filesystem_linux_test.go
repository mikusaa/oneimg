//go:build linux

package services

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestFilesystemBlockSizePrefersLinuxFragmentSize(t *testing.T) {
	stats := unix.Statfs_t{Bsize: 1024 * 1024, Frsize: 4096}
	if got := filesystemBlockSize(stats); got != 4096 {
		t.Fatalf("filesystemBlockSize() = %d, want 4096", got)
	}
}

func TestFilesystemBlockSizeFallsBackToBlockSize(t *testing.T) {
	stats := unix.Statfs_t{Bsize: 4096}
	if got := filesystemBlockSize(stats); got != 4096 {
		t.Fatalf("filesystemBlockSize() = %d, want 4096", got)
	}
}
