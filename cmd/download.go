package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

		if !cmd.Flags().Changed("first") && !cmd.Flags().Changed("chapters") && !cmd.Flags().Changed("all") {
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
		case "mangapark":
			s = source.NewMangaPark(manga, bm)
		case "weebcentral":
			s = source.NewWeebCentral(manga, bm)
		case "comix":
			s = source.NewComix(manga, group)
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
			selectedChapterNumbers = []float32{firstChapterNr}
		case latest:
			selectedChapterNumbers = []float32{latestChapterNr}
		case downloadAll:
			selectedChapterNumbers = sortedChapterNumbers(selectedManga.Chapters)
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
			name          string
			chapterNumber float32
			status        string
			err           error
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
					name:          fmt.Sprintf("Chapter %g", chapterNumber),
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
					result.err = fmt.Errorf("chapter %g not found", chapterNumber)
					log.Error().Err(result.err).Msgf("Failed to find chapter with number %g", chapterNumber)
					return
				}

				t := templater.New(selectedManga, selectedChapter)
				templatedName := t.ExecTemplate(naming)
				result.name = templatedName

				chapterFolder := sanitize.Filename(templatedName)
				contentPath := filepath.Join(downloadDirectory, selectedManga.Title, chapterFolder+".cbz")

				if _, err := os.Stat(contentPath); err == nil {
					log.Info().Msgf("Chapter has already been downloaded, skipping %q", templatedName)
					result.status = chapterStatusSkipped
					result.err = nil
					shouldDelay = false
					return
				}

				if err := s.GetImageURLs(ctx, &selectedChapter); err != nil {
					result.err = err
					log.Error().Err(err).Msgf("Failed to get image URLs for chapter %g", selectedChapter.Number)
					return
				}

				log.Info().Msgf("Downloading %q", templatedName)
				if err := download.Chapter(ctx, log, contentPath, selectedChapter, selectedManga.IsManhwa, files.CreateCbzArchive); err != nil {
					result.err = err
					log.Error().Err(err).Msgf("Failed to download chapter %q", templatedName)
					return
				}

				log.Info().Msgf("Finished downloading %q", templatedName)
				result.status = chapterStatusDownloaded
				result.err = nil
			})
		}

		wg.Wait()
		close(results)

		if len(selectedChapterNumbers) > 1 {
			downloaded := 0
			var skipped []float32
			var failed []float32

			for res := range results {
				switch res.status {
				case chapterStatusDownloaded:
					downloaded++
				case chapterStatusSkipped:
					skipped = append(skipped, res.chapterNumber)
				case chapterStatusFailed:
					failed = append(failed, res.chapterNumber)
				}
			}

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
		} else {
			for res := range results {
				// drain channel if no summary required
				_ = res
			}
		}
	},
}

const chapterDelay = 2 * time.Second

func sortedChapterNumbers(chapters map[float32]domain.Chapter) []float32 {
	numbers := make([]float32, 0, len(chapters))
	for number := range chapters {
		numbers = append(numbers, number)
	}

	slices.Sort(numbers)

	return numbers
}
