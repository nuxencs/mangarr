package cmd

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// Automatic config discovery must not read a developer's real configuration.
	home, err := os.MkdirTemp("", "mangarr-command-tests-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, key := range []string{"HOME", "XDG_CONFIG_HOME", "AppData"} {
		if err := os.Setenv(key, home); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "MANGARR__") {
			_ = os.Unsetenv(key)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}
