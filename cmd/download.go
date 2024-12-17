package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"mangarr/internal/domain"
	"mangarr/internal/download"
	"mangarr/internal/files"
	"mangarr/internal/parse"
	"mangarr/internal/sanitize"
	"mangarr/internal/semaphore"
	"mangarr/internal/source"
	"mangarr/internal/templater"

	"github.com/spf13/cobra"
)

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download a specified chapter",
	Run: func(cmd *cobra.Command, _ []string) {
		ctx := cmd.Context()

		if !cmd.Flags().Changed("first") && !cmd.Flags().Changed("chapters") {
			latest = true
		}

		if err := files.IsValidLocation(downloadDirectory); err != nil {
			fmt.Println("Invalid location:", err)
			return
		}

		var s domain.Source

		switch mangaSource {
		case "tcbscans":
			s = source.NewTCBScans(manga)
		case "mangadex":
			s = source.NewMangadex(manga, group, language)
		case "mangaplus":
			s = source.NewMangaPlus(manga)
		case "flamecomics":
			s = source.NewFlamecomics(manga)
		case "asurascans":
			s = source.NewAsurascans(manga)
		case "cubari":
			s = source.NewCubari(manga, group)
		default:
			fmt.Println("Invalid source:", mangaSource)
			return
		}

		if err := s.ValidateInput(); err != nil {
			fmt.Println("Invalid input:", err)
			return
		}

		selectedManga, err := s.GetManga(ctx)
		if err != nil {
			fmt.Printf("Failed to get manga from %s: %v\n", s, err)
			return
		}

		if err := s.GetChapters(ctx, selectedManga); err != nil {
			fmt.Printf("Failed to get chapters for %s: %v\n", selectedManga.Title, err)
			return
		}

		var selectedChapterNumbers []float32

		firstChapterNr, latestChapterNr, err := parse.GetMinAndMaxKeys(selectedManga.Chapters)
		if err != nil {
			fmt.Printf("Failed to parse chapter number for %s: %v\n", selectedManga.Title, err)
			return
		}

		switch {
		case first:
			selectedChapterNumbers = firstChapterNr
		case latest:
			selectedChapterNumbers = latestChapterNr
		default:
			selectedChapterNumbers, err = parse.ChapterSelection(chapterNumbers, selectedManga.Chapters)
			if err != nil {
				fmt.Printf("Failed to parse chapter selection for %s: %v\n", selectedManga.Title, err)
				return
			}
		}

		if len(selectedChapterNumbers) == 0 {
			fmt.Printf("Failed to find matching chapters in range %s for %s\n", chapterNumbers, selectedManga.Title)
			return
		}

		// semaphore to limit concurrency to maxConcurrentChapterProcesses which is set to 10
		sem := semaphore.NewWeighted(maxConcurrentChapterProcesses)
		wg := sync.WaitGroup{}

		for _, num := range selectedChapterNumbers {
			wg.Add(1)

			go func() {
				sem.Acquire()
				defer func() { sem.Release(); wg.Done() }()

				selectedChapter, ok := selectedManga.Chapters[num]
				if !ok {
					fmt.Println("Failed to find chapter with number", num)
					return
				}

				if err := s.GetImageURLs(ctx, &selectedChapter); err != nil {
					fmt.Printf("Failed to get image URLs for chapter %g: %v\n", selectedChapter.Number, err)
					return
				}

				overwrittenTitle := sanitize.Filename(overwrite)

				if len(overwrittenTitle) != 0 {
					selectedManga.Title = overwrittenTitle
				}

				t := templater.New(selectedManga, selectedChapter)
				templatedName := t.ExecTemplate(naming)

				chapterFolder := sanitize.Filename(templatedName)
				contentPath := filepath.Join(downloadDirectory, selectedManga.Title, chapterFolder+".cbz")

				if _, err := os.Stat(contentPath); err == nil {
					fmt.Println("Chapter has already been downloaded, skipping", templatedName)
					return
				}

				fmt.Printf("Downloading %q...\n", templatedName)
				if err := download.Chapter(ctx, contentPath, selectedChapter, selectedManga.IsManhwa); err != nil {
					fmt.Printf("Failed to download chapter %q: %v\n", templatedName, err)
					return
				}

				fmt.Println("Finished downloading", templatedName)
			}()
		}

		wg.Wait()
	},
}
