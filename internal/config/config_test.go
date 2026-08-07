package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/logger"
)

func TestLoadRejectsInvalidCheckInterval(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, `downloadLocation: "`+dir+`"
checkInterval: 0
`)

	if _, err := Load(dir, "test"); err == nil {
		t.Fatal("expected invalid check interval to fail")
	}
}

func TestSnapshotDoesNotExposeMutableConfig(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, `downloadLocation: "`+dir+`"
checkInterval: 15
monitoredManga:
  One Piece:
    source: tcbscans
    manga: One Piece
`)

	cfg, err := Load(dir, "test")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	snapshot := cfg.Snapshot()
	snapshot.MonitoredManga["One Piece"].Manga = "changed"
	delete(snapshot.MonitoredManga, "One Piece")

	got := cfg.Snapshot()
	if got.MonitoredManga["One Piece"].Manga != "One Piece" {
		t.Fatalf("stored config changed through snapshot: %#v", got.MonitoredManga)
	}
}

func TestDynamicReloadPublishesValidatedSnapshot(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, `downloadLocation: "`+dir+`"
checkInterval: 15
`)

	cfg, err := Load(dir, "test")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	snapshot := cfg.Snapshot()
	reloads, err := cfg.DynamicReload(logger.New(&snapshot))
	if err != nil {
		t.Fatalf("watch config: %v", err)
	}

	writeTestConfig(t, dir, `downloadLocation: "`+dir+`"
checkInterval: 2
monitoredManga:
  One Piece:
    source: tcbscans
    manga: One Piece
`)

	select {
	case <-reloads:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for config reload")
	}

	got := cfg.Snapshot()
	if got.CheckInterval != 2 {
		t.Fatalf("check interval = %s, want 2", got.CheckInterval)
	}
	if got.MonitoredManga["One Piece"].Source != "tcbscans" {
		t.Fatalf("monitored manga not reloaded: %#v", got.MonitoredManga)
	}
}

func TestValidateRejectsNullMonitoredManga(t *testing.T) {
	cfg := domain.Config{
		DownloadLocation: "/tmp",
		NamingTemplate:   "{manga:<.>}",
		CheckInterval:    15,
		LogLevel:         "INFO",
		LogMaxSize:       50,
		LogMaxBackups:    3,
		MonitoredManga: map[string]*domain.MonitoredManga{
			"Broken": nil,
		},
	}

	if err := validate(cfg); err == nil {
		t.Fatal("expected null monitored manga to fail")
	}
}

func writeTestConfig(t *testing.T, dir, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
