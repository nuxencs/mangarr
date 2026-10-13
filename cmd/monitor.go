package cmd

import (
	"context"
	"fmt"
	"maps"
	"os/signal"
	"slices"
	"sync"
	"syscall"
	"time"

	"mangarr/internal/buildinfo"
	"mangarr/internal/config"
	"mangarr/internal/domain"
	"mangarr/internal/files"
	"mangarr/internal/logger"
	"mangarr/internal/parse"
	"mangarr/internal/perf"
	"mangarr/internal/source"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func newMonitorCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:          "monitor",
		Short:        "Monitor a specified manga for new chapters",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			runCtx, stop := signal.NotifyContext(
				cmd.Context(),
				syscall.SIGHUP,
				syscall.SIGINT,
				syscall.SIGQUIT,
				syscall.SIGTERM,
			)
			defer stop()

			cfg, err := config.Load(options.configPath, buildinfo.Version)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			snapshot := cfg.Snapshot()

			log := logger.New(&snapshot)

			if err := cfg.UpdateConfig(); err != nil {
				log.Error().Err(err).Msg("error updating config")
			}

			reloads, err := cfg.DynamicReload(log)
			if err != nil {
				log.Error().Err(err).Msg("dynamic config reload disabled")
				reloads = make(chan struct{})
			}

			if err := files.IsValidLocation(snapshot.DownloadLocation); err != nil {
				return fmt.Errorf("invalid download location: %w", err)
			}

			log.Info().Msg("Starting to monitor configured manga")
			log.Info().Msgf("Version: %s", buildinfo.Version)
			log.Info().Msgf("Commit: %s", buildinfo.Commit)
			log.Info().Msgf("Build date: %s", buildinfo.Date)
			log.Info().Msgf("Log-level: %s", snapshot.LogLevel)

			if snapshot.PprofEnabled {
				pprofAddress, err := perf.StartPprofServer(runCtx, log, snapshot.PprofAddress)
				if err != nil {
					log.Error().Err(err).Msg("error starting pprof endpoint")
				} else {
					log.Info().Msgf("pprof endpoint available at http://%s/debug/pprof/", pprofAddress)
				}
			}

			marks := newMonitorMarks()
			runMonitorCycle(runCtx, snapshot, log, marks)

			timer := time.NewTimer(snapshot.CheckInterval * time.Minute)
			defer timer.Stop()
			for {
				select {
				case <-runCtx.Done():
					log.Info().Msg("stopping monitoring")
					return nil
				case <-reloads:
					snapshot = cfg.Snapshot()
					resetTimer(timer, snapshot.CheckInterval*time.Minute)
				case <-timer.C:
					snapshot = cfg.Snapshot()
					runMonitorCycle(runCtx, snapshot, log, marks)
					timer.Reset(snapshot.CheckInterval * time.Minute)
				}
			}
		},
	}
}

func resetTimer(timer *time.Timer, duration time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(duration)
}

func runMonitorCycle(ctx context.Context, cfg domain.Config, log *logger.DefaultLogger, marks *monitorMarks) {
	if err := files.IsValidLocation(cfg.DownloadLocation); err != nil {
		log.Error().Err(err).Msg("invalid download location")
		return
	}

	marks.prune(cfg.MonitoredManga)

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(maxConcurrentSourceProcesses)
	for mangaTitle, monitoredManga := range cfg.MonitoredManga {
		group.Go(func() error {
			if err := monitorManga(groupCtx, cfg, mangaTitle, *monitoredManga, log, marks); err != nil {
				log.Error().Err(err).
					Str("manga", mangaTitle).
					Str("source", monitoredManga.Source).
					Msg("monitor check failed")
			}
			return nil
		})
	}
	_ = group.Wait()
}

func monitorManga(ctx context.Context, cfg domain.Config, mangaTitle string, monitoredManga domain.MonitoredManga, log *logger.DefaultLogger, marks *monitorMarks) error {
	mangaSource, err := source.Select(monitoredManga)
	if err != nil {
		return fmt.Errorf("selecting manga source: %w", err)
	}
	mLog := log.With().Str("manga", mangaTitle).Str("source", mangaSource.String()).Logger()

	if err := mangaSource.ValidateInput(); err != nil {
		return fmt.Errorf("validating input: %w", err)
	}

	selectedManga, err := mangaSource.Discover(ctx)
	if err != nil {
		return fmt.Errorf("getting manga from %s: %w", monitoredManga.Source, err)
	}

	numbers := sortedChapterNumbers(selectedManga.Chapters)
	if len(numbers) == 0 {
		return fmt.Errorf("finding chapters for %s", selectedManga.Title)
	}
	latest := numbers[len(numbers)-1]
	mark, hasMark := marks.get(mangaTitle, monitoredManga)
	selected := monitorSelection(numbers, mark, hasMark)

	result := downloadChapters(ctx, mLog, chapterBatch{
		source:            mangaSource,
		manga:             selectedManga,
		downloadDirectory: cfg.DownloadLocation,
		namingTemplate:    cfg.NamingTemplate,
		titleOverride:     monitoredManga.Overwrite,
		skipLevel:         zerolog.DebugLevel,
	}, selected)
	if len(result.failed) > 0 {
		// Keep the old mark so the next check selects the failed chapters again.
		return fmt.Errorf("downloading chapters %s", parse.FormatChapterList(result.failed))
	}

	marks.set(mangaTitle, monitoredManga, latest)
	return nil
}

// monitorSelection returns the latest chapter plus every chapter above mark.
// Without a mark, as on the first check of a series, only the latest chapter is selected.
func monitorSelection(sorted []domain.ChapterNumber, mark domain.ChapterNumber, hasMark bool) []domain.ChapterNumber {
	latest := sorted[len(sorted)-1]
	if !hasMark || !mark.Less(latest) {
		return []domain.ChapterNumber{latest}
	}

	first := slices.IndexFunc(sorted, mark.Less)
	return sorted[first:]
}

// monitorMarks remembers the highest chapter number of each monitored series at its last successful check.
// The marks live only in memory, so chapters released while monitor is stopped are not selected.
type monitorMarks struct {
	mu     sync.Mutex
	series map[string]seriesMark
}

type seriesMark struct {
	identity seriesIdentity
	highest  domain.ChapterNumber
}

// seriesIdentity holds the entry fields that decide chapter numbering.
// A change to any of them resets the mark, because another source or group can number chapters differently.
type seriesIdentity struct {
	source   string
	manga    string
	group    string
	language string
}

func newMonitorMarks() *monitorMarks {
	return &monitorMarks{series: make(map[string]seriesMark)}
}

func identityOf(entry domain.MonitoredManga) seriesIdentity {
	return seriesIdentity{source: entry.Source, manga: entry.Manga, group: entry.Group, language: entry.Language}
}

func (m *monitorMarks) get(title string, entry domain.MonitoredManga) (domain.ChapterNumber, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mark, ok := m.series[title]
	if !ok || mark.identity != identityOf(entry) {
		return domain.ChapterNumber{}, false
	}
	return mark.highest, true
}

func (m *monitorMarks) set(title string, entry domain.MonitoredManga, highest domain.ChapterNumber) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.series[title] = seriesMark{identity: identityOf(entry), highest: highest}
}

// prune drops marks of series that are no longer configured.
func (m *monitorMarks) prune(configured map[string]*domain.MonitoredManga) {
	m.mu.Lock()
	defer m.mu.Unlock()

	maps.DeleteFunc(m.series, func(title string, _ seriesMark) bool {
		_, ok := configured[title]
		return !ok
	})
}
