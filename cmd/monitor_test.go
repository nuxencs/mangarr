package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"mangarr/internal/domain"
	"mangarr/internal/logger"

	"github.com/stretchr/testify/require"
)

func TestRunMonitorCycleErrorIncludesConfiguredSource(t *testing.T) {
	cfg := domain.Config{
		Version:          "test",
		DownloadLocation: t.TempDir(),
		LogLevel:         "DEBUG",
		MonitoredManga: map[string]*domain.MonitoredManga{
			"Broken Manga": {
				Source: "asurascans",
			},
		},
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	originalStderr := os.Stderr
	os.Stderr = writer
	t.Cleanup(func() {
		os.Stderr = originalStderr
		_ = reader.Close()
		_ = writer.Close()
	})

	runMonitorCycle(t.Context(), cfg, logger.New(&cfg), newMonitorMarks())
	os.Stderr = originalStderr
	if err := writer.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}

	contents, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read monitor log: %v", err)
	}

	var entry map[string]any
	if err := json.Unmarshal(contents, &entry); err != nil {
		t.Fatalf("decode monitor log %q: %v", contents, err)
	}
	if got := entry["message"]; got != "monitor check failed" {
		t.Fatalf("message = %v, want monitor check failed", got)
	}
	if got := entry["manga"]; got != "Broken Manga" {
		t.Fatalf("manga = %v, want Broken Manga", got)
	}
	if got := entry["source"]; got != "asurascans" {
		t.Fatalf("source = %v, want asurascans", got)
	}
}

func TestMonitorSelection(t *testing.T) {
	numbers := func(raw ...string) []domain.ChapterNumber {
		out := make([]domain.ChapterNumber, 0, len(raw))
		for _, r := range raw {
			out = append(out, mustChapterNumber(t, r))
		}
		return out
	}
	sorted := numbers("99", "100", "100.5", "101", "102")

	for _, tt := range []struct {
		name    string
		mark    string
		hasMark bool
		want    []domain.ChapterNumber
	}{
		{name: "first check selects latest only", want: numbers("102")},
		{name: "selects every chapter above the mark", mark: "100", hasMark: true, want: numbers("100.5", "101", "102")},
		{name: "mark at latest selects latest", mark: "102", hasMark: true, want: numbers("102")},
		{name: "mark above latest selects latest", mark: "9999", hasMark: true, want: numbers("102")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var mark domain.ChapterNumber
			if tt.hasMark {
				mark = mustChapterNumber(t, tt.mark)
			}
			require.Equal(t, tt.want, monitorSelection(sorted, mark, tt.hasMark))
		})
	}
}

func TestMonitorMarksResetOnIdentityChange(t *testing.T) {
	marks := newMonitorMarks()
	entry := domain.MonitoredManga{Source: "cubari", Manga: "fixture", Group: "test", Language: "en"}
	marks.set("Fixture", entry, mustChapterNumber(t, "10"))

	renamed := entry
	renamed.Overwrite = "Other Title"
	mark, ok := marks.get("Fixture", renamed)
	require.True(t, ok, "overwrite does not change chapter numbering")
	require.Equal(t, mustChapterNumber(t, "10"), mark)

	moved := entry
	moved.Source = "mangadex"
	_, ok = marks.get("Fixture", moved)
	require.False(t, ok, "a new source must reset the mark")

	marks.prune(map[string]*domain.MonitoredManga{"Other": &entry})
	_, ok = marks.get("Fixture", entry)
	require.False(t, ok, "removed series must lose the mark")
}

// Regression: monitor used to download only the latest chapter, so a second chapter
// released in the same check interval was never downloaded.
func TestMonitorDownloadsEveryNewChapter(t *testing.T) {
	var pngBytes bytes.Buffer
	require.NoError(t, png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 1, 1))))

	var mu sync.Mutex
	chapters := []string{"1", "2"}
	broken := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path == "/gist" {
			entries := make([]string, 0, len(chapters))
			for _, number := range chapters {
				entries = append(entries, fmt.Sprintf(`%q:{"groups":{"test":["http://%s/page/%s"]}}`, number, r.Host, number))
			}
			fmt.Fprintf(w, `{"title":"Fixture","chapters":{%s}}`, strings.Join(entries, ","))
			return
		}
		if broken[strings.TrimPrefix(r.URL.Path, "/page/")] {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes.Bytes())
	}))
	defer server.Close()

	dir := t.TempDir()
	entry := &domain.MonitoredManga{Source: "cubari", Manga: server.URL + "/gist", Group: "test"}
	cfg := domain.Config{
		Version:          "test",
		DownloadLocation: dir,
		NamingTemplate:   "{num}",
		LogLevel:         "ERROR",
		MonitoredManga:   map[string]*domain.MonitoredManga{"Fixture": entry},
	}
	log := logger.New(&cfg)
	marks := newMonitorMarks()
	check := func() error { return monitorManga(t.Context(), cfg, "Fixture", *entry, log, marks) }
	archives := func() []string {
		entries, err := os.ReadDir(filepath.Join(dir, "Fixture"))
		require.NoError(t, err)
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		return names
	}
	release := func(numbers ...string) {
		mu.Lock()
		defer mu.Unlock()
		chapters = append(chapters, numbers...)
	}

	require.NoError(t, check())
	require.Equal(t, []string{"2.cbz"}, archives(), "first check keeps latest-only behavior")

	release("3", "4")
	require.NoError(t, check())
	require.Equal(t, []string{"2.cbz", "3.cbz", "4.cbz"}, archives())

	mu.Lock()
	broken["5"] = true
	mu.Unlock()
	release("5", "6")
	require.ErrorContains(t, check(), "downloading chapters 5")
	require.Equal(t, []string{"2.cbz", "3.cbz", "4.cbz", "6.cbz"}, archives())

	mu.Lock()
	broken["5"] = false
	mu.Unlock()
	require.NoError(t, check())
	require.Equal(t, []string{"2.cbz", "3.cbz", "4.cbz", "5.cbz", "6.cbz"}, archives(), "failed chapter is retried on the next check")
}

func mustChapterNumber(t *testing.T, raw string) domain.ChapterNumber {
	t.Helper()
	number, err := domain.ParseChapterNumber(raw)
	require.NoError(t, err)
	return number
}
