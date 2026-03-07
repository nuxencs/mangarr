package cmd

import (
	"context"
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
	"mangarr/internal/perf"
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
		runCtx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

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

		if cfg.Config.PprofEnabled {
			pprofAddress, err := perf.StartPprofServer(runCtx, log, cfg.Config.PprofAddress)
			if err != nil {
				log.Error().Err(err).Msgf("error starting pprof endpoint")
			} else {
				log.Info().Msgf("pprof endpoint available at http://%s/debug/pprof/", pprofAddress)
			}
		}

		ticker := time.NewTicker(cfg.Config.CheckInterval * time.Minute)
		defer ticker.Stop()

		bm := browser.NewManager()

		// semaphore to limit concurrency to maxConcurrentSourceProcesses which is set to 10
		sem := semaphore.NewWeighted(maxConcurrentSourceProcesses)
		quit := make(chan bool, 1)
		var wg sync.WaitGroup

		go func() {
			for {
				select {
				case <-quit:
					return
				case <-ticker.C:
					for mangaTitle, monitoredManga := range cfg.Config.MonitoredManga {
						wg.Go(func() {
							sem.Acquire()
							defer sem.Release()

							mangaSource, err := source.Select(*monitoredManga, bm)
							if err != nil {
								log.Error().Err(err).Msgf("error selecting manga source")
								return
							}
							mLog := log.With().Str("manga", mangaTitle).Str("source", mangaSource.String()).Logger()

							if err := mangaSource.ValidateInput(); err != nil {
								mLog.Error().Err(err).Msgf("error validating input")
								return
							}

							selectedManga, err := mangaSource.GetManga(runCtx)
							if err != nil {
								mLog.Error().Err(err).Msgf("error getting manga from %s", monitoredManga.Source)
								return
							}

							if err := mangaSource.GetChapters(runCtx, selectedManga); err != nil {
								mLog.Error().Err(err).Msg("error getting manga chapters")
								return
							}

							_, latestChapterNr, err := parse.MinMaxChapterNumbers(selectedManga.Chapters)
							if err != nil {
								mLog.Error().Err(err).Msg("error getting latest chapter number")
								return
							}

							selectedChapter, ok := selectedManga.Chapters[latestChapterNr]
							if !ok {
								mLog.Error().Msgf("error finding chapter with number %s", latestChapterNr)
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

							if err := mangaSource.GetImageURLs(runCtx, &selectedChapter); err != nil {
								mLog.Error().Err(err).Msgf("error getting image urls for chapter %s", selectedChapter.Number)
								return
							}

							mLog.Info().Msgf("downloading %q", templatedName)
							if err := download.Chapter(runCtx, mLog, contentPath, selectedChapter, selectedManga.IsManhwa, files.CreateCbzArchive); err != nil {
								mLog.Error().Err(err).Msgf("error downloading chapter %s", templatedName)
								return
							}
							mLog.Info().Msgf("finished downloading %s", templatedName)
						})
					}

					wg.Wait()
				}
			}
		}()

		// set up a channel to catch signals for graceful shutdown
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

		sig := <-sigCh
		fmt.Printf("received signal: %s, stopping monitoring.\n", sig)
		cancel()
		quit <- true
		wg.Wait()
		bm.Close()
	},
}
