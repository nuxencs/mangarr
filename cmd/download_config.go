package cmd

import (
	"fmt"
	"strings"

	"mangarr/internal/domain"

	"github.com/spf13/cobra"
)

func resolveDownloadOptions(cmd *cobra.Command, snapshot domain.Config, options *downloadOptions) error {
	if !cmd.Flags().Changed("series") {
		return nil
	}
	if strings.TrimSpace(options.series) == "" {
		return fmt.Errorf("--series requires a configured entry name")
	}

	entry, ok := snapshot.MonitoredManga[options.series]
	if !ok {
		return fmt.Errorf("unknown configured series %q: no matching monitoredManga entry", options.series)
	}
	if entry == nil {
		return fmt.Errorf("monitoredManga %q cannot be null", options.series)
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
	if options.naming == "" {
		return fmt.Errorf("configured series %q: naming template cannot be empty (set namingTemplate or --naming)", options.series)
	}
	if strings.TrimSpace(options.mangaSource) == "" {
		return fmt.Errorf("configured series %q: source is required (set source or --source)", options.series)
	}
	if strings.TrimSpace(options.manga) == "" {
		return fmt.Errorf("configured series %q: manga is required (set manga or --manga)", options.series)
	}
	return nil
}
