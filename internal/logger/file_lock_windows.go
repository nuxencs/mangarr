package logger

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func lockFile(file *os.File, nonblocking bool) (bool, error) {
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK)
	if nonblocking {
		flags |= windows.LOCKFILE_FAIL_IMMEDIATELY
	}
	// Lock beyond log contents so Windows readers can still inspect an active run.
	offset := windows.Overlapped{OffsetHigh: 0x7fffffff, Offset: 0xffffffff}
	err := windows.LockFileEx(windows.Handle(file.Fd()), flags, 0, 1, 0, &offset)
	if nonblocking && errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return false, nil
	}
	return err == nil, err
}
