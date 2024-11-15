package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"mangarr/internal/buildinfo"
	"mangarr/internal/config"
	"mangarr/internal/domain"
	"mangarr/internal/download"
	"mangarr/internal/files"
	"mangarr/internal/logger"
	"mangarr/internal/parse"
	"mangarr/internal/sanitize"
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

		var sources []domain.Source

		for mangaName, monitoredManga := range cfg.Config.MonitoredManga {
			switch monitoredManga.Source {
			case "tcbscans":
				sources = append(sources, source.NewTCBScans(monitoredManga.Manga))
			case "mangadex":
				sources = append(sources, source.NewMangadex(monitoredManga.Manga, monitoredManga.Group, monitoredManga.Language))
			case "mangaplus":
				sources = append(sources, source.NewMangaPlus(monitoredManga.Manga))
			case "flamecomics":
				sources = append(sources, source.NewFlamecomics(monitoredManga.Manga))
			case "asurascans":
				sources = append(sources, source.NewAsurascans(monitoredManga.Manga))
			case "cubari":
				sources = append(sources, source.NewCubari(monitoredManga.Manga, monitoredManga.Group))
			default:
				log.Error().Msgf("unknown monitored manga source for %s: %s", mangaName, monitoredManga.Source)
				continue
			}
		}

		log.Info().Msg("starting to monitor configured manga")

		ticker := time.NewTicker(time.Duration(cfg.Config.CheckInterval)*time.Minute - 40*time.Second)
		defer ticker.Stop()

		wg := sync.WaitGroup{}
		quit := make(chan bool, 1)

		go func() {
			for {
				select {
				case <-quit:
					return
				case <-ticker.C:
					for _, s := range sources {
						wg.Add(1)

						go func() {
							defer wg.Done()

							if err := s.ValidateInput(); err != nil {
								log.Error().Err(err).Msgf("error validating input")
								return
							}

							selectedManga, err := s.GetManga(ctx)
							if err != nil {
								log.Error().Err(err).Msgf("error getting manga from %s", s)
								return
							}
							mLog := log.With().Str("manga", selectedManga.Title).Str("source", s.String()).Logger()

							if err := s.GetChapters(ctx, selectedManga); err != nil {
								mLog.Error().Err(err).Msg("error getting manga chapters")
								return
							}

							_, latestChapterNr, err := parse.GetMinAndMaxKeys(selectedManga.Chapters)
							if err != nil {
								mLog.Error().Err(err).Msg("error parsing chapter number")
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

							if err := s.GetImageURLs(ctx, &selectedChapter); err != nil {
								mLog.Error().Err(err).Msgf("error getting image urls for chapter %g", selectedChapter.Number)
								return
							}

							t := templater.New(selectedManga, selectedChapter)
							templatedName := t.ExecTemplate(cfg.Config.NamingTemplate)

							chapterFolder := sanitize.Filename(templatedName)
							contentPath := filepath.Join(cfg.Config.DownloadLocation, selectedManga.Title, chapterFolder+".cbz")

							if _, err := os.Stat(contentPath); err == nil {
								mLog.Debug().Msgf("chapter has already been downloaded, skipping %q", templatedName)
								return
							}

							mLog.Info().Msgf("downloading %q", templatedName)
							if err := download.Chapter(ctx, contentPath, selectedChapter); err != nil {
								mLog.Error().Err(err).Msgf("error downloading chapter %q", templatedName)
								return
							}
							mLog.Info().Msgf("finished downloading %q", templatedName)
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
	},
}
