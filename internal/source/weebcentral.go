package source

import (
	"context"
	"fmt"
	"github.com/go-rod/rod"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mangarr/internal/browser"
	"mangarr/internal/domain"
	"mangarr/internal/sanitize"

	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
)

const (
	weebcentralURL = "https://weebcentral.com"
)

var weebcentralChapterNumberPattern = regexp.MustCompile(`(?:Chapter|Ch.) ?(\d+(\.\d+)?)`)

type weebcentral struct {
	MangaURL  string
	Browser   *browser.Manager
	Collector colly.Collector
}

func NewWeebCentral(mangaURL string, bm *browser.Manager) domain.Source {
	collector := colly.NewCollector(
		colly.AllowURLRevisit(),
	)
	extensions.RandomUserAgent(collector)

	collector.SetRequestTimeout(120 * time.Second)

	return &weebcentral{
		Collector: *collector,
		Browser:   bm,
		MangaURL:  mangaURL,
	}
}

func (w *weebcentral) String() string {
	return "Weeb Central"
}

func (w *weebcentral) ValidateInput() error {
	if len(w.MangaURL) == 0 {
		return fmt.Errorf("weebcentral manga URL is required")
	}

	if !strings.HasPrefix(w.MangaURL, weebcentralURL) {
		return fmt.Errorf("the URL for Weeb Central must start with %s", mangaparkURL)
	}

	if _, err := url.Parse(w.MangaURL); err != nil {
		return fmt.Errorf("parsing URL %s: %w", w.MangaURL, err)
	}

	return nil
}

// GetManga gets the selected manga from Manga Park
func (w *weebcentral) GetManga(_ context.Context) (domain.Manga, error) {
	var manga domain.Manga
	var errors []error
	c := w.Collector.Clone()

	c.OnError(func(r *colly.Response, err error) {
		errors = append(errors, fmt.Errorf("requesting URL %s: %w", r.Request.URL, err))
	})

	c.OnHTML("h1.hidden", func(e *colly.HTMLElement) {
		manga = domain.Manga{
			Title:    sanitize.Filename(e.Text),
			Chapters: make(map[float32]domain.Chapter),
		}
	})

	err := c.Visit(w.MangaURL)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("visiting URL %s: %w", w.MangaURL, err)
	}

	if len(errors) > 0 {
		return domain.Manga{}, fmt.Errorf("processing %d URLs: %w", len(errors), errors[0])
	}

	return manga, nil
}

// GetChapters gets all chapters for a manga
func (w *weebcentral) GetChapters(_ context.Context, manga domain.Manga) error {
	var errors []error
	c := w.Collector.Clone()

	c.OnError(func(r *colly.Response, err error) {
		errors = append(errors, fmt.Errorf("requesting URL %s: %w", r.Request.URL, err))
	})

	c.OnHTML("a.flex", func(e *colly.HTMLElement) {
		chapterURL := e.Attr("href")
		name := e.ChildText("span.grow")
		number, err := w.getChapterNumber(name)
		if err != nil {
			// Skip chapters that don't match the regex pattern instead of failing
			return
		}

		manga.Chapters[number] = domain.Chapter{
			URL:    chapterURL,
			Number: number,
		}
	})

	path, err := url.JoinPath(w.MangaURL, "full-chapter-list")
	if err != nil {
		return fmt.Errorf("building URL: %w", err)
	}

	err = c.Visit(path)
	if err != nil {
		return fmt.Errorf("visiting URL %s: %w", w.MangaURL, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("processing %d URLs: %w", len(errors), errors[0])
	}

	if len(manga.Chapters) == 0 {
		return fmt.Errorf("getting chapters for manga %s", manga.Title)
	}

	return nil
}

// GetImageURLs gets all image urls for a chapter
func (w *weebcentral) GetImageURLs(_ context.Context, chapter *domain.Chapter) error {
	var imageInfos []domain.ImageInfo
	var imageElements rod.Elements
	var errors []error

	page := w.Browser.Get().MustPage()
	defer page.MustClose()

	pageWithTimeout := page.Timeout(browser.Timeout)

	err := rod.Try(func() {
		_ = pageWithTimeout.MustNavigate(chapter.URL).WaitDOMStable(time.Second, 1)

		imageElements = pageWithTimeout.MustElements("img.maw-w-full")
	})
	if err != nil {
		return fmt.Errorf("opening image page: %w", browser.HandleError(err))
	}

	for _, e := range imageElements {
		imgURL, err := e.Attribute("src")
		if err != nil {
			errors = append(errors, fmt.Errorf("getting image URL: %w", err))
		}

		if imgURL == nil {
			continue
		}

		imageInfos = append(imageInfos, domain.ImageInfo{ImageURL: *imgURL})
	}

	if len(errors) > 0 {
		return fmt.Errorf("processing %d URLs: %w", len(errors), errors[0])
	}

	if len(imageInfos) == 0 {
		return fmt.Errorf("getting image URLs for chapter %g", chapter.Number)
	}

	chapter.ImageInfo = imageInfos
	return nil
}

// getChapterNumber gets the chapter number from the scraped chapter name
func (w *weebcentral) getChapterNumber(name string) (float32, error) {
	// FindSubmatch returns an array where the first element is the full match, and the rest are submatches.
	matches := weebcentralChapterNumberPattern.FindStringSubmatch(name)

	if len(matches) <= 1 {
		return 0, fmt.Errorf("finding matches in %s", name)
	}

	number, err := strconv.ParseFloat(matches[1], 32)
	if err != nil {
		return 0, fmt.Errorf("parsing chapter number from %s: %w", name, err)
	}

	return float32(number), nil
}
