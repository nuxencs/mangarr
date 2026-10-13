package cmd

import (
	"fmt"
	"os"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/files"
	"mangarr/internal/parse"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

func newDownloadCommand(options *downloadOptions, root *rootOptions, selectSource func(domain.MonitoredManga) (domain.Source, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "download",
		Short: "Download chapters from explicit inputs or a configured series",
		Example: `  mangarr download -c ~/.config/mangarr --series "One Piece" -C "1-3"
  mangarr download -d ./downloads -s tcbscans -m "One Piece"`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if err := resolveDownloadOptions(cmd, root.configPath, options); err != nil {
				return err
			}

			// init new logger
			log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).With().Timestamp().Logger()

			if !cmd.Flags().Changed("first") && !cmd.Flags().Changed("chapters") && !cmd.Flags().Changed("all") {
				options.latest = true
			}

			if err := files.IsValidLocation(options.downloadDirectory); err != nil {
				return fmt.Errorf("invalid download location: %w", err)
			}

			s, err := selectSource(domain.MonitoredManga{
				Source:   options.mangaSource,
				Manga:    options.manga,
				Group:    options.group,
				Language: options.language,
			})
			if err != nil {
				if cmd.Flags().Changed("series") {
					return fmt.Errorf("configured series %q: selecting source: %w", options.series, err)
				}
				return fmt.Errorf("selecting source: %w", err)
			}

			if err := s.ValidateInput(); err != nil {
				if cmd.Flags().Changed("series") {
					return fmt.Errorf("configured series %q: invalid input: %w", options.series, err)
				}
				return fmt.Errorf("invalid input: %w", err)
			}

			selectedManga, err := s.Discover(ctx)
			if err != nil {
				return fmt.Errorf("getting manga from %s: %w", s, err)
			}

			var selectedChapterNumbers []domain.ChapterNumber

			firstChapterNr, latestChapterNr, err := parse.MinMaxChapterNumbers(selectedManga.Chapters)
			if err != nil {
				return fmt.Errorf("parsing chapter numbers for %s: %w", selectedManga.Title, err)
			}

			switch {
			case options.first:
				selectedChapterNumbers = []domain.ChapterNumber{firstChapterNr}
			case options.latest:
				selectedChapterNumbers = []domain.ChapterNumber{latestChapterNr}
			case options.downloadAll:
				selectedChapterNumbers = sortedChapterNumbers(selectedManga.Chapters)
			default:
				selectedChapterNumbers, err = parse.ChapterSelection(options.chapterNumbers, selectedManga.Chapters)
				if err != nil {
					return fmt.Errorf("parsing chapter selection for %s: %w", selectedManga.Title, err)
				}
			}

			if len(selectedChapterNumbers) == 0 {
				return fmt.Errorf("finding matching chapters in range %s for %s", options.chapterNumbers, selectedManga.Title)
			}

			result := downloadChapters(ctx, log, chapterBatch{
				source:            s,
				manga:             selectedManga,
				downloadDirectory: options.downloadDirectory,
				namingTemplate:    options.naming,
				titleOverride:     options.overwrite,
				force:             options.force,
				skipLevel:         zerolog.InfoLevel,
			}, selectedChapterNumbers)

			if len(selectedChapterNumbers) > 1 {
				log.Info().Msgf(
					"Summary: downloaded=%d skipped=%d failed=%d",
					result.downloaded,
					len(result.skipped),
					len(result.failed),
				)

				if len(result.skipped) > 0 {
					log.Info().Msgf("Skipped chapters: %s", parse.FormatChapterList(result.skipped))
				}
				if len(result.failed) > 0 {
					log.Info().Msgf("Failed chapters: %s", parse.FormatChapterList(result.failed))
				}
			}

			if len(result.failed) > 0 {
				return fmt.Errorf("failed to download chapters: %s", parse.FormatChapterList(result.failed))
			}

			return nil
		},
	}
}
