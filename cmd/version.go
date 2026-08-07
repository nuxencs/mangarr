package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"mangarr/internal/buildinfo"

	"github.com/spf13/cobra"
)

const githubURL = "https://api.github.com/repos/nuxencs/mangarr/releases/latest"

func newVersionCommand(client *http.Client, releaseURL string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Display version info",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "Version:", buildinfo.Version)
			fmt.Fprintln(cmd.OutOrStdout(), "Commit:", buildinfo.Commit)
			fmt.Fprintln(cmd.OutOrStdout(), "Build date:", buildinfo.Date)

			release, err := latestRelease(cmd, client, releaseURL)
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "Update check unavailable:", err)
				return nil
			}
			if release.TagName != buildinfo.Version && buildinfo.Version != "dev" {
				fmt.Fprintln(cmd.OutOrStdout())
				fmt.Fprintln(cmd.OutOrStdout(), "Update available:", buildinfo.Version, "->", release.TagName)
				fmt.Fprintln(cmd.OutOrStdout(), "Published at:", release.PublishedAt.Format(time.RFC3339))
			}

			return nil
		},
	}
}

type releaseInfo struct {
	TagName     string    `json:"tag_name"`
	PublishedAt time.Time `json:"published_at"`
}

func latestRelease(cmd *cobra.Command, client *http.Client, releaseURL string) (releaseInfo, error) {
	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, releaseURL, nil)
	if err != nil {
		return releaseInfo{}, fmt.Errorf("creating request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return releaseInfo{}, fmt.Errorf("fetching latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return releaseInfo{}, fmt.Errorf("no published release")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return releaseInfo{}, fmt.Errorf("fetching latest release: status code %d", resp.StatusCode)
	}

	var release releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return releaseInfo{}, fmt.Errorf("decoding latest release: %w", err)
	}
	if release.TagName == "" {
		return releaseInfo{}, fmt.Errorf("latest release response has no tag")
	}

	return release, nil
}
