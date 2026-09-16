package logger

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"mangarr/internal/domain"

	"github.com/stretchr/testify/require"
)

func downloadConfig(t *testing.T) domain.Config {
	t.Helper()
	return domain.Config{Version: "v1.0.0", LogPath: filepath.Join(t.TempDir(), "monitor.log"), LogLevel: "DEBUG", LogMaxSize: 1, LogMaxBackups: 2}
}

func runLogs(t *testing.T, cfg domain.Config) []string {
	t.Helper()
	paths, err := filepath.Glob(cfg.LogPath + ".downloads/download-*.jsonl")
	require.NoError(t, err)
	return paths
}

func TestDownloadConsoleAndFileLevelsStaySeparate(t *testing.T) {
	for _, version := range []string{"dev", "v1.0.0"} {
		t.Run(version, func(t *testing.T) {
			cfg := downloadConfig(t)
			cfg.Version, cfg.LogLevel = version, "WARN"
			var console bytes.Buffer
			logging, err := NewDownload(cfg, &console)
			require.NoError(t, err)
			logging.Log.Info().Msg("visible in terminal")
			logging.Log.Warn().Msg("retained warning")
			require.NoError(t, logging.Close(nil))
			require.Contains(t, console.String(), "visible in terminal")
			require.Contains(t, console.String(), "INF")
			require.NotContains(t, console.String(), `"message":`)
			paths := runLogs(t, cfg)
			require.Len(t, paths, 1)
			require.Contains(t, console.String(), paths[0])
			data, err := os.ReadFile(paths[0])
			require.NoError(t, err)
			require.NotContains(t, string(data), "visible in terminal")
			require.Contains(t, string(data), "retained warning")
			for _, line := range bytes.Split(bytes.TrimSpace(data), []byte{'\n'}) {
				require.True(t, json.Valid(line), string(line))
			}
			if runtime.GOOS != "windows" {
				info, err := os.Stat(paths[0])
				require.NoError(t, err)
				require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
			}
			require.NoFileExists(t, cfg.LogPath)
		})
	}
}

func TestDownloadConsoleOnly(t *testing.T) {
	var console bytes.Buffer
	logging, err := NewDownload(domain.Config{}, &console)
	require.NoError(t, err)
	logging.Log.Info().Msg("console only")
	failure := errors.New("returned to main")
	require.ErrorIs(t, logging.Close(failure), failure)
	require.Contains(t, console.String(), "console only")
	require.NotContains(t, console.String(), "Download log:")
	require.NotContains(t, console.String(), failure.Error())
}

func TestDownloadLogInitializationFailure(t *testing.T) {
	cfg := downloadConfig(t)
	require.NoError(t, os.WriteFile(cfg.LogPath+".downloads", []byte("unrelated file"), 0o600))
	_, err := NewDownload(cfg, io.Discard)
	require.ErrorContains(t, err, "initializing download logging")
	data, err := os.ReadFile(cfg.LogPath + ".downloads")
	require.NoError(t, err)
	require.Equal(t, "unrelated file", string(data))
}

func TestDownloadLogRejectsDirectoryDestination(t *testing.T) {
	for _, path := range []string{t.TempDir(), filepath.Join(t.TempDir(), "missing") + string(os.PathSeparator)} {
		cfg := downloadConfig(t)
		cfg.LogPath = path
		_, err := NewDownload(cfg, io.Discard)
		require.ErrorContains(t, err, "directory")
		require.NoDirExists(t, path+".downloads")
	}
}

func TestDownloadLogWriteFailurePreservesConsoleAndCommandError(t *testing.T) {
	cfg := downloadConfig(t)
	var console bytes.Buffer
	logging, err := NewDownload(cfg, &console)
	require.NoError(t, err)
	require.NoError(t, logging.run.file.Close())
	logging.Log.Error().Msg("first chapter failure")
	logging.Log.Info().Msg("another chapter completed")
	failure := errors.New("failed to download chapters: 1")
	err = logging.Close(failure)
	require.ErrorIs(t, err, failure)
	require.ErrorContains(t, err, "is incomplete")
	require.Contains(t, console.String(), "first chapter failure")
	require.Contains(t, console.String(), "another chapter completed")
	require.Equal(t, 1, strings.Count(console.String(), "File logging failed"))
}

func TestDownloadLogSizeBound(t *testing.T) {
	cfg := downloadConfig(t)
	logging, err := NewDownload(cfg, io.Discard)
	require.NoError(t, err)
	logging.Log.Info().Msg("retained before limit")
	logging.Log.Info().Msg(strings.Repeat("x", 1024*1024))
	logging.Log.Error().Msg("terminal only after limit")
	require.ErrorIs(t, logging.Close(nil), errLogLimit)
	data, err := os.ReadFile(runLogs(t, cfg)[0])
	require.NoError(t, err)
	require.LessOrEqual(t, len(data), 1024*1024)
	require.Contains(t, string(data), "retained before limit")
	require.Contains(t, string(data), "size limit reached")
	require.NotContains(t, string(data), "terminal only after limit")
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte{'\n'}) {
		require.True(t, json.Valid(line))
	}
}

