package cmd

import "github.com/spf13/cobra"

const (
	maxConcurrentChapterProcesses = 10
	maxConcurrentSourceProcesses  = 10
)

type rootOptions struct {
	configPath string
}

type downloadOptions struct {
	naming            string
	downloadDirectory string
	mangaSource       string
	overwrite         string
	manga             string
	group             string
	language          string
	chapterNumbers    string
	first             bool
	latest            bool
	downloadAll       bool
}

func initRootFlags(root *cobra.Command, options *rootOptions) {
	root.PersistentFlags().StringVarP(
		&options.configPath,
		"config",
		"c",
		"",
		"specifies the path to your config file",
	)
}

func initDownloadFlags(download *cobra.Command, options *downloadOptions) {
	download.Flags().StringVarP(
		&options.downloadDirectory,
		"downloadDirectory",
		"d",
		"",
		"specifies the directory where you want to save your downloads to",
	)
	download.Flags().StringVarP(
		&options.mangaSource,
		"source",
		"s",
		"",
		"specifies the source of the manga",
	)
	download.Flags().StringVarP(
		&options.naming,
		"naming",
		"n",
		"{manga:<.>} Ch. {num:3}{title: - <.>}",
		"specifies the naming template you want to use for naming chapters",
	)
	download.Flags().StringVarP(
		&options.overwrite,
		"overwrite",
		"o",
		"",
		"overwrites the parsed manga name",
	)

	download.Flags().StringVarP(
		&options.manga,
		"manga",
		"m",
		"",
		"specifies the manga you want to download",
	)
	download.Flags().StringVarP(
		&options.group,
		"group",
		"g",
		"",
		"specifies the group you want to download the chapter from",
	)
	download.Flags().StringVarP(
		&options.language,
		"language",
		"l",
		"en",
		"specifies the language you want to download. default: en",
	)

	download.Flags().StringVarP(
		&options.chapterNumbers,
		"chapters",
		"C",
		"",
		"specifies the chapter numbers you want to download",
	)
	download.Flags().BoolVarP(
		&options.first,
		"first",
		"1",
		false,
		"download the first chapter",
	)
	download.Flags().BoolVarP(
		&options.latest,
		"latest",
		"L",
		false,
		"download the latest chapter",
	)
	download.Flags().BoolVarP(
		&options.downloadAll,
		"all",
		"A",
		false,
		"download all available chapters",
	)

	download.MarkFlagsMutuallyExclusive("first", "chapters")
	download.MarkFlagsMutuallyExclusive("latest", "chapters")
	download.MarkFlagsMutuallyExclusive("first", "latest")
	download.MarkFlagsMutuallyExclusive("all", "chapters")
	download.MarkFlagsMutuallyExclusive("all", "first")
	download.MarkFlagsMutuallyExclusive("all", "latest")

	_ = download.MarkFlagRequired("downloadDirectory")
	_ = download.MarkFlagRequired("source")
	_ = download.MarkFlagRequired("manga")
}
