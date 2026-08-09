package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/logger"
)

func TestDefaultConfigLocationsFollowDocumentedOrder(t *testing.T) {
	t.Parallel()

	locations := defaultConfigLocations(
		filepath.Join("root", "user-config"),
		filepath.Join("root", "home"),
		filepath.Join("root", "bin", "mangarr"),
	)
	want := []string{
		filepath.Join("root", "user-config", "mangarr", "config.yaml"),
		filepath.Join("root", "home", ".mangarr", "config.yaml"),
		filepath.Join("root", "bin", "config.yaml"),
	}
	if !slices.Equal(locations, want) {
		t.Fatalf("config locations = %v, want %v", locations, want)
	}
}

func TestDefaultConfigLocationsOmitUnavailableDirectories(t *testing.T) {
	t.Parallel()

	locations := defaultConfigLocations("", "", filepath.Join("root", "bin", "mangarr"))
	want := []string{filepath.Join("root", "bin", "config.yaml")}
	if !slices.Equal(locations, want) {
		t.Fatalf("config locations = %v, want %v", locations, want)
	}
}

func TestFirstExistingConfigUsesLocationOrder(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	missing := filepath.Join(root, "missing", "config.yaml")
	first := filepath.Join(root, "first.yaml")
	second := filepath.Join(root, "second.yaml")
	if err := os.WriteFile(first, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := firstExistingConfig([]string{missing, first, second})
	if !ok || got != first {
		t.Fatalf("first existing config = %q, %v; want %q, true", got, ok, first)
	}
}

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
	t.Cleanup(func() {
		if err := cfg.watcher.Close(); err != nil {
			t.Errorf("close config watcher: %v", err)
		}
	})

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

func TestDynamicReloadSurvivesRenameAndRecreateSave(t *testing.T) {
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
	t.Cleanup(func() {
		if err := cfg.watcher.Close(); err != nil {
			t.Errorf("close config watcher: %v", err)
		}
	})

	configFile := filepath.Join(dir, "config.yaml")
	if err := os.Rename(configFile, configFile+".backup"); err != nil {
		t.Fatalf("rename config: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	writeTestConfig(t, dir, `downloadLocation: "`+dir+`"
checkInterval: 2
`)
	select {
	case <-reloads:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for reload after rename-and-recreate save")
	}
	if got := cfg.Snapshot().CheckInterval; got != 2 {
		t.Fatalf("check interval = %s, want 2", got)
	}

	writeTestConfig(t, dir, `downloadLocation: "`+dir+`"
checkInterval: 3
`)
	select {
	case <-reloads:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for reload after the next save")
	}
	if got := cfg.Snapshot().CheckInterval; got != 3 {
		t.Fatalf("check interval = %s, want 3", got)
	}
}

func TestDynamicReloadDebouncesIncompleteWrites(t *testing.T) {
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
	t.Cleanup(func() {
		if err := cfg.watcher.Close(); err != nil {
			t.Errorf("close config watcher: %v", err)
		}
	})

	configFile := filepath.Join(dir, "config.yaml")
	writer, err := os.OpenFile(configFile, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		t.Fatalf("truncate config: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close truncated config: %v", err)
	}
	time.Sleep(configReloadDebounceDelay / 4)

	select {
	case <-reloads:
		t.Fatal("published a reload while the config file was incomplete")
	case <-time.After(configReloadDebounceDelay / 2):
	}
	if got := cfg.Snapshot().CheckInterval; got != 15 {
		t.Fatalf("check interval = %s, want previous value 15", got)
	}

	writeTestConfig(t, dir, `downloadLocation: "`+dir+`"
checkInterval: 2
`)
	select {
	case <-reloads:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for completed config reload")
	}
	time.Sleep(2 * configReloadDebounceDelay)
	select {
	case <-reloads:
		t.Fatal("published a duplicate reload for one completed save")
	default:
	}
	if got := cfg.Snapshot().CheckInterval; got != 2 {
		t.Fatalf("check interval = %s, want 2", got)
	}
}

func TestDynamicReloadFollowsReplacedConfigSymlink(t *testing.T) {
	dir := t.TempDir()
	firstTarget := filepath.Join(dir, "config-first.yaml")
	secondTarget := filepath.Join(dir, "config-second.yaml")
	configFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(firstTarget, []byte(`downloadLocation: "`+dir+`"
checkInterval: 15
`), 0o644); err != nil {
		t.Fatalf("write first config target: %v", err)
	}
	if err := os.WriteFile(secondTarget, []byte(`downloadLocation: "`+dir+`"
checkInterval: 2
`), 0o644); err != nil {
		t.Fatalf("write second config target: %v", err)
	}
	if err := os.Symlink(firstTarget, configFile); err != nil {
		t.Skipf("config symlinks are unavailable: %v", err)
	}
	nextSymlink := configFile + ".next"
	if err := os.Symlink(secondTarget, nextSymlink); err != nil {
		t.Fatalf("create replacement config symlink: %v", err)
	}

	cfg := &AppConfig{configFile: configFile, version: "test"}
	snapshot, err := cfg.loadSnapshot()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.current.Store(snapshot)
	reloads, err := cfg.DynamicReload(logger.New(snapshot))
	if err != nil {
		t.Fatalf("watch config: %v", err)
	}
	t.Cleanup(func() {
		if err := cfg.watcher.Close(); err != nil {
			t.Errorf("close config watcher: %v", err)
		}
	})

	time.Sleep(configReloadDebounceDelay)
	if err := os.Rename(nextSymlink, configFile); err != nil {
		t.Fatalf("replace config symlink: %v", err)
	}
	if err := os.WriteFile(secondTarget, []byte(`downloadLocation: "`+dir+`"
checkInterval: 2
`), 0o644); err != nil {
		t.Fatalf("update replacement config target: %v", err)
	}

	select {
	case <-reloads:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for reload after config symlink replacement")
	}
	if got := cfg.Snapshot().CheckInterval; got != 2 {
		t.Fatalf("check interval = %s, want 2", got)
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