func TestDownloadPersistedURLRedaction(t *testing.T) {
	cfg := downloadConfig(t)
	logging, err := NewDownload(cfg, io.Discard)
	require.NoError(t, err)
	logging.Log.Warn().Interface("details", []any{
		map[string]any{"url": "https://name:password@example.invalid/page?token=secret#fragment"},
		"http://bad:password@example.invalid/%zz?secret=yes",
		"ftp://name:password@example.invalid/file?secret=yes",
		"https://example.invalid/gist?label=O'Reilly&token=secret",
	}).Msg("requesting https://example.invalid/series?signature=secret")
	failure := errors.New(`Get "https://name:password@example.invalid/gist?label=O'Reilly&token=secret": offline failure`)
	require.ErrorIs(t, logging.Close(failure), failure)
	data, err := os.ReadFile(runLogs(t, cfg)[0])
	require.NoError(t, err)
	for _, secret := range []string{"name:", "password", "token=", "secret", "fragment", "signature=", "label=", "Reilly"} {
		require.NotContains(t, string(data), secret)
	}
	require.Contains(t, string(data), "https://example.invalid/page")
	require.Contains(t, string(data), "https://example.invalid/gist")
	require.Contains(t, string(data), "[redacted URL]")
	require.Contains(t, string(data), "Download failed")
}

func TestDownloadPersistedURLQueryRedaction(t *testing.T) {
	for _, test := range []struct {
		name, suffix string
	}{
		{name: "normal URL"},
		{name: "normal query", suffix: "?token=secret"},
		{name: "apostrophe", suffix: "?label=O'Reilly&token=secret"},
		{name: "double quotes", suffix: `?label="title"&token=secret`},
		{name: "space and punctuation", suffix: "?label=a b<>\"'&token=secret"},
		{name: "fragment", suffix: "#secret"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := downloadConfig(t)
			var console bytes.Buffer
			logging, err := NewDownload(cfg, &console)
			require.NoError(t, err)
			base := "https://example.invalid/gist"
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, base+test.suffix, nil)
			require.NoError(t, err)
			failure := &url.Error{Op: "Get", URL: req.URL.String(), Err: errors.New("offline failure")}
			logging.Log.Warn().Str("url", req.URL.String()).Interface("details", []any{req.URL.String()}).Msg("requesting " + req.URL.String())
			logging.Log.Error().Err(failure).Msg("request failed")
			require.ErrorIs(t, logging.Close(failure), failure)
			require.Contains(t, console.String(), "offline failure")
			if test.suffix != "" {
				require.Contains(t, console.String(), "secret")
			}
			data, err := os.ReadFile(runLogs(t, cfg)[0])
			require.NoError(t, err)
			wantURL := base
			wantError := failure.Error()
			if test.suffix != "" {
				wantURL += " [redacted URL suffix]"
				wantError = `Get "` + wantURL
			}
			scanner := bufio.NewScanner(bytes.NewReader(data))
			var records []map[string]any
			for scanner.Scan() {
				var record map[string]any
				require.NoError(t, json.Unmarshal(scanner.Bytes(), &record))
				records = append(records, record)
			}
			require.NoError(t, scanner.Err())
			require.Len(t, records, 4)
			require.Equal(t, wantURL, records[1]["url"])
			require.Equal(t, []any{wantURL}, records[1]["details"])
			require.Equal(t, "requesting "+wantURL, records[1]["message"])
			require.Equal(t, wantError, records[2]["error"])
			require.Equal(t, wantError, records[3]["error"])
		})
	}
}

func TestDownloadRetentionPreservesActiveAndUnrelatedFiles(t *testing.T) {
	cfg := downloadConfig(t)
	active, err := NewDownload(cfg, io.Discard)
	require.NoError(t, err)
	defer active.Close(nil)
	activePath := active.run.file.Name()
	unrelated := filepath.Join(cfg.LogPath+".downloads", "notes.txt")
	for _, path := range []string{cfg.LogPath, cfg.LogPath + ".backup", unrelated} {
		require.NoError(t, os.WriteFile(path, []byte("keep me"), 0o600))
	}
	for range 6 {
		logging, err := NewDownload(cfg, io.Discard)
		require.NoError(t, err)
		logging.Log.Info().Msg("completed run")
		require.NoError(t, logging.Close(nil))
	}
	require.Len(t, runLogs(t, cfg), cfg.LogMaxBackups+1)
	active.Log.Info().Msg("still active")
	data, err := os.ReadFile(activePath)
	require.NoError(t, err)
	require.Contains(t, string(data), "still active")
	for _, path := range []string{cfg.LogPath, cfg.LogPath + ".backup", unrelated} {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "keep me", string(data))
	}
}

