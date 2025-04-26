package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"mangarr/internal/browser"
	"mangarr/internal/buildinfo"
	"mangarr/internal/config"
	"mangarr/internal/download"
	"mangarr/internal/files"
	"mangarr/internal/logger"
	"mangarr/internal/parse"
	"mangarr/internal/sanitize"
	"mangarr/internal/semaphore"
	"mangarr/internal/source"
	"mangarr/internal/templater"

	"github.com/spf13/cobra"
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Monitor a specified manga for new chapters",
	Run: func(cmd *cobra.Command, _ []string) {
		ctx := cmd.Context()

		// read config
		cfg := config.New(configPath, buildinfo.Version)

		// init new logger
		log := logger.New(cfg.Config)

		if err := cfg.UpdateConfig(); err != nil {
			log.Error().Err(err).Msgf("error updating config")
		}

		// init dynamic config
		cfg.DynamicReload(log)

		if err := files.IsValidLocation(cfg.Config.DownloadLocation); err != nil {
			log.Fatal().Err(err).Msgf("invalid download location")
		}

		log.Info().Msg("Starting to monitor configured manga")
		log.Info().Msgf("Version: %s", buildinfo.Version)
		log.Info().Msgf("Commit: %s", buildinfo.Commit)
		log.Info().Msgf("Build date: %s", buildinfo.Date)
		log.Info().Msgf("Log-level: %s", cfg.Config.LogLevel)

		ticker := time.NewTicker(cfg.Config.CheckInterval * time.Minute)
		defer ticker.Stop()

		bm := browser.NewManager()

		// semaphore to limit concurrency to maxConcurrentSourceProcesses which is set to 10
		sem := semaphore.NewWeighted(maxConcurrentSourceProcesses)
		quit := make(chan bool, 1)
		wg := sync.WaitGroup{}

		go func() {
			for {
				select {
				case <-quit:
					return
				case <-ticker.C:
					for _, monitoredManga := range cfg.Config.MonitoredManga {
						wg.Add(1)

						go func() {
							sem.Acquire()
							defer func() { sem.Release(); wg.Done() }()

							mangaSource, err := source.Select(*monitoredManga, bm)
							if err != nil {
								log.Error().Err(err).Msgf("error selecting manga source")
								return
							}

							if err := mangaSource.ValidateInput(); err != nil {
								log.Error().Err(err).Msgf("error validating input")
								return
							}

							selectedManga, err := mangaSource.GetManga(ctx)
							if err != nil {
								log.Error().Err(err).Msgf("error getting manga from %s", monitoredManga.Source)
								return
							}
							mLog := log.With().Str("manga", selectedManga.Title).Str("source", mangaSource.String()).Logger()

							if err := mangaSource.GetChapters(ctx, selectedManga); err != nil {
								mLog.Error().Err(err).Msg("error getting manga chapters")
								return
							}

							_, latestChapterNr, err := parse.MinMaxKeys(selectedManga.Chapters)
							if err != nil {
								mLog.Error().Err(err).Msg("error getting latest chapter number")
								return
							}

							if len(latestChapterNr) == 0 {
								mLog.Error().Msg("error finding latest chapter")
								return
							}

							var num float32
							for _, n := range latestChapterNr {
								num = n
								break
							}

							selectedChapter, ok := selectedManga.Chapters[num]
							if !ok {
								mLog.Error().Err(err).Msgf("error finding chapter with number %g", num)
								return
							}

							overwrittenTitle := sanitize.Filename(monitoredManga.Overwrite)

							if len(overwrittenTitle) != 0 {
								selectedManga.Title = overwrittenTitle
							}

							t := templater.New(selectedManga, selectedChapter)
							templatedName := t.ExecTemplate(cfg.Config.NamingTemplate)

							chapterFolder := sanitize.Filename(templatedName)
							contentPath := filepath.Join(cfg.Config.DownloadLocation, selectedManga.Title, chapterFolder+".cbz")

							if _, err := os.Stat(contentPath); err == nil {
								mLog.Debug().Msgf("chapter has already been downloaded, skipping %s", templatedName)
								return
							}

							if err := mangaSource.GetImageURLs(ctx, &selectedChapter); err != nil {
								mLog.Error().Err(err).Msgf("error getting image urls for chapter %g", selectedChapter.Number)
								return
							}

							mLog.Info().Msgf("downloading %q", templatedName)
							if err := download.Chapter(ctx, mLog, contentPath, selectedChapter, selectedManga.IsManhwa); err != nil {
								mLog.Error().Err(err).Msgf("error downloading chapter %s", templatedName)
								return
							}
							mLog.Info().Msgf("finished downloading %s", templatedName)
						}()
					}

					wg.Wait()
				}
			}
		}()

		// set up a channel to catch signals for graceful shutdown
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

		fmt.Printf("received signal: %s, stopping monitoring.\n", <-sigCh)
		quit <- true
		wg.Wait()
		bm.Close()
	},
}
