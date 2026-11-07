//go:build linux

package server

import "syscall"

func readDiskUsage() (uint64, uint64) {
	var stats syscall.Statfs_t
	if err := syscall.Statfs("/", &stats); err != nil {
		return 0, 0
	}
	total := stats.Blocks * uint64(stats.Bsize)
	free := stats.Bavail * uint64(stats.Bsize)
	if free > total {
		return 0, total
	}
	return total - free, total
}
