package cmd

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"mangarr/internal/acquire"
	"mangarr/internal/domain"

	"github.com/rs/zerolog"
)

const chapterDelay = 2 * time.Second

// chapterBatch holds the settings shared by every chapter of one download or monitor check.
type chapterBatch struct {
	source            domain.Source
	manga             domain.Manga
	downloadDirectory string
	namingTemplate    string
	titleOverride     string
	force             bool
	// skipLevel keeps repeated monitor skips of the latest archive out of info logs.
	skipLevel zerolog.Level
}

type chapterBatchResult struct {
	downloaded int
	skipped    []domain.ChapterNumber
	failed     []domain.ChapterNumber
}

// downloadChapters acquires the selected chapters with bounded concurrency and reports each outcome.
func downloadChapters(ctx context.Context, log zerolog.Logger, batch chapterBatch, numbers []domain.ChapterNumber) chapterBatchResult {
	type chapterOutcome struct {
		number domain.ChapterNumber
		status acquire.Status
		err    bool
	}

	outcomes := make(chan chapterOutcome, len(numbers))
	limit := make(chan struct{}, maxConcurrentChapterProcesses)
	var queuedChapters atomic.Int64
	queuedChapters.Store(int64(len(numbers)))
	var wg sync.WaitGroup

	for _, number := range numbers {
		wg.Go(func() {
			limit <- struct{}{}
			queuedChapters.Add(-1)

			outcome := chapterOutcome{number: number, err: true}
			shouldDelay := true

			defer func() {
				// Keep the cooldown only when another chapter is waiting for this slot.
				if shouldDelay && queuedChapters.Load() > 0 {
					time.Sleep(chapterDelay)
				}

				outcomes <- outcome
				<-limit
			}()

			chapter, ok := batch.manga.Chapters[number]
			if !ok {
				err := fmt.Errorf("chapter %s not found", number)
				log.Error().Err(err).Msgf("Failed to find chapter with number %s", number)
				return
			}

			acquisition, err := acquire.Chapter(ctx, log, acquire.Request{
				Source:            batch.source,
				Manga:             batch.manga,
				Chapter:           chapter,
				DownloadDirectory: batch.downloadDirectory,
				NamingTemplate:    batch.namingTemplate,
				TitleOverride:     batch.titleOverride,
				Force:             batch.force,
			})
			if err != nil {
				log.Error().Err(err).Msgf("Failed to acquire chapter %s", chapter.Number)
				return
			}

			outcome.err = false
			outcome.status = acquisition.Status
			switch acquisition.Status {
			case acquire.Skipped:
				log.WithLevel(batch.skipLevel).Msgf("Chapter has already been downloaded, skipping %q", acquisition.Name)
				shouldDelay = false
			case acquire.Downloaded:
				log.Info().Msgf("Finished downloading %q", acquisition.Name)
			}
		})
	}

	wg.Wait()
	close(outcomes)

	var result chapterBatchResult
	for outcome := range outcomes {
		switch {
		case outcome.err:
			result.failed = append(result.failed, outcome.number)
		case outcome.status == acquire.Skipped:
			result.skipped = append(result.skipped, outcome.number)
		default:
			result.downloaded++
		}
	}

	return result
}

func sortedChapterNumbers(chapters map[domain.ChapterNumber]domain.Chapter) []domain.ChapterNumber {
	return slices.SortedFunc(maps.Keys(chapters), domain.ChapterNumber.Compare)
}