type closedConsole struct{}

func (closedConsole) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestDownloadRetainsDiagnosticsAfterConsoleCloses(t *testing.T) {
	cfg := downloadConfig(t)
	logging, err := NewDownload(cfg, closedConsole{})
	require.NoError(t, err)
	logging.Log.Error().Msg("retained without a terminal")
	require.NoError(t, logging.Close(nil))
	data, err := os.ReadFile(runLogs(t, cfg)[0])
	require.NoError(t, err)
	require.Contains(t, string(data), "retained without a terminal")
}

func TestDownloadRetentionDoesNotFollowSymlinks(t *testing.T) {
	cfg := downloadConfig(t)
	cfg.LogMaxBackups = 1
	logging, err := NewDownload(cfg, io.Discard)
	require.NoError(t, err)
	require.NoError(t, logging.Close(nil))
	target := filepath.Join(t.TempDir(), "unrelated.log")
	require.NoError(t, os.WriteFile(target, []byte("untouched"), 0o600))
	link := filepath.Join(cfg.LogPath+".downloads", "download-20000101T000000.000000000Z-0123456789abcdef0123456789abcdef.jsonl")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	for range 3 {
		logging, err := NewDownload(cfg, io.Discard)
		require.NoError(t, err)
		require.NoError(t, logging.Close(nil))
	}
	info, err := os.Lstat(link)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeSymlink)
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "untouched", string(data))
}

func TestDownloadLogProcessHelper(t *testing.T) {
	path := os.Getenv("MANGARR_TEST_LOG_PROCESS")
	if path == "" {
		return
	}
	cfg := domain.Config{LogPath: path, LogLevel: "DEBUG", LogMaxSize: 1, LogMaxBackups: 1}
	logging, err := NewDownload(cfg, io.Discard)
	require.NoError(t, err)
	fmt.Println(logging.run.file.Name())
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		switch scanner.Text() {
		case "crash":
			os.Exit(0) // Deliberately bypass Close to test kernel lock release.
		case "write":
			logging.Log.Info().Msg("active subprocess record")
			fmt.Println("written")
		case "close":
			require.NoError(t, logging.Close(nil))
			return
		}
	}
	require.NoError(t, scanner.Err())
	require.NoError(t, logging.Close(nil))
}

func TestDownloadLogConcurrentProcesses(t *testing.T) {
	cfg := downloadConfig(t)
	cfg.LogMaxBackups = 1
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	type child struct {
		cmd    *exec.Cmd
		input  io.WriteCloser
		output *bufio.Scanner
		path   string
	}
	children := make([]child, 3)
	executable, err := os.Executable()
	require.NoError(t, err)
	for i := range children {
		cmd := exec.CommandContext(ctx, executable, "-test.run=^TestDownloadLogProcessHelper$")
		cmd.Env = append(os.Environ(), "MANGARR_TEST_LOG_PROCESS="+cfg.LogPath)
		cmd.Stderr = os.Stderr
		input, err := cmd.StdinPipe()
		require.NoError(t, err)
		output, err := cmd.StdoutPipe()
		require.NoError(t, err)
		require.NoError(t, cmd.Start())
		children[i] = child{cmd: cmd, input: input, output: bufio.NewScanner(output)}
		t.Cleanup(func() {
			_ = input.Close()
			if cmd.ProcessState == nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		})
	}
	paths := make(map[string]bool)
	for i := range children {
		c := &children[i]
		require.True(t, c.output.Scan(), "child failed to initialize")
		c.path = c.output.Text()
		require.FileExists(t, c.path)
		require.False(t, paths[c.path], "run names must not collide")
		paths[c.path] = true
	}

	for range 4 {
		logging, err := NewDownload(cfg, io.Discard)
		require.NoError(t, err)
		require.NoError(t, logging.Close(nil))
	}
	require.Len(t, runLogs(t, cfg), len(children)+1)
	for _, c := range children {
		_, err := fmt.Fprintln(c.input, "write")
		require.NoError(t, err)
		require.True(t, c.output.Scan())
		require.Equal(t, "written", c.output.Text())
		data, err := os.ReadFile(c.path)
		require.NoError(t, err)
		require.Contains(t, string(data), "active subprocess record")
	}

	_, err = fmt.Fprintln(children[0].input, "crash")
	require.NoError(t, err)
	require.NoError(t, children[0].cmd.Wait())
	logging, err := NewDownload(cfg, io.Discard)
	require.NoError(t, err)
	require.NoError(t, logging.Close(nil))
	require.NoFileExists(t, children[0].path, "an interrupted run must not stay active forever")
	for _, c := range children[1:] {
		require.FileExists(t, c.path)
		_, err := fmt.Fprintln(c.input, "close")
		require.NoError(t, err)
		require.NoError(t, c.cmd.Wait())
	}
	require.Len(t, runLogs(t, cfg), 1)
}
