package source

import (
	"context"
	"fmt"
	"html"
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

const asurascansBaseURL = "https://asurascans.com"

var (
	asurascansChapterAssetPattern       = regexp.MustCompile(`https://cdn\.asurascans\.com/asura-images/chapters/[^"&<]+`)
	asurascansUnavailableChapterPattern = regexp.MustCompile(`"number":\[0,(\d+(?:\.\d+)?)\][^}]*"(?:is_locked|is_premium)":\[0,true\]`)
)

type asurascans struct {
	MangaURL  string
	Collector *colly.Collector
	Client    http.Client
	BaseURL   string
}

func NewAsurascans(mangaURL string) domain.Source {
	collector := colly.NewCollector(
		colly.AllowURLRevisit(),
	)
	extensions.RandomUserAgent(collector)
	collector.SetRequestTimeout(120 * time.Second)

	return &asurascans{
		MangaURL:  mangaURL,
		Collector: collector,
		BaseURL:   asurascansBaseURL,
		Client: http.Client{
			Timeout:   120 * time.Second,
			Transport: sharedhttp.Transport,
		},
	}
}

func (a *asurascans) String() string {
	return "Asura Scans"
}

func (a *asurascans) ValidateInput() error {
	if len(a.MangaURL) == 0 {
		return fmt.Errorf("asurascans manga URL is required")
	}

	parsed, err := url.Parse(a.MangaURL)
	if err != nil {
		return fmt.Errorf("parsing URL %s: %w", a.MangaURL, err)
	}

	if strings.ToLower(parsed.Host) != "asurascans.com" {
		return fmt.Errorf("the URL for Asura Scans must start with %s", asurascansBaseURL)
	}

	if !strings.HasPrefix(parsed.Path, "/comics/") {
		return fmt.Errorf("the URL for Asura Scans must point to /comics/...")
	}

	return nil
}

func (a *asurascans) Discover(ctx context.Context) (domain.Manga, error) {
	parsed, err := url.Parse(a.MangaURL)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("parsing URL %s: %w", a.MangaURL, err)
	}

	resolvedURL := stripURLQueryAndFragment(parsed)
	manga := domain.Manga{
		URL:      resolvedURL,
		Chapters: make(map[domain.ChapterNumber]domain.Chapter),
		IsManhwa: true,
	}

	var errors []error
	c := a.Collector.Clone()
	c.Context = ctx

	c.OnError(func(r *colly.Response, err error) {
		errors = append(errors, fmt.Errorf("requesting URL %s: %w", r.Request.URL, err))
	})

	c.OnHTML("h1", func(e *colly.HTMLElement) {
		if len(manga.Title) != 0 {
			return
		}

		title := sanitize.Filename(strings.TrimSpace(html.UnescapeString(e.Text)))
		if len(title) != 0 {
			manga.Title = title
		}
	})

	lockedChapters := make(map[domain.ChapterNumber]struct{})

	c.OnHTML("astro-island", func(e *colly.HTMLElement) {
		componentURL := e.Attr("component-url")
		if !strings.Contains(componentURL, "ChapterList") {
			return
		}

		props := html.UnescapeString(e.Attr("props"))
		for _, match := range asurascansUnavailableChapterPattern.FindAllStringSubmatch(props, -1) {
			chapterNum, err := domain.ParseChapterNumber(match[1])
			if err != nil {
				continue
			}
			lockedChapters[chapterNum] = struct{}{}
		}
	})

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		href := strings.TrimSpace(e.Attr("href"))
		if !strings.HasPrefix(href, "/comics/") || !strings.Contains(href, "/chapter/") {
			return
		}

		chapterNum, title, err := a.chapterDetailsFromLink(href, e)
		if err != nil {
			return
		}

		chapterURL, err := resolveAgainstBase(a.BaseURL, href)
		if err != nil {
			errors = append(errors, fmt.Errorf("building chapter URL from %s: %w", href, err))
			return
		}

		manga.Chapters[chapterNum] = domain.Chapter{
			URL:    chapterURL,
			Number: chapterNum,
			Title:  sanitize.Filename(title),
		}
	})

	if err := c.Visit(resolvedURL); err != nil {
		return domain.Manga{}, fmt.Errorf("visiting URL %s: %w", resolvedURL, err)
	}

	if len(errors) > 0 {
		return domain.Manga{}, fmt.Errorf("processing %d URLs: %w", len(errors), errors[0])
	}

	for num := range lockedChapters {
		delete(manga.Chapters, num)
	}

	if len(manga.Title) == 0 {
		return domain.Manga{}, fmt.Errorf("getting manga for URL %s", resolvedURL)
	}

	if len(manga.Chapters) == 0 {
		return domain.Manga{}, fmt.Errorf("getting chapters for manga %s", manga.Title)
	}

	return manga, nil
}

func (a *asurascans) Pages(ctx context.Context, chapter domain.Chapter) ([]domain.ImageInfo, error) {
	body, err := a.fetch(ctx, chapter.URL)
	if err != nil {
		return nil, fmt.Errorf("fetching chapter page %s: %w", chapter.URL, err)
	}

	matches := asurascansChapterAssetPattern.FindAllString(string(body), -1)
	imageURLs := dedupeStrings(matches)
	if len(imageURLs) == 0 {
		return nil, fmt.Errorf("getting image URLs for chapter %s", chapter.Number)
	}

	imageInfos := make([]domain.ImageInfo, 0, len(imageURLs))
	for _, imageURL := range imageURLs {
		imageInfos = append(imageInfos, domain.ImageInfo{ImageURL: imageURL})
	}

	return imageInfos, nil
}

func (a *asurascans) fetch(ctx context.Context, rawURL string) ([]byte, error) {
	var body []byte

	err := retry.Do(func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return fmt.Errorf("creating request for %s: %w", rawURL, err)
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; mangarr/1.0; +https://github.com/nuxencs/mangarr)")
		req.Header.Set("Accept", "text/html,application/json;q=0.9,*/*;q=0.8")

		resp, err := sharedhttp.ExecRequest(a.Client, req)
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

func (a *asurascans) chapterDetailsFromLink(href string, e *colly.HTMLElement) (domain.ChapterNumber, string, error) {
	title := strings.TrimSpace(html.UnescapeString(e.DOM.Find("span.block.truncate").First().Text()))

	texts := make([]string, 0, 3)
	e.DOM.Find("span").Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(html.UnescapeString(s.Text()))
		if len(text) != 0 {
			texts = append(texts, text)
		}
	})

	if len(title) == 0 && len(texts) >= 3 {
		title = texts[1]
	}

	if len(texts) == 0 {
		return domain.ChapterNumber{}, "", fmt.Errorf("missing chapter text for %s", href)
	}

	headline := strings.TrimSpace(strings.TrimPrefix(texts[0], "Chapter "))
	chapterNum, err := domain.ParseChapterNumber(headline)
	if err != nil {
		return domain.ChapterNumber{}, "", fmt.Errorf("parsing chapter number from %s: %w", headline, err)
	}

	return chapterNum, title, nil
}

func stripURLQueryAndFragment(parsed *url.URL) string {
	clean := *parsed
	clean.RawQuery = ""
	clean.Fragment = ""
	return clean.String()
}

func resolveAgainstBase(baseURL, ref string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	relative, err := url.Parse(ref)
	if err != nil {
		return "", err
	}

	return base.ResolveReference(relative).String(), nil
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}

		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}
