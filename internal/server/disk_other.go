//go:build !linux

package server

func readDiskUsage() (uint64, uint64) {
	return 0, 0
}
