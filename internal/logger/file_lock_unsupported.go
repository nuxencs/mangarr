//go:build !windows && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd

package logger

import (
	"fmt"
	"os"
	"runtime"
)

func lockFile(_ *os.File, _ bool) (bool, error) {
	return false, fmt.Errorf("download file logging does not support file locking on %s", runtime.GOOS)
}
