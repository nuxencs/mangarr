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
