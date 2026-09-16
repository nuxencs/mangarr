package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mangarr/internal/domain"

	"github.com/stretchr/testify/require"
)

func TestDownloadRetainsDiscoveryFailure(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "monitor.log")
	configDir := writeDownloadConfig(t, fmt.Sprintf("logPath: %q\n", logPath))
	failure := &url.Error{Op: "Get", URL: "https://user:password@example.invalid/series?label=O'Reilly&token=secret#fragment", Err: errors.New("offline failure")}
	fake := &configuredDownloadSource{discoveryError: failure}
	deps := defaultDependencies()
	deps.selectSource = func(domain.MonitoredManga) (domain.Source, error) { return fake, nil }
	root := newRootCommand(deps)
	var stderr, stdout bytes.Buffer
	root.SetErr(&stderr)
	root.SetOut(&stdout)
	root.SetArgs([]string{"download", "-c", configDir, "-d", t.TempDir(), "-s", "fixture", "-m", "fixture"})
	err := root.Execute()
	require.ErrorIs(t, err, failure)
	// main prints the returned error; the command must not print it again.
	fmt.Fprintln(&stderr, err)
	require.Equal(t, 1, strings.Count(stderr.String(), "offline failure"))
	require.Empty(t, stdout.String())
	require.NoFileExists(t, logPath)
	logs, err := filepath.Glob(logPath + ".downloads/*.jsonl")
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Contains(t, stderr.String(), logs[0])
	data, err := os.ReadFile(logs[0])
	require.NoError(t, err)
	require.Contains(t, string(data), "offline failure")
	require.Contains(t, string(data), "https://example.invalid/series")
	require.Equal(t, 1, strings.Count(string(data), "offline failure"))
	for _, secret := range []string{"password", "user:", "token=", "secret", "fragment", "label=", "Reilly"} {
		require.NotContains(t, string(data), secret)
	}
}

func TestDownloadConfiguredSeriesValidationAndLogging(t *testing.T) {
	for _, test := range []struct {
		name     string
		interval int
		logging  bool
	}{
		{name: "invalid config", interval: 0, logging: true},
		{name: "valid config with file logging", interval: 15, logging: true},
		{name: "valid config without file logging", interval: 15},
	} {
		t.Run(test.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "monitor.log")
			body := fmt.Sprintf("downloadLocation: %q\ncheckInterval: %d\nmonitoredManga:\n  My Series:\n    source: cubari\n    manga: https://example.invalid/gist\n    group: fixture\n", t.TempDir(), test.interval)
			if test.logging {
				body += fmt.Sprintf("logPath: %q\n", logPath)
			}
			configDir := writeDownloadConfig(t, body)
			failure := errors.New("configured series discovery failure")
			fake := &configuredDownloadSource{discoveryError: failure}
			selected := false
			deps := defaultDependencies()
			deps.selectSource = func(domain.MonitoredManga) (domain.Source, error) {
				selected = true
				return fake, nil
			}
			root := newRootCommand(deps)
			var console bytes.Buffer
			root.SetErr(&console)
			root.SetArgs([]string{"download", "-c", configDir, "--series", "My Series"})
			err := root.Execute()
			if test.interval == 0 {
				require.ErrorContains(t, err, `loading config for series "My Series"`)
				require.ErrorContains(t, err, "checkInterval must be greater than zero")
				require.False(t, selected)
				require.False(t, fake.discovered)
			} else {
				require.ErrorIs(t, err, failure)
				require.True(t, fake.discovered)
			}
			logs, err := filepath.Glob(logPath + ".downloads/*.jsonl")
			require.NoError(t, err)
			if test.interval > 0 && test.logging {
				require.Len(t, logs, 1)
				require.Contains(t, console.String(), logs[0])
				data, err := os.ReadFile(logs[0])
				require.NoError(t, err)
				require.Contains(t, string(data), failure.Error())
			} else {
				require.Empty(t, logs)
				require.NotContains(t, console.String(), "Download log:")
			}
			require.NoFileExists(t, logPath)
		})
	}
}

