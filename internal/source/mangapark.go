package source

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mangarr/internal/browser"
	"mangarr/internal/domain"
	"mangarr/internal/sanitize"

	"github.com/go-rod/rod"
	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
)

const (
	mangaparkURL = "https://mangapark.net"
)

var mangaparkChapterNumberPattern = regexp.MustCompile(`(?:Chapter|Ch.) ?(\d+(\.\d+)?)`)

type mangapark struct {
	MangaURL  string
	Browser   *browser.Manager
	Collector colly.Collector
}

func NewMangaPark(mangaURL string, bm *browser.Manager) domain.Source {
	collector := colly.NewCollector(
		colly.AllowURLRevisit(),
	)
	extensions.RandomUserAgent(collector)

	collector.SetRequestTimeout(120 * time.Second)

	return &mangapark{
		Collector: *collector,
		Browser:   bm,
		MangaURL:  mangaURL,
	}
}

func (m *mangapark) String() string {
	return "MangaPark"
}

func (m *mangapark) ValidateInput() error {
	if len(m.MangaURL) == 0 {
		return fmt.Errorf("mangapark manga URL is required")
	}

	if !strings.HasPrefix(m.MangaURL, mangaparkURL) {
		return fmt.Errorf("the URL for Manga Park must start with %s", mangaparkURL)
	}

	if _, err := url.Parse(m.MangaURL); err != nil {
		return fmt.Errorf("parsing URL %s: %w", m.MangaURL, err)
	}

	return nil
}

// GetManga gets the selected manga from Manga Park
func (m *mangapark) GetManga(_ context.Context) (domain.Manga, error) {
	var manga domain.Manga
	var errors []error
	var assigned bool
	c := m.Collector.Clone()

	c.OnError(func(r *colly.Response, err error) {
		errors = append(errors, fmt.Errorf("requesting URL %s: %w", r.Request.URL, err))
	})

	c.OnHTML(".text-lg.font-bold", func(e *colly.HTMLElement) {
		if assigned {
			return
		}

		title := e.ChildText("a")

		manga = domain.Manga{
			Title:    sanitize.Filename(title),
			Chapters: make(map[float32]domain.Chapter),
		}
		assigned = true
	})

	err := c.Visit(m.MangaURL)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("visiting URL %s: %w", m.MangaURL, err)
	}

	if len(errors) > 0 {
		return domain.Manga{}, fmt.Errorf("processing %d URLs: %w", len(errors), errors[0])
	}

	return manga, nil
}

// GetChapters gets all chapters for a manga
func (m *mangapark) GetChapters(_ context.Context, manga domain.Manga) error {
	var errors []error
	c := m.Collector.Clone()

	c.OnError(func(r *colly.Response, err error) {
		errors = append(errors, fmt.Errorf("requesting URL %s: %w", r.Request.URL, err))
	})

	c.OnHTML("div.px-2.py-2.flex", func(e *colly.HTMLElement) {
		firstChild := e.DOM.Children().First()
		chapterURL := firstChild.Find("a").AttrOr("href", "")

		name := firstChild.Find("a").Text()
		number, err := m.getChapterNumber(name)
		if err != nil {
			// Skip chapters that don't match the regex pattern instead of failing
			return
		}

		titleSpan := firstChild.Find("span.opacity-80")
		title := ""
		if titleSpan.Length() > 0 {
			title = strings.TrimPrefix(titleSpan.Text(), ": ")
			title = strings.TrimSpace(title)
		}

		manga.Chapters[number] = domain.Chapter{
			URL:    chapterURL,
			Number: number,
			Title:  sanitize.Filename(title),
		}
	})

	err := c.Visit(m.MangaURL)
	if err != nil {
		return fmt.Errorf("visiting URL %s: %w", m.MangaURL, err)
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
func (m *mangapark) GetImageURLs(_ context.Context, chapter *domain.Chapter) error {
	var imageInfos []domain.ImageInfo
	var imageElements rod.Elements
	var errors []error

	path, err := url.JoinPath(mangaparkURL, chapter.URL)
	if err != nil {
		return fmt.Errorf("building URL: %w", err)
	}

	page := m.Browser.Get().MustPage()
	defer page.MustClose()

	pageWithTimeout := page.Timeout(browser.Timeout)

	err = rod.Try(func() {
		_ = pageWithTimeout.MustNavigate(path).WaitDOMStable(time.Second, 0)

		imageElements = pageWithTimeout.MustElements("div[data-name='image-item'] img")
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
func (m *mangapark) getChapterNumber(name string) (float32, error) {
	// FindSubmatch returns an array where the first element is the full match, and the rest are submatches.
	matches := mangaparkChapterNumberPattern.FindStringSubmatch(name)

	if len(matches) <= 1 {
		return 0, fmt.Errorf("finding matches in %s", name)
	}

	number, err := strconv.ParseFloat(matches[1], 32)
	if err != nil {
		return 0, fmt.Errorf("parsing chapter number from %s: %w", name, err)
	}

	return float32(number), nil
}
