package cmd

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDownloadForceReplacesOnlySelectedChapters(t *testing.T) {
	for _, tt := range []struct {
		name     string
		flags    []string
		replaced []string
	}{
		{name: "ordinary skip", flags: []string{"-C", "1"}},
		{name: "explicit force", flags: []string{"--force", "-C", "1"}, replaced: []string{"1"}},
		{name: "first force", flags: []string{"-f", "--first"}, replaced: []string{"1"}},
		{name: "default latest force", flags: []string{"--force"}, replaced: []string{"2"}},
		{name: "all force", flags: []string{"--force", "--all"}, replaced: []string{"1", "2"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var pngBytes bytes.Buffer
			require.NoError(t, png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 1, 1))))
			var imageRequests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/gist" {
					fmt.Fprintf(w, `{"title":"Fixture","chapters":{"1":{"groups":{"test":["http://%s/page"]}},"2":{"groups":{"test":["http://%s/page"]}}}}`, r.Host, r.Host)
					return
				}
				imageRequests.Add(1)
				w.Header().Set("Content-Type", "image/png")
				_, _ = w.Write(pngBytes.Bytes())
			}))
			defer server.Close()
			dir := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(dir, "Fixture"), 0o755))
			for _, number := range []string{"1", "2", "99"} {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "Fixture", number+".cbz"), []byte("old archive"), 0o644))
			}
			root := newRootCommand(defaultDependencies())
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			args := []string{"download", "-d", dir, "-s", "cubari", "-m", server.URL + "/gist", "-g", "test", "-n", "{num}"}
			root.SetArgs(append(args, tt.flags...))
			require.NoError(t, root.ExecuteContext(t.Context()))
			require.Equal(t, int32(len(tt.replaced)), imageRequests.Load())
			for _, number := range []string{"1", "2", "99"} {
				path := filepath.Join(dir, "Fixture", number+".cbz")
				replaced := false
				for _, selected := range tt.replaced {
					if selected == number {
						replaced = true
					}
				}
				if !replaced {
					data, err := os.ReadFile(path)
					require.NoError(t, err)
					require.Equal(t, "old archive", string(data))
					continue
				}
				archive, err := zip.OpenReader(path)
				require.NoError(t, err)
				require.Len(t, archive.File, 1)
				page, err := archive.File[0].Open()
				require.NoError(t, err)
				data, err := io.ReadAll(page)
				require.NoError(t, err)
				require.NoError(t, page.Close())
				require.Equal(t, pngBytes.Bytes(), data)
				require.NoError(t, archive.Close())
			}
		})
	}
}

// This is a representative throttle fixture, not a capture from a live provider.
// It exercises Cobra selection, Cubari discovery, image retries and CBZ publication.
func TestDownloadRateLimit(t *testing.T) {
	for _, tc := range []struct {
		name          string
		selector      string
		firstStatus   int
		firstFailures int32
		wantFirst     int32
		chapters      []string
		wantError     string
	}{
		{name: "all control", selector: "--all", firstStatus: http.StatusOK, wantFirst: 1, chapters: []string{"1", "2"}},
		{name: "latest control", selector: "--latest", firstStatus: http.StatusTooManyRequests, firstFailures: 1, chapters: []string{"2"}},
		{name: "all transient server error", selector: "--all", firstStatus: http.StatusServiceUnavailable, firstFailures: 1, wantFirst: 2, chapters: []string{"1", "2"}},
		{name: "all transient rate limit", selector: "--all", firstStatus: http.StatusTooManyRequests, firstFailures: 1, wantFirst: 2, chapters: []string{"1", "2"}},
		{name: "first transient rate limit", selector: "--first", firstStatus: http.StatusTooManyRequests, firstFailures: 1, wantFirst: 2, chapters: []string{"1"}},
		{name: "all persistent rate limit", selector: "--all", firstStatus: http.StatusTooManyRequests, firstFailures: 10, wantFirst: 3, chapters: []string{"2"}, wantError: "failed to download chapters: 1"},
		{name: "all permanent failure", selector: "--all", firstStatus: http.StatusNotFound, firstFailures: 10, wantFirst: 1, chapters: []string{"2"}, wantError: "failed to download chapters: 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var imageBytes bytes.Buffer
			require.NoError(t, png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 1, 1))))
			var firstAttempts atomic.Int32
			var secondAttempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/gist":
					fmt.Fprintf(w, `{"title":"Fixture","chapters":{
						"1":{"groups":{"test":["http://%s/1.png"]}},
						"2":{"groups":{"test":["http://%s/2.png"]}}
					}}`, r.Host, r.Host)
				case "/1.png", "/2.png":
					if r.URL.Path == "/1.png" {
						if firstAttempts.Add(1) <= tc.firstFailures {
							w.Header().Set("Retry-After", "0")
							w.WriteHeader(tc.firstStatus)
							return
						}
					} else {
						secondAttempts.Add(1)
					}
					w.Header().Set("Content-Type", "image/png")
					_, _ = w.Write(imageBytes.Bytes())
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			directory := t.TempDir()
			root := NewRootCommand()
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			root.SetArgs([]string{"download", "-d", directory, "-s", "cubari", "-m", server.URL + "/gist", "-g", "test", "-n", "Chapter {num}", tc.selector})
			logPath := filepath.Join(t.TempDir(), "monitor.log")
			if tc.wantError != "" {
				t.Setenv("MANGARR__LOG_PATH", logPath)
			}
			err := root.ExecuteContext(t.Context())
			if tc.wantError != "" {
				require.EqualError(t, err, tc.wantError)
				require.NoFileExists(t, filepath.Join(directory, "Fixture", "Chapter 1.cbz"))
				logs, err := filepath.Glob(logPath + ".downloads/*.jsonl")
				require.NoError(t, err)
				require.Len(t, logs, 1)
				data, err := os.ReadFile(logs[0])
				require.NoError(t, err)
				require.Contains(t, string(data), "Failed to acquire chapter 1")
				require.Contains(t, string(data), "Finished downloading")
				require.Contains(t, string(data), "Summary: downloaded=1 skipped=0 failed=1")
			} else {
				require.NoError(t, err, "transient throttling must not leave selected chapters failed")
			}
			require.Equal(t, tc.wantFirst, firstAttempts.Load())
			if tc.selector == "--first" {
				require.Zero(t, secondAttempts.Load())
			} else {
				require.Equal(t, int32(1), secondAttempts.Load())
			}
			for _, number := range tc.chapters {
				archive, err := zip.OpenReader(filepath.Join(directory, "Fixture", "Chapter "+number+".cbz"))
				require.NoError(t, err)
				require.Len(t, archive.File, 1)
				require.Equal(t, "001.png", archive.File[0].Name)
				require.NoError(t, archive.Close())
			}
		})
	}
}
