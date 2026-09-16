//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package logger

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// Closing the descriptor releases the lock, including after an interrupted run.
func lockFile(file *os.File, nonblocking bool) (bool, error) {
	flags := unix.LOCK_EX
	if nonblocking {
		flags |= unix.LOCK_NB
	}
	for {
		err := unix.Flock(int(file.Fd()), flags)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if nonblocking && (errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN)) {
			return false, nil
		}
		return err == nil, err
	}
}
