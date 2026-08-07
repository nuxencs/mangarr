// Package acquire resolves and stores one manga chapter.
package acquire

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"mangarr/internal/domain"
	"mangarr/internal/download"
	"mangarr/internal/files"
	"mangarr/internal/sanitize"
	"mangarr/internal/templater"

	"github.com/rs/zerolog"
)

type Status uint8

const (
	Downloaded Status = iota
	Skipped
)

type PageSource interface {
	Pages(context.Context, domain.Chapter) ([]domain.ImageInfo, error)
}

type Request struct {
	Source            PageSource
	Manga             domain.Manga
	Chapter           domain.Chapter
	DownloadDirectory string
	NamingTemplate    string
	TitleOverride     string
}

type Result struct {
	Status Status
	Name   string
	Path   string
}

func Chapter(ctx context.Context, log zerolog.Logger, request Request) (Result, error) {
	manga := request.Manga
	if title := sanitize.Filename(request.TitleOverride); title != "" {
		manga.Title = title
	}

	name := templater.New(manga, request.Chapter).ExecTemplate(request.NamingTemplate)
	archiveName := sanitize.Filename(name) + ".cbz"
	archivePath := filepath.Join(request.DownloadDirectory, manga.Title, archiveName)
	result := Result{Name: name, Path: archivePath}

	if _, err := os.Stat(archivePath); err == nil {
		result.Status = Skipped
		return result, nil
	} else if !os.IsNotExist(err) {
		return result, fmt.Errorf("checking archive %s: %w", archivePath, err)
	}

	pages, err := request.Source.Pages(ctx, request.Chapter)
	if err != nil {
		return result, fmt.Errorf("getting pages for chapter %s: %w", request.Chapter.Number, err)
	}
	request.Chapter.ImageInfo = pages

	log.Info().Msgf("Downloading %q", name)
	if err := download.Chapter(
		ctx,
		log,
		archivePath,
		request.Chapter,
		manga.IsManhwa,
		files.CreateCbzArchive,
	); err != nil {
		return result, fmt.Errorf("downloading chapter %q: %w", name, err)
	}

	result.Status = Downloaded
	return result, nil
}
