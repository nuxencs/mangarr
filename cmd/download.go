package cmd

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"mangarr/internal/browser"
	"mangarr/internal/domain"
	"mangarr/internal/download"
	"mangarr/internal/files"
	"mangarr/internal/parse"
	"mangarr/internal/sanitize"
	"mangarr/internal/semaphore"
	"mangarr/internal/source"
	"mangarr/internal/templater"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download a specified chapter",
	Run: func(cmd *cobra.Command, _ []string) {
		ctx := cmd.Context()

		// init new logger
		log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).With().Timestamp().Logger()

		if !cmd.Flags().Changed("first") && !cmd.Flags().Changed("chapters") {
			latest = true
		}

		if err := files.IsValidLocation(downloadDirectory); err != nil {
			log.Error().Err(err).Msgf("Invalid download location")
			return
		}

		var s domain.Source

		bm := browser.NewManager()
		defer bm.Close()

		switch mangaSource {
		case "tcbscans":
			s = source.NewTCBScans(manga)
		case "mangadex":
			s = source.NewMangadex(manga, group, language)
		case "mangaplus":
			s = source.NewMangaPlus(manga)
		case "flamecomics":
			s = source.NewFlamecomics(manga)
		case "asurascans":
			s = source.NewAsurascans(manga, bm)
		case "cubari":
			s = source.NewCubari(manga, group)
		case "comick":
			s = source.NewComick(manga, group, language, bm)
		default:
			log.Error().Msgf("Invalid source: %s", mangaSource)
			return
		}

		if err := s.ValidateInput(); err != nil {
			log.Error().Err(err).Msgf("Invalid input")
			return
		}

		selectedManga, err := s.GetManga(ctx)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to get manga from %s", s)
			return
		}

		if err := s.GetChapters(ctx, selectedManga); err != nil {
			log.Error().Err(err).Msgf("Failed to get chapters for %s", selectedManga.Title)
			return
		}

		var selectedChapterNumbers []float32

		firstChapterNr, latestChapterNr, err := parse.MinMaxKeys(selectedManga.Chapters)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to parse chapter number for %s", selectedManga.Title)
			return
		}

		switch {
		case first:
			selectedChapterNumbers = firstChapterNr
		case latest:
			selectedChapterNumbers = latestChapterNr
		default:
			selectedChapterNumbers, err = parse.ChapterSelection(chapterNumbers, selectedManga.Chapters)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to parse chapter selection for %s", selectedManga.Title)
				return
			}
		}

		if len(selectedChapterNumbers) == 0 {
			log.Error().Msgf("Failed to find matching chapters in range %s for %s", chapterNumbers, selectedManga.Title)
			return
		}

		// semaphore to limit concurrency to maxConcurrentChapterProcesses which is set to 10
		sem := semaphore.NewWeighted(maxConcurrentChapterProcesses)
		wg := sync.WaitGroup{}

		for _, num := range selectedChapterNumbers {
			wg.Add(1)

			go func() {
				sem.Acquire()
				defer func() { sem.Release(); wg.Done() }()

				selectedChapter, ok := selectedManga.Chapters[num]
				if !ok {
					log.Error().Msgf("Failed to find chapter with number %g", num)
					return
				}

				overwrittenTitle := sanitize.Filename(overwrite)

				if len(overwrittenTitle) != 0 {
					selectedManga.Title = overwrittenTitle
				}

				t := templater.New(selectedManga, selectedChapter)
				templatedName := t.ExecTemplate(naming)

				chapterFolder := sanitize.Filename(templatedName)
				contentPath := filepath.Join(downloadDirectory, selectedManga.Title, chapterFolder+".cbz")

				if _, err := os.Stat(contentPath); err == nil {
					log.Info().Msgf("Chapter has already been downloaded, skipping %q", templatedName)
					return
				}

				if err := s.GetImageURLs(ctx, &selectedChapter); err != nil {
					log.Error().Err(err).Msgf("Failed to get image URLs for chapter %g", selectedChapter.Number)
					return
				}

				log.Info().Msgf("Downloading %q", templatedName)
				if err := download.Chapter(ctx, log, contentPath, selectedChapter, selectedManga.IsManhwa); err != nil {
					log.Error().Err(err).Msgf("Failed to download chapter %q", templatedName)
					return
				}

				log.Info().Msgf("Finished downloading %q", templatedName)
			}()
		}

		wg.Wait()
	},
}
