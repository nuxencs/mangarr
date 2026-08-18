package cmd

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"mangarr/internal/domain"
	"mangarr/internal/logger"
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

	runMonitorCycle(t.Context(), cfg, logger.New(&cfg))
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