func TestDownloadAutomaticallyUsesDiscoveredLogging(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", home)
			t.Setenv("AppData", home)
			userConfig, err := os.UserConfigDir()
			require.NoError(t, err)
			configDir := filepath.Join(userConfig, "mangarr")
			require.NoError(t, os.MkdirAll(configDir, 0o700))
			logPath := filepath.Join(home, "monitor.log")
			body := "checkInterval: 0\n"
			if enabled {
				body += fmt.Sprintf("logPath: %q\n", logPath)
			}
			configFile := filepath.Join(configDir, "config.yaml")
			require.NoError(t, os.WriteFile(configFile, []byte(body), 0o600))
			fake := &configuredDownloadSource{discoveryError: errors.New("automatic logging fixture")}
			deps := defaultDependencies()
			deps.selectSource = func(domain.MonitoredManga) (domain.Source, error) { return fake, nil }
			root := newRootCommand(deps)
			var console bytes.Buffer
			root.SetErr(&console)
			root.SetArgs([]string{"download", "-d", t.TempDir(), "-s", "fixture", "-m", "fixture"})
			require.ErrorIs(t, root.Execute(), fake.discoveryError)
			logs, err := filepath.Glob(logPath + ".downloads/*.jsonl")
			require.NoError(t, err)
			if enabled {
				require.Len(t, logs, 1)
				require.Contains(t, console.String(), logs[0])
				data, err := os.ReadFile(logs[0])
				require.NoError(t, err)
				require.Contains(t, string(data), fake.discoveryError.Error())
			} else {
				require.Empty(t, logs)
				require.NotContains(t, console.String(), "Download log:")
			}
			data, err := os.ReadFile(configFile)
			require.NoError(t, err)
			require.Equal(t, body, string(data), "one-shot config reads must not rewrite it")
		})
	}
}

func TestDownloadLoggingInitializationStopsBeforeSource(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "monitor.log")
	require.NoError(t, os.WriteFile(logPath+".downloads", []byte("not a directory"), 0o600))
	configDir := writeDownloadConfig(t, fmt.Sprintf("logPath: %q\n", logPath))
	selected := false
	deps := defaultDependencies()
	deps.selectSource = func(domain.MonitoredManga) (domain.Source, error) {
		selected = true
		return &configuredDownloadSource{}, nil
	}
	root := newRootCommand(deps)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"download", "-c", configDir, "-d", t.TempDir(), "-s", "fixture", "-m", "fixture"})
	require.ErrorContains(t, root.Execute(), "initializing download logging")
	require.False(t, selected)
}

func TestDownloadRetainsSelectionFailure(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "monitor.log")
	configDir := writeDownloadConfig(t, fmt.Sprintf("logPath: %q\n", logPath))
	deps := defaultDependencies()
	deps.selectSource = func(domain.MonitoredManga) (domain.Source, error) { return &configuredDownloadSource{}, nil }
	root := newRootCommand(deps)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"download", "-c", configDir, "-d", t.TempDir(), "-s", "fixture", "-m", "fixture", "-C", "invalid"})
	err := root.Execute()
	require.ErrorContains(t, err, "parsing chapter selection")
	logs, globErr := filepath.Glob(logPath + ".downloads/*.jsonl")
	require.NoError(t, globErr)
	require.Len(t, logs, 1)
	data, readErr := os.ReadFile(logs[0])
	require.NoError(t, readErr)
	require.Contains(t, string(data), "parsing chapter selection")
}

func TestDownloadBinaryAdjacentConfigProcessHelper(t *testing.T) {
	if os.Getenv("MANGARR_TEST_BINARY_CONFIG") == "" {
		return
	}
	deps := defaultDependencies()
	failure := errors.New("binary-adjacent fixture")
	deps.selectSource = func(domain.MonitoredManga) (domain.Source, error) {
		return &configuredDownloadSource{discoveryError: failure}, nil
	}
	root := newRootCommand(deps)
	root.SetArgs([]string{"download", "-d", t.TempDir(), "-s", "fixture", "-m", "fixture"})
	require.ErrorIs(t, root.Execute(), failure)
}

func TestDownloadBinaryAdjacentConfig(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "monitor.log")
	configDir := writeDownloadConfig(t, fmt.Sprintf("logPath: %q\n", logPath))
	if err := os.Symlink(filepath.Join(configDir, "config.yaml"), filepath.Join(root, "config.yaml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	executable, err := os.Executable()
	require.NoError(t, err)
	data, err := os.ReadFile(executable)
	require.NoError(t, err)
	copyPath := filepath.Join(root, filepath.Base(executable))
	require.NoError(t, os.WriteFile(copyPath, data, 0o700))
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, copyPath, "-test.run=^TestDownloadBinaryAdjacentConfigProcessHelper$")
	command.Env = append(os.Environ(), "MANGARR_TEST_BINARY_CONFIG=1")
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
	logs, err := filepath.Glob(logPath + ".downloads/*.jsonl")
	require.NoError(t, err)
	require.Len(t, logs, 1, string(output))
	retained, err := os.ReadFile(logs[0])
	require.NoError(t, err)
	require.Contains(t, string(retained), "binary-adjacent fixture")
	require.NoFileExists(t, logPath)
}
