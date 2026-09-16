package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDownloadConfigIgnoresMonitorValidation(t *testing.T) {
	dir := t.TempDir()
	body := `logPath: monitor.log
checkInterval: 0
pprofEnabled: true
pprofAddress: ""
monitoredManga:
  Unused: null
`
	writeTestConfig(t, dir, body)
	cfg, err := LoadDownload(dir, "release", true)
	require.NoError(t, err)
	require.Equal(t, "monitor.log", cfg.LogPath)
	require.Equal(t, "DEBUG", cfg.LogLevel)
	require.Equal(t, 50, cfg.LogMaxSize)
	require.Equal(t, 3, cfg.LogMaxBackups)
	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)
	require.Equal(t, body, string(data))
	_, err = LoadExisting(dir, "release")
	require.ErrorContains(t, err, "downloadLocation")
}

func TestDownloadConfigFileSettings(t *testing.T) {
	for _, test := range []struct {
		name, body, wantError string
	}{
		{"no destination ignores logging validation", "logLevel: INVALID\nlogMaxSize: 0\nlogMaxBackups: 0\n", ""},
		{"invalid level", "logPath: monitor.log\nlogLevel: INVALID\n", "logLevel"},
		{"invalid size", "logPath: monitor.log\nlogMaxSize: 0\n", "logMaxSize"},
		{"invalid retention", "logPath: monitor.log\nlogMaxBackups: 0\n", "logMaxBackups"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTestConfig(t, dir, test.body)
			_, err := LoadDownload(dir, "test", true)
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDownloadConfigEnvironmentPrecedence(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "logPath: yaml.log\nlogLevel: DEBUG\nlogMaxSize: 50\nlogMaxBackups: 3\n")
	t.Setenv("MANGARR__LOG_PATH", "environment.log")
	t.Setenv("MANGARR__LOG_LEVEL", "warn")
	t.Setenv("MANGARR__LOG_MAX_SIZE", "2")
	t.Setenv("MANGARR__LOG_MAX_BACKUPS", "5")
	cfg, err := LoadDownload(dir, "test", true)
	require.NoError(t, err)
	require.Equal(t, "environment.log", cfg.LogPath)
	require.Equal(t, "WARN", cfg.LogLevel)
	require.Equal(t, 2, cfg.LogMaxSize)
	require.Equal(t, 5, cfg.LogMaxBackups)
}

func TestDownloadConfigDiscoveryAndMissingConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("AppData", home)
	t.Setenv("MANGARR__LOG_PATH", "")
	cfg, err := LoadDownload("", "test", false)
	require.NoError(t, err)
	require.Empty(t, cfg.LogPath)
	_, err = LoadDownload("", "test", true)
	require.ErrorIs(t, err, errConfigNotFound)

	t.Setenv("MANGARR__LOG_PATH", "environment.log")
	cfg, err = LoadDownload("", "test", false)
	require.NoError(t, err)
	require.Equal(t, "environment.log", cfg.LogPath)
	t.Setenv("MANGARR__LOG_PATH", "")

	userConfig, err := os.UserConfigDir()
	require.NoError(t, err)
	dir := filepath.Join(userConfig, "mangarr")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	writeTestConfig(t, dir, "logPath: discovered.log\n")
	cfg, err = LoadDownload("", "test", false)
	require.NoError(t, err)
	require.Equal(t, "discovered.log", cfg.LogPath)

	missing := filepath.Join(home, "missing")
	_, err = LoadDownload(missing, "test", true)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.NoDirExists(t, missing)
}

func TestBinaryAdjacentConfigSymlink(t *testing.T) {
	root := t.TempDir()
	binaryDir := filepath.Join(root, "bin")
	configDir := filepath.Join(root, "config")
	require.NoError(t, os.Mkdir(binaryDir, 0o700))
	require.NoError(t, os.Mkdir(configDir, 0o700))
	writeTestConfig(t, configDir, "logPath: monitor.log\n")
	link := filepath.Join(binaryDir, "config.yaml")
	if err := os.Symlink(filepath.Join(configDir, "config.yaml"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	locations := defaultConfigLocations("", "", filepath.Join(binaryDir, "mangarr"))
	found, ok := firstExistingConfig(locations)
	require.True(t, ok)
	require.Equal(t, link, found)
	cfg, err := LoadDownload(filepath.Dir(found), "test", true)
	require.NoError(t, err, fmt.Sprint(locations))
	require.Equal(t, "monitor.log", cfg.LogPath)
}
