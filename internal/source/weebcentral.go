package source

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/sanitize"
	"mangarr/internal/sharedhttp"

	"github.com/PuerkitoBio/goquery"
	"github.com/avast/retry-go"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
)

const (
	weebcentralURL = "https://weebcentral.com"
)

var weebcentralChapterNumberPattern = regexp.MustCompile(`(?:Chapter|Ch.) ?(\d+(\.\d+)?)`)

type weebcentral struct {
	MangaURL  string
	Collector *colly.Collector
	Client    http.Client
	BaseURL   string
}

func NewWeebCentral(mangaURL string) domain.Source {
	collector := colly.NewCollector(
		colly.AllowURLRevisit(),
	)
	extensions.RandomUserAgent(collector)

	collector.SetRequestTimeout(120 * time.Second)

	return &weebcentral{
		Collector: collector,
		MangaURL:  mangaURL,
		BaseURL:   weebcentralURL,
		Client: http.Client{
			Timeout:   120 * time.Second,
			Transport: sharedhttp.Transport,
		},
	}
}

func (w *weebcentral) String() string {
	return "Weeb Central"
}

func (w *weebcentral) ValidateInput() error {
	if len(w.MangaURL) == 0 {
		return fmt.Errorf("weebcentral manga URL is required")
	}

	parsed, err := url.Parse(w.MangaURL)
	if err != nil {
		return fmt.Errorf("parsing URL %s: %w", w.MangaURL, err)
	}
	if parsed.Scheme != "https" || !strings.EqualFold(parsed.Host, "weebcentral.com") {
		return fmt.Errorf("the URL for Weeb Central must use %s", weebcentralURL)
	}

	return nil
}

// GetManga gets the selected manga from Weeb Central
func (w *weebcentral) GetManga(ctx context.Context) (domain.Manga, error) {
	var manga domain.Manga
	var errors []error
	c := w.Collector.Clone()
	c.Context = ctx

	c.OnError(func(r *colly.Response, err error) {
		errors = append(errors, fmt.Errorf("requesting URL %s: %w", r.Request.URL, err))
	})

	c.OnHTML("h1.hidden", func(e *colly.HTMLElement) {
		manga = domain.Manga{
			Title:    sanitize.Filename(e.Text),
			Chapters: make(map[domain.ChapterNumber]domain.Chapter),
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
func (w *weebcentral) GetChapters(ctx context.Context, manga domain.Manga) error {
	var errors []error
	c := w.Collector.Clone()
	c.Context = ctx

	c.OnError(func(r *colly.Response, err error) {
		errors = append(errors, fmt.Errorf("requesting URL %s: %w", r.Request.URL, err))
	})

	c.OnHTML("a.flex", func(e *colly.HTMLElement) {
		chapterURL, err := resolveAgainstBase(w.BaseURL, e.Attr("href"))
		if err != nil {
			errors = append(errors, fmt.Errorf("resolving chapter URL: %w", err))
			return
		}

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
func (w *weebcentral) GetImageURLs(ctx context.Context, chapter *domain.Chapter) error {
	imageURL, err := w.chapterImagesURL(chapter.URL)
	if err != nil {
		return fmt.Errorf("building chapter images URL from %s: %w", chapter.URL, err)
	}

	body, err := w.fetch(ctx, imageURL)
	if err != nil {
		return fmt.Errorf("fetching chapter images %s: %w", imageURL, err)
	}

	imageURLs, err := w.extractImageURLs(body)
	if err != nil {
		return fmt.Errorf("extracting chapter image URLs from %s: %w", imageURL, err)
	}

	if len(imageURLs) == 0 {
		return fmt.Errorf("getting image URLs for chapter %s", chapter.Number)
	}

	imageInfos := make([]domain.ImageInfo, 0, len(imageURLs))
	for _, imageURL := range imageURLs {
		imageInfos = append(imageInfos, domain.ImageInfo{ImageURL: imageURL})
	}

	chapter.ImageInfo = imageInfos
	return nil
}

func (w *weebcentral) chapterImagesURL(chapterURL string) (string, error) {
	parsed, err := url.Parse(chapterURL)
	if err != nil {
		return "", err
	}

	parsed.RawQuery = ""
	parsed.Fragment = ""

	imageURL, err := url.JoinPath(parsed.String(), "images")
	if err != nil {
		return "", err
	}

	u, err := url.Parse(imageURL)
	if err != nil {
		return "", err
	}

	params := u.Query()
	params.Set("is_prev", "False")
	params.Set("current_page", "1")
	params.Set("reading_style", "long_strip")
	u.RawQuery = params.Encode()

	return u.String(), nil
}

func (w *weebcentral) fetch(ctx context.Context, rawURL string) ([]byte, error) {
	var body []byte

	err := retry.Do(func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return fmt.Errorf("creating request for %s: %w", rawURL, err)
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; mangarr/1.0; +https://github.com/nuxencs/mangarr)")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

		resp, err := sharedhttp.ExecRequest(w.Client, req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("reading response body from %s: %w", rawURL, err)
		}

		return nil
	}, sharedhttp.RetryOptions(ctx)...)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (w *weebcentral) extractImageURLs(body []byte) ([]string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	imageURLs := make([]string, 0)
	doc.Find("img[src]").Each(func(_ int, selection *goquery.Selection) {
		src := strings.TrimSpace(selection.AttrOr("src", ""))
		if len(src) == 0 {
			return
		}

		resolvedURL, err := resolveAgainstBase(w.BaseURL, src)
		if err != nil {
			return
		}

		imageURLs = append(imageURLs, resolvedURL)
	})

	return dedupeStrings(imageURLs), nil
}

// getChapterNumber gets the chapter number from the scraped chapter name
func (w *weebcentral) getChapterNumber(name string) (domain.ChapterNumber, error) {
	// FindSubmatch returns an array where the first element is the full match, and the rest are submatches.
	matches := weebcentralChapterNumberPattern.FindStringSubmatch(name)

	if len(matches) <= 1 {
		return domain.ChapterNumber{}, fmt.Errorf("finding matches in %s", name)
	}

	number, err := domain.ParseChapterNumber(matches[1])
	if err != nil {
		return domain.ChapterNumber{}, fmt.Errorf("parsing chapter number from %s: %w", name, err)
	}

	return number, nil
}
