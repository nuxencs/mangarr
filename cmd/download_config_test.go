package cmd

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"mangarr/internal/config"
	"mangarr/internal/domain"
	"mangarr/internal/source"

	"github.com/stretchr/testify/require"
)

func TestDownloadConfiguredSeriesCreatesArchives(t *testing.T) {
	var page bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 20, 30))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	require.NoError(t, png.Encode(&page, img))
	var pageRequests atomic.Int64
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()
	mux.HandleFunc("GET /series", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"title":"Original Title","chapters":{
			"1":{"groups":{"Configured Group":[%q]}},
			"2":{"groups":{"Configured Group":[%q]}},
			"3":{"groups":{"Configured Group":[%q]}},
			"4":{"groups":{"Other Group":[%q]}}
		}}`, server.URL+"/page/1", server.URL+"/page/2", server.URL+"/page/3", server.URL+"/wrong-group")
	})
	mux.HandleFunc("GET /page/{number}", func(w http.ResponseWriter, r *http.Request) {
		pageRequests.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(page.Bytes())
	})
	destination := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "monitor.log")
	configDir := writeDownloadConfig(t, fmt.Sprintf(`downloadLocation: %q
logPath: %q
namingTemplate: "{manga:<.>}-{num}"
monitoredManga:
  My Series:
    source: cubari
    manga: %q
    group: Configured Group
    overwrite: Saved Title
`, destination, logPath, server.URL+"/series"))

	for range 2 {
		root := NewRootCommand()
		root.SetArgs([]string{"download", "-c", configDir, "--series", "My Series", "-C", "1-2"})
		require.NoError(t, root.ExecuteContext(t.Context()))
	}
	logs, err := filepath.Glob(logPath + ".downloads/*.jsonl")
	require.NoError(t, err)
	require.Len(t, logs, 2)
	firstRun, err := os.ReadFile(logs[0])
	require.NoError(t, err)
	require.Contains(t, string(firstRun), "Finished downloading")
	secondRun, err := os.ReadFile(logs[1])
	require.NoError(t, err)
	require.Contains(t, string(secondRun), "skipping")
	require.Contains(t, string(secondRun), "Summary: downloaded=0 skipped=2 failed=0")
	require.EqualValues(t, 2, pageRequests.Load(), "existing archives must skip page downloads")
	archives, err := filepath.Glob(filepath.Join(destination, "*", "*.cbz"))
	require.NoError(t, err)
	require.Equal(t, []string{
		filepath.Join(destination, "Saved Title", "Saved Title-1.cbz"),
		filepath.Join(destination, "Saved Title", "Saved Title-2.cbz"),
	}, archives)
	for _, path := range archives {
		archive, err := zip.OpenReader(path)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, archive.Close()) })
		require.Len(t, archive.File, 1)
		require.Equal(t, "001.png", archive.File[0].Name)
		reader, err := archive.File[0].Open()
		require.NoError(t, err)
		data, err := io.ReadAll(reader)
		require.NoError(t, reader.Close())
		require.NoError(t, err)
		require.Equal(t, page.Bytes(), data)
	}
}

func writeDownloadConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o600))
	return dir
}

func TestDownloadConfiguredSeriesChapterRange(t *testing.T) {
	destination := t.TempDir()
	configDir := writeDownloadConfig(t, fmt.Sprintf(`downloadLocation: %q
namingTemplate: "{manga:<.>}-{num}"
monitoredManga:
  My Series:
    source: cubari
    manga: https://git.io/OPM
    group: /r/OnePunchMan
    language: ja
    overwrite: Saved Title
