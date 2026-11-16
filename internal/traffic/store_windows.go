//go:build windows

package traffic

import (
	"errors"
	"syscall"
	"unsafe"
)

const replaceFileWriteThrough = 0x00000001

var replaceFileW = syscall.NewLazyDLL("kernel32.dll").NewProc("ReplaceFileW")

func replaceFile(source, target string) error {
	sourcePointer, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	targetPointer, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	if _, statErr := syscall.GetFileAttributes(targetPointer); errors.Is(statErr, syscall.ERROR_FILE_NOT_FOUND) || errors.Is(statErr, syscall.ERROR_PATH_NOT_FOUND) {
		return syscall.Rename(source, target)
	} else if statErr != nil {
		return statErr
	}
	result, _, callErr := replaceFileW.Call(
		uintptr(unsafe.Pointer(targetPointer)),
		uintptr(unsafe.Pointer(sourcePointer)),
		0,
		replaceFileWriteThrough,
		0,
		0,
	)
	if result == 0 {
		return callErr
	}
	return nil
}

func isIgnorableDirectorySyncError(err error) bool {
	return errors.Is(err, syscall.ERROR_ACCESS_DENIED) || errors.Is(err, syscall.Errno(6))
}
