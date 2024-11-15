package source

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/sanitize"

	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
)

const (
	tcbscansURL = "https://tcbscans.me"
)

var chapterNumberPattern = regexp.MustCompile(`Chapter (\d+(\.\d+)?)`)

type tcbscans struct {
	MangaTitle string
	Collector  colly.Collector
}

func NewTCBScans(mangaTitle string) domain.Source {
	collector := colly.NewCollector(
		colly.AllowURLRevisit(),
	)
	extensions.RandomUserAgent(collector)

	collector.SetRequestTimeout(120 * time.Second)

	return &tcbscans{
		Collector:  *collector,
		MangaTitle: mangaTitle,
	}
}

func (t *tcbscans) String() string {
	return "TCB Scans"
}

func (t *tcbscans) ValidateInput() error {
	if len(t.MangaTitle) == 0 {
		return fmt.Errorf("tcbscans manga title is required")
	}

	return nil
}

// GetManga gets the selected manga from TCB Scans
func (t *tcbscans) GetManga(_ context.Context) (domain.Manga, error) {
	mangas := make(map[string]domain.Manga)
	c := t.Collector.Clone()

	c.OnHTML("div.bg-card.border.border-border.rounded.p-3.mb-3", func(e *colly.HTMLElement) {
		mangaURL := e.ChildAttr("a", "href")
		name := e.ChildAttr("img", "alt")

		mangas[name] = domain.Manga{
			URL:      mangaURL,
			Title:    sanitize.Filename(name),
			Chapters: make(map[float32]domain.Chapter),
		}
	})

	path, err := url.JoinPath(tcbscansURL, "projects")
	if err != nil {
		return domain.Manga{}, err
	}

	err = c.Visit(path)
	if err != nil {
		return domain.Manga{}, err
	}

	selectedManga, ok := mangas[t.MangaTitle]
	if !ok {
		return domain.Manga{}, fmt.Errorf("failed to get manga for provided name: %s", t.MangaTitle)
	}

	return selectedManga, nil
}

// GetChapters gets all chapters for a manga
func (t *tcbscans) GetChapters(_ context.Context, manga domain.Manga) error {
	c := t.Collector.Clone()

	c.OnHTML("a.block.border.border-border.bg-card.mb-3.p-3.rounded", func(e *colly.HTMLElement) {
		chapterURL := e.Attr("href")

		name := strings.TrimSpace(e.ChildText("div.text-lg.font-bold"))
		number, err := t.getChapterNumber(name)
		if err != nil {
			return
		}

		title := sanitize.Filename(e.ChildText("div.text-gray-500"))

		manga.Chapters[number] = domain.Chapter{
			URL:    chapterURL,
			Number: number,
			Title:  title,
		}
	})

	path, err := url.JoinPath(tcbscansURL, manga.URL)
	if err != nil {
		return err
	}

	err = c.Visit(path)
	if err != nil {
		return err
	}

	if len(manga.Chapters) == 0 {
		return fmt.Errorf("failed to get chapters for manga: %s", manga.Title)
	}

	return nil
}

// GetImageURLs gets all image urls for a chapter
func (t *tcbscans) GetImageURLs(_ context.Context, chapter *domain.Chapter) error {
	c := t.Collector.Clone()

	var imageInfos []domain.ImageInfo

	c.OnHTML("img.fixed-ratio-content", func(e *colly.HTMLElement) {
		imageInfos = append(imageInfos, domain.ImageInfo{ImageURL: e.Attr("src")})
	})

	path, err := url.JoinPath(tcbscansURL, chapter.URL)
	if err != nil {
		return err
	}

	err = c.Visit(path)
	if err != nil {
		return err
	}

	if len(imageInfos) == 0 {
		return fmt.Errorf("failed to get image urls for chapter number: %g", chapter.Number)
	}

	chapter.ImageInfo = imageInfos
	return nil
}

// getChapterNumber gets the chapter number from the scraped chapter name
func (t *tcbscans) getChapterNumber(name string) (float32, error) {
	var number float64
	var err error

	// FindSubmatch returns an array where the first element is the full match, and the rest are submatches.
	matches := chapterNumberPattern.FindStringSubmatch(name)
	if len(matches) > 1 {
		number, err = strconv.ParseFloat(matches[1], 32)
		if err != nil {
			return 0, err
		}
		return float32(number), nil
	}

	return 0, err
}
