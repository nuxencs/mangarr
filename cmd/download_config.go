package cmd

import (
	"fmt"
	"strings"

	"mangarr/internal/buildinfo"
	"mangarr/internal/config"

	"github.com/spf13/cobra"
)

func resolveDownloadOptions(cmd *cobra.Command, configPath string, options *downloadOptions) error {
	if !cmd.Flags().Changed("series") {
		return nil
	}
	if strings.TrimSpace(options.series) == "" {
		return fmt.Errorf("--series requires a configured entry name")
	}

	cfg, err := config.LoadExisting(configPath, buildinfo.Version)
	if err != nil {
		return fmt.Errorf("loading config for series %q: %w", options.series, err)
	}
	snapshot := cfg.Snapshot()
	entry, ok := snapshot.MonitoredManga[options.series]
	if !ok {
		return fmt.Errorf("unknown configured series %q: no matching monitoredManga entry", options.series)
	}

	defaults := []struct {
		flag   string
		target *string
		value  string
	}{
		{"source", &options.mangaSource, entry.Source},
		{"manga", &options.manga, entry.Manga},
		{"group", &options.group, entry.Group},
		{"overwrite", &options.overwrite, entry.Overwrite},
		{"downloadDirectory", &options.downloadDirectory, snapshot.DownloadLocation},
		{"naming", &options.naming, snapshot.NamingTemplate},
	}
	for _, setting := range defaults {
		if !cmd.Flags().Changed(setting.flag) {
			*setting.target = setting.value
		}
	}
	if !cmd.Flags().Changed("language") && entry.Language != "" {
		options.language = entry.Language
	}
	if strings.TrimSpace(options.mangaSource) == "" {
		return fmt.Errorf("configured series %q: source is required (set source or --source)", options.series)
	}
	if strings.TrimSpace(options.manga) == "" {
		return fmt.Errorf("configured series %q: manga is required (set manga or --manga)", options.series)
	}
	return nil
}
