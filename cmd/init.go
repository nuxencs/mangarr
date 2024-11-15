package cmd

var (
	configPath        string
	naming            string
	downloadDirectory string
	mangaSource       string

	manga    string
	group    string
	language string

	chapterNumbers string
	first          bool
	latest         bool
)

func initRootFlags() {
	rootCmd.PersistentFlags().StringVarP(
		&configPath,
		"config",
		"c",
		"",
		"specifies the path to your config file",
	)
}

func initDownloadFlags() {
	downloadCmd.Flags().StringVarP(
		&downloadDirectory,
		"downloadDirectory",
		"d",
		"",
		"specifies the directory where you want to save your downloads to",
	)
	downloadCmd.Flags().StringVarP(
		&mangaSource,
		"source",
		"s",
		"",
		"specifies the source of the manga",
	)
	downloadCmd.Flags().StringVarP(
		&naming,
		"naming",
		"n",
		"{manga:<.>} Ch. {num:3}{title: - <.>}",
		"specifies the naming template you want to use for naming chapters",
	)

	downloadCmd.Flags().StringVarP(
		&manga,
		"manga",
		"m",
		"",
		"specifies the manga you want to download",
	)

	downloadCmd.Flags().StringVarP(
		&group,
		"group",
		"g",
		"",
		"specifies the group you want to download the chapter from",
	)
	downloadCmd.Flags().StringVarP(
		&language,
		"language",
		"l",
		"en",
		"specifies the language you want to download. default: en",
	)

	downloadCmd.Flags().StringVarP(
		&chapterNumbers,
		"chapters",
		"C",
		"",
		"specifies the chapter numbers you want to download",
	)
	downloadCmd.Flags().BoolVarP(
		&first,
		"first",
		"1",
		false,
		"download the first chapter",
	)
	downloadCmd.Flags().BoolVarP(
		&latest,
		"latest",
		"L",
		false,
		"download the latest chapter",
	)

	downloadCmd.MarkFlagsMutuallyExclusive("first", "chapters")
	downloadCmd.MarkFlagsMutuallyExclusive("latest", "chapters")
	downloadCmd.MarkFlagsMutuallyExclusive("first", "latest")

	_ = downloadCmd.MarkFlagRequired("downloadDirectory")
	_ = downloadCmd.MarkFlagRequired("source")
	_ = downloadCmd.MarkFlagRequired("manga")
}
