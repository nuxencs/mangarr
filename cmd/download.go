package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

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
	Use:          "download",
	Short:        "Download a specified chapter",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()

		// init new logger
		log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).With().Timestamp().Logger()

		if !cmd.Flags().Changed("first") && !cmd.Flags().Changed("chapters") && !cmd.Flags().Changed("all") {
			latest = true
		}

		if err := files.IsValidLocation(downloadDirectory); err != nil {
			return fmt.Errorf("invalid download location: %w", err)
		}

		var s domain.Source

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
			s = source.NewAsurascans(manga)
		case "cubari":
			s = source.NewCubari(manga, group)
		case "weebcentral":
			s = source.NewWeebCentral(manga)
		case "comix":
			s = source.NewComix(manga, group)
		case "atsumaru":
			s = source.NewAtsumaru(manga, group)
		default:
			return fmt.Errorf("invalid source %q", mangaSource)
		}

		if err := s.ValidateInput(); err != nil {
			return fmt.Errorf("invalid input: %w", err)
		}

		selectedManga, err := s.GetManga(ctx)
		if err != nil {
			return fmt.Errorf("getting manga from %s: %w", s, err)
		}

		if err := s.GetChapters(ctx, selectedManga); err != nil {
			return fmt.Errorf("getting chapters for %s: %w", selectedManga.Title, err)
		}

		var selectedChapterNumbers []domain.ChapterNumber

		firstChapterNr, latestChapterNr, err := parse.MinMaxChapterNumbers(selectedManga.Chapters)
		if err != nil {
			return fmt.Errorf("parsing chapter numbers for %s: %w", selectedManga.Title, err)
		}

		switch {
		case first:
			selectedChapterNumbers = []domain.ChapterNumber{firstChapterNr}
		case latest:
			selectedChapterNumbers = []domain.ChapterNumber{latestChapterNr}
		case downloadAll:
			selectedChapterNumbers = sortedChapterNumbers(selectedManga.Chapters)
		default:
			selectedChapterNumbers, err = parse.ChapterSelection(chapterNumbers, selectedManga.Chapters)
			if err != nil {
				return fmt.Errorf("parsing chapter selection for %s: %w", selectedManga.Title, err)
			}
		}

		if len(selectedChapterNumbers) == 0 {
			return fmt.Errorf("finding matching chapters in range %s for %s", chapterNumbers, selectedManga.Title)
		}

		overwrittenTitle := sanitize.Filename(overwrite)
		if len(overwrittenTitle) != 0 {
			selectedManga.Title = overwrittenTitle
		}

		const (
			chapterStatusDownloaded = "downloaded"
			chapterStatusSkipped    = "skipped"
			chapterStatusFailed     = "failed"
		)

		type chapterResult struct {
			chapterNumber domain.ChapterNumber
			status        string
		}

		results := make(chan chapterResult, len(selectedChapterNumbers))

		// semaphore to limit concurrency to maxConcurrentChapterProcesses which is set to 10
		sem := semaphore.NewWeighted(maxConcurrentChapterProcesses)
		var wg sync.WaitGroup

		for _, chapterNumber := range selectedChapterNumbers {
			wg.Go(func() {
				sem.Acquire()

				result := chapterResult{
					chapterNumber: chapterNumber,
					status:        chapterStatusFailed,
				}
				shouldDelay := true

				defer func() {
					if shouldDelay {
						time.Sleep(chapterDelay)
					}

					results <- result
					sem.Release()
				}()

				selectedChapter, ok := selectedManga.Chapters[chapterNumber]
				if !ok {
					err := fmt.Errorf("chapter %s not found", chapterNumber)
					log.Error().Err(err).Msgf("Failed to find chapter with number %s", chapterNumber)
					return
				}

				t := templater.New(selectedManga, selectedChapter)
				templatedName := t.ExecTemplate(naming)

				chapterFolder := sanitize.Filename(templatedName)
				contentPath := filepath.Join(downloadDirectory, selectedManga.Title, chapterFolder+".cbz")

				if _, err := os.Stat(contentPath); err == nil {
					log.Info().Msgf("Chapter has already been downloaded, skipping %q", templatedName)
					result.status = chapterStatusSkipped
					shouldDelay = false
					return
				}

				if err := s.GetImageURLs(ctx, &selectedChapter); err != nil {
					log.Error().Err(err).Msgf("Failed to get image URLs for chapter %s", selectedChapter.Number)
					return
				}

				log.Info().Msgf("Downloading %q", templatedName)
				if err := download.Chapter(ctx, log, contentPath, selectedChapter, selectedManga.IsManhwa, files.CreateCbzArchive); err != nil {
					log.Error().Err(err).Msgf("Failed to download chapter %q", templatedName)
					return
				}

				log.Info().Msgf("Finished downloading %q", templatedName)
				result.status = chapterStatusDownloaded
			})
		}

		wg.Wait()
		close(results)

		downloaded := 0
		var skipped []domain.ChapterNumber
		var failed []domain.ChapterNumber
		for result := range results {
			switch result.status {
			case chapterStatusDownloaded:
				downloaded++
			case chapterStatusSkipped:
				skipped = append(skipped, result.chapterNumber)
			case chapterStatusFailed:
				failed = append(failed, result.chapterNumber)
			}
		}

		if len(selectedChapterNumbers) > 1 {
			log.Info().Msgf(
				"Summary: downloaded=%d skipped=%d failed=%d",
				downloaded,
				len(skipped),
				len(failed),
			)

			if len(skipped) > 0 {
				log.Info().Msgf("Skipped chapters: %s", parse.FormatChapterList(skipped))
			}
			if len(failed) > 0 {
				log.Info().Msgf("Failed chapters: %s", parse.FormatChapterList(failed))
			}
		}

		if len(failed) > 0 {
			return fmt.Errorf("failed to download chapters: %s", parse.FormatChapterList(failed))
		}

		return nil
	},
}

const chapterDelay = 2 * time.Second

func sortedChapterNumbers(chapters map[domain.ChapterNumber]domain.Chapter) []domain.ChapterNumber {
	numbers := make([]domain.ChapterNumber, 0, len(chapters))
	for number := range chapters {
		numbers = append(numbers, number)
	}

	slices.SortFunc(numbers, func(a, b domain.ChapterNumber) int {
		return a.Compare(b)
	})

	return numbers
}
