package acquire

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"mangarr/internal/domain"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestChapterDownloadsAndThenSkipsExistingArchive(t *testing.T) {
	t.Parallel()

	var imageBytes bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.Black)
	require.NoError(t, png.Encode(&imageBytes, img))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes.Bytes())
	}))
	defer server.Close()

	source := &pageSource{pages: []domain.ImageInfo{{ImageURL: server.URL}}}
	request := Request{
		Source: source,
		Manga: domain.Manga{
			Title: "Original Title",
		},
		Chapter: domain.Chapter{
			Number: mustChapterNumber("7.1"),
			Title:  "The Chapter",
		},
		DownloadDirectory: t.TempDir(),
		NamingTemplate:    "{manga:<.>} Ch. {num}{title: - <.>}",
		TitleOverride:     "Replacement: Title",
	}

	result, err := Chapter(t.Context(), zerolog.Nop(), request)
	require.NoError(t, err)
	require.Equal(t, Downloaded, result.Status)
	require.Equal(t, "Replacement Title Ch. 7.1 - The Chapter", result.Name)
	require.Equal(t, filepath.Join(request.DownloadDirectory, "Replacement Title", "Replacement Title Ch. 7.1 - The Chapter.cbz"), result.Path)
	require.Equal(t, 1, source.calls)

	archive, err := zip.OpenReader(result.Path)
	require.NoError(t, err)
	require.Len(t, archive.File, 1)
	require.NoError(t, archive.Close())

	result, err = Chapter(t.Context(), zerolog.Nop(), request)
	require.NoError(t, err)
	require.Equal(t, Skipped, result.Status)
	require.Equal(t, 1, source.calls, "skip must not resolve pages")
}

func TestChapterReturnsPageResolutionErrorWithoutPublishingArchive(t *testing.T) {
	t.Parallel()

	downloadDirectory := t.TempDir()
	request := Request{
		Source:            &pageSource{err: context.Canceled},
		Manga:             domain.Manga{Title: "Title"},
		Chapter:           domain.Chapter{Number: mustChapterNumber("2")},
		DownloadDirectory: downloadDirectory,
		NamingTemplate:    "Chapter {num}",
	}

	result, err := Chapter(t.Context(), zerolog.Nop(), request)
	require.ErrorIs(t, err, context.Canceled)
	require.NoFileExists(t, result.Path)

	entries, readErr := os.ReadDir(downloadDirectory)
	require.NoError(t, readErr)
	require.Empty(t, entries)
}

type pageSource struct {
	pages []domain.ImageInfo
	err   error
	calls int
}

func mustChapterNumber(input string) domain.ChapterNumber {
	number, err := domain.ParseChapterNumber(input)
	if err != nil {
		panic(err)
	}
	return number
}

func (s *pageSource) Pages(context.Context, domain.Chapter) ([]domain.ImageInfo, error) {
	s.calls++
	return s.pages, s.err
}
