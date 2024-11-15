package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"mangarr/internal/buildinfo"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const githubURL = "https://api.github.com/repos/nuxencs/mangarr/releases/latest"

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version info",
	Run: func(cmd *cobra.Command, _ []string) {
		ctx := cmd.Context()

		fmt.Println("Version:", buildinfo.Version)
		fmt.Println("Commit:", buildinfo.Commit)
		fmt.Println("Build date:", buildinfo.Date)
		fmt.Println()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubURL, nil)
		if err != nil {
			fmt.Println("Failed to create request:", err)
			os.Exit(1)
		}

		// get the latest release tag from api
		client := http.Client{
			Timeout: 10 * time.Second,
		}

		resp, err := client.Do(req)
		if err != nil {
			if errors.Is(err, http.ErrHandlerTimeout) {
				fmt.Println("Server timed out while fetching latest release from api")
			} else {
				fmt.Println("Failed to fetch latest release from api:", err)
			}
			os.Exit(1)
		}
		defer resp.Body.Close()

		// api returns 500 instead of 404 here
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusInternalServerError {
			fmt.Println("No release found")
			os.Exit(1)
		}

		var rel struct {
			TagName     string    `json:"tag_name"`
			PublishedAt time.Time `json:"published_at"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
			fmt.Println("Failed to decode response from api:", err)
			os.Exit(1)
		}

		if rel.TagName != buildinfo.Version && buildinfo.Version != "dev" {
			fmt.Println("Update available:", buildinfo.Version, "->", rel.TagName)
			fmt.Println("Published at:", rel.PublishedAt.Format(time.RFC3339))
		}
	},
}