`, destination))

	// Only requested chapters exist. Selecting chapter 3 or ignoring configured
	// naming/title/destination would call Pages and fail instead of skipping.
	require.NoError(t, os.Mkdir(filepath.Join(destination, "Saved Title"), 0o700))
	for _, number := range []string{"1", "2"} {
		require.NoError(t, os.WriteFile(filepath.Join(destination, "Saved Title", "Saved Title-"+number+".cbz"), []byte("existing"), 0o600))
	}
	fake := &configuredDownloadSource{}
	deps := defaultDependencies()
	deps.selectSource = func(input domain.MonitoredManga) (domain.Source, error) {
		require.Equal(t, domain.MonitoredManga{Source: "cubari", Manga: "https://git.io/OPM", Group: "/r/OnePunchMan", Language: "ja"}, input)
		adapter, err := source.Select(input)
		require.NoError(t, err)
		require.NoError(t, adapter.ValidateInput())
		return fake, nil
	}
	root := newRootCommand(deps)
	root.SetArgs([]string{"download", "-c", configDir, "--series", "My Series", "-C", "1-2"})
	require.NoError(t, root.Execute())
	require.True(t, fake.discovered)
}

func TestDownloadConfiguredSeriesSourceInputs(t *testing.T) {
	for _, test := range []struct {
		name, entry string
		flags       []string
		want        domain.MonitoredManga
	}{
		{
			name:  "MangaDex IDs and default language",
			entry: "source: mangadex\n    manga: d8f1d7da-8bb1-407b-8be3-10ac2894d3c6\n    group: 310361d7-52dd-4848-9b36-2eb4fcc95e83",
			want:  domain.MonitoredManga{Source: "mangadex", Manga: "d8f1d7da-8bb1-407b-8be3-10ac2894d3c6", Group: "310361d7-52dd-4848-9b36-2eb4fcc95e83", Language: "en"},
		},
		{
			name:  "explicit inputs override entry",
			entry: "source: cubari\n    manga: https://git.io/OPM\n    group: /r/OnePunchMan\n    language: ja",
			flags: []string{"-s", "mangaplus", "-m", "100274", "-g", "", "-l", "en"},
			want:  domain.MonitoredManga{Source: "mangaplus", Manga: "100274", Language: "en"},
		},
		{
			name:  "CLI supplies missing entry fields",
			entry: "overwrite: Saved Title",
			flags: []string{"-s", "tcbscans", "-m", "One Piece"},
			want:  domain.MonitoredManga{Source: "tcbscans", Manga: "One Piece", Language: "en"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			configDir := writeDownloadConfig(t, fmt.Sprintf("downloadLocation: %q\nmonitoredManga:\n  My Series:\n    %s\n", t.TempDir(), test.entry))
			fake := &configuredDownloadSource{discoveryError: errors.New("offline discovery reached")}
			deps := defaultDependencies()
			deps.selectSource = func(input domain.MonitoredManga) (domain.Source, error) {
				require.Equal(t, test.want, input)
				adapter, err := source.Select(input)
				require.NoError(t, err)
				require.NoError(t, adapter.ValidateInput())
				return fake, nil
			}
			root := newRootCommand(deps)
			args := []string{"download", "-c", configDir, "--series", "My Series", "-C", "1-3"}
			root.SetArgs(append(args, test.flags...))
			require.ErrorIs(t, root.Execute(), fake.discoveryError)
			require.True(t, fake.discovered)
		})
	}
}

func TestDownloadConfiguredSeriesOverrides(t *testing.T) {
	configDir := writeDownloadConfig(t, `downloadLocation: /config-downloads
namingTemplate: configured
monitoredManga:
  My Series:
    source: cubari
    manga: https://git.io/OPM
    group: configured-group
    language: ja
    overwrite: configured-title
