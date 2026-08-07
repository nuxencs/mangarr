package cmd

import (
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

type dependencies struct {
	versionClient *http.Client
	releaseURL    string
}

func defaultDependencies() dependencies {
	return dependencies{
		versionClient: &http.Client{Timeout: 10 * time.Second},
		releaseURL:    githubURL,
	}
}

func NewRootCommand() *cobra.Command {
	return newRootCommand(defaultDependencies())
}

func newRootCommand(deps dependencies) *cobra.Command {
	rootOptions := &rootOptions{}
	downloadOptions := &downloadOptions{}

	root := &cobra.Command{
		Use:           "mangarr",
		Short:         "Download and monitor manga chapters from various providers.",
		SilenceErrors: true,
		Long: `Download and monitor manga chapters from various providers.

Provide a configuration file using one of the following methods:
1. Use the --config <path> or -c <path> flag.
2. Place config.yaml in the operating system user config directory under mangarr/.
3. Place config.yaml in ~/.mangarr/.
4. Place a config.yaml file in the directory of the binary.

For more information and examples, visit https://github.com/nuxencs/mangarr`,
	}

	initRootFlags(root, rootOptions)
	download := newDownloadCommand(downloadOptions)
	initDownloadFlags(download, downloadOptions)
	root.AddCommand(newVersionCommand(deps.versionClient, deps.releaseURL))
	root.AddCommand(download)
	root.AddCommand(newMonitorCommand(rootOptions))

	return root
}

func Execute() error {
	return NewRootCommand().Execute()
}
