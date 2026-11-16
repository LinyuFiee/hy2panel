//go:build !windows

package traffic

import "os"

func replaceFile(source, target string) error {
	return os.Rename(source, target)
}

func isIgnorableDirectorySyncError(_ error) bool {
	return false
}