`)
	t.Setenv("MANGARR__DOWNLOAD_LOCATION", "/environment-downloads")
	t.Setenv("MANGARR__NAMING_TEMPLATE", "environment-naming")
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			options := &downloadOptions{}
			command := newDownloadCommand(options, &rootOptions{}, nil)
			initDownloadFlags(command, options)
			args := []string{"--series", "My Series", "-C", "1-3"}
			if explicit {
				args = append(args, "-s", "mangadex", "-m", "cli-id", "-g", "", "-l", "en", "-o", "", "-d", "/cli-downloads", "-n", "cli-naming")
			}
			require.NoError(t, command.ParseFlags(args))
			cfg, err := config.LoadExisting(configDir, "test")
			require.NoError(t, err)
			require.NoError(t, resolveDownloadOptions(command, cfg.Snapshot(), options))
			require.Equal(t, "1-3", options.chapterNumbers)
			if explicit {
				require.Equal(t, "mangadex", options.mangaSource)
				require.Equal(t, "cli-id", options.manga)
				require.Empty(t, options.group)
				require.Empty(t, options.overwrite)
				require.Equal(t, "en", options.language)
				require.Equal(t, "/cli-downloads", options.downloadDirectory)
				require.Equal(t, "cli-naming", options.naming)
			} else {
				require.Equal(t, "/environment-downloads", options.downloadDirectory)
				require.Equal(t, "environment-naming", options.naming)
			}
		})
	}
}

func TestDownloadConfiguredSeriesInvalidInput(t *testing.T) {
	for _, test := range []struct{ name, entry, selected, want string }{
		{"unknown", "source: cubari\n    manga: https://git.io/OPM", "Unknown", `unknown configured series "Unknown"`},
		{"case sensitive", "source: cubari", "my series", `unknown configured series "my series"`},
		{"empty name", "source: cubari", "", "--series requires"},
		{"missing source", "manga: https://git.io/OPM", "My Series", "source is required"},
		{"missing manga", "source: cubari", "My Series", "manga is required"},
		{"missing group", "source: cubari\n    manga: https://git.io/OPM", "My Series", "group"},
		{"unknown source", "source: nonexistent\n    manga: something", "My Series", "unknown monitored manga source"},
		{"null entry", "", "My Series", "cannot be null"},
	} {
		t.Run(test.name, func(t *testing.T) {
			configDir := writeDownloadConfig(t, fmt.Sprintf("downloadLocation: %q\nmonitoredManga:\n  My Series:\n    %s\n", t.TempDir(), test.entry))
			deps := defaultDependencies()
			deps.selectSource = func(input domain.MonitoredManga) (domain.Source, error) {
				adapter, err := source.Select(input)
				if err != nil {
					return nil, err
				}
				return &configuredDownloadSource{validationError: adapter.ValidateInput()}, nil
			}
			root := newRootCommand(deps)
			root.SetErr(io.Discard)
			root.SetArgs([]string{"download", "-c", configDir, "--series", test.selected, "-C", "1-2"})
			err := root.Execute()
			require.ErrorContains(t, err, test.want)
			require.NotContains(t, err.Error(), "getting manga", "invalid inputs must not reach discovery")
		})
	}
}

func TestDownloadConfiguredSeriesRequiresExistingConfig(t *testing.T) {
	for _, withEnvironment := range []bool{false, true} {
		for _, missingDirectory := range []bool{false, true} {
			t.Run(fmt.Sprintf("environment=%t/missingDirectory=%t", withEnvironment, missingDirectory), func(t *testing.T) {
				destination := t.TempDir()
				t.Setenv("MANGARR__DOWNLOAD_LOCATION", "")
				if withEnvironment {
					t.Setenv("MANGARR__DOWNLOAD_LOCATION", destination)
				}
				configDir := t.TempDir()
				if missingDirectory {
					configDir = filepath.Join(configDir, "typo")
				}
				selected := false
				deps := defaultDependencies()
				deps.selectSource = func(domain.MonitoredManga) (domain.Source, error) {
					selected = true
					return nil, errors.New("unexpected source selection")
				}
				root := newRootCommand(deps)
				root.SetErr(io.Discard)
				root.SetArgs([]string{"download", "-c", configDir, "--series", "One Piece", "-C", "1"})
				err := root.Execute()
				require.ErrorIs(t, err, os.ErrNotExist)
				require.ErrorContains(t, err, `loading config for series "One Piece"`)
				require.ErrorContains(t, err, filepath.Join(configDir, "config.yaml"))
				require.False(t, selected)
				require.NoFileExists(t, filepath.Join(configDir, "config.yaml"))
				if missingDirectory {
					require.NoDirExists(t, configDir)
				}
			})
		}
	}
}

func TestDownloadExplicitInputsDoNotUseSeriesDefaults(t *testing.T) {
	configDir := writeDownloadConfig(t, "downloadLocation: unused\ncheckInterval: 0\nmonitoredManga:\n  Unused: null\n")
	fake := &configuredDownloadSource{discoveryError: errors.New("offline discovery reached")}
	deps := defaultDependencies()
	deps.selectSource = func(input domain.MonitoredManga) (domain.Source, error) {
		require.Equal(t, domain.MonitoredManga{Source: "tcbscans", Manga: "One Piece", Language: "en"}, input)
		return fake, nil
	}
	root := newRootCommand(deps)
	root.SetArgs([]string{"download", "-c", configDir, "-d", t.TempDir(), "-s", "tcbscans", "-m", "One Piece", "-C", "1-3"})
	require.ErrorIs(t, root.Execute(), fake.discoveryError)
	require.True(t, fake.discovered)
}

func TestDownloadExplicitInputsStillRequired(t *testing.T) {
	for _, flag := range []string{"-d", "-s", "-m"} {
		args := []string{"download"}
		for _, pair := range [][2]string{{"-d", t.TempDir()}, {"-s", "tcbscans"}, {"-m", "One Piece"}} {
			if pair[0] != flag {
				args = append(args, pair[:]...)
			}
		}
		root := NewRootCommand()
		root.SetErr(io.Discard)
		root.SetArgs(args)
		require.ErrorContains(t, root.Execute(), "at least one of the flags")
	}
}

type configuredDownloadSource struct {
	validationError error
	discoveryError  error
	discovered      bool
}

func (*configuredDownloadSource) String() string         { return "offline source" }
func (s *configuredDownloadSource) ValidateInput() error { return s.validationError }
func (s *configuredDownloadSource) Discover(context.Context) (domain.Manga, error) {
	s.discovered = true
	manga := domain.Manga{Title: "Original Title", Chapters: make(map[domain.ChapterNumber]domain.Chapter)}
	for _, raw := range strings.Fields("1 2 3") {
		number, _ := domain.ParseChapterNumber(raw)
		manga.Chapters[number] = domain.Chapter{Number: number}
	}
	return manga, s.discoveryError
}

func (*configuredDownloadSource) Pages(context.Context, domain.Chapter) ([]domain.ImageInfo, error) {
	return nil, errors.New("unexpected page resolution")
}
