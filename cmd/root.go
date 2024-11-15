package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mangarr",
	Short: "Download and monitor manga chapters from various providers.",
	Long: `Download and monitor manga chapters from various providers.

Provide a configuration file using one of the following methods:
1. Use the --config <path> or -c <path> flag.
2. Place a config.yaml file in the default user configuration directory (e.g., ~/.config/mangarr/).
3. Place a config.yaml file a folder inside your home directory (e.g., ~/.mangarr/).
4. Place a config.yaml file in the directory of the binary.

For more information and examples, visit https://github.com/nuxencs/mangarr`,
}

func init() {
	initRootFlags()
	initDownloadFlags()

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(monitorCmd)
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
