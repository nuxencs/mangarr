package source

import (
	"cmp"
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/sanitize"
	"mangarr/internal/sharedhttp"
)

const (
	asurascansBaseURL = "https://asurascans.com"
	// Since 2026-11-10, Cloudflare has challenged every asurascans.com page. The JSON API of the site is still public.
	asurascansAPIURL = "https://api.asurascans.com"
)

type asurascans struct {
	MangaURL string
	Client   http.Client
	APIURL   string
}

func NewAsurascans(mangaURL string) domain.Source {
	return &asurascans{
		MangaURL: mangaURL,
		APIURL:   asurascansAPIURL,
		Client: http.Client{
			Timeout:   120 * time.Second,
			Transport: sharedhttp.Transport,
		},
	}
}

// Unlike the chapter endpoints, the series endpoint has no "data" wrapper.
type asurascansSeriesResponse struct {
	Series struct {
		Title string `json:"title"`
	} `json:"series"`
}

type asurascansChapterListResponse struct {
	Data []struct {
		Number     float64 `json:"number"`
		Title      string  `json:"title"`
		Slug       string  `json:"slug"`
		SeriesSlug string  `json:"series_slug"`
		IsPremium  bool    `json:"is_premium"`
	} `json:"data"`
}

type asurascansChapterResponse struct {
	Data struct {
		Chapter struct {
			Pages []struct {
				URL string `json:"url"`
			} `json:"pages"`
		} `json:"chapter"`
	} `json:"data"`
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

	if _, err := asurascansSeriesSlug(parsed); err != nil {
		return err
	}

	return nil
}

func (a *asurascans) Discover(ctx context.Context) (domain.Manga, error) {
	parsed, err := url.Parse(a.MangaURL)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("parsing URL %s: %w", a.MangaURL, err)
	}

	slug, err := asurascansSeriesSlug(parsed)
	if err != nil {
		return domain.Manga{}, err
	}

	var series asurascansSeriesResponse
	if err := a.getJSON(ctx, a.seriesAPIURL(slug), &series); err != nil {
		return domain.Manga{}, fmt.Errorf("getting series %s: %w", slug, err)
	}

	title := sanitize.Filename(strings.TrimSpace(series.Series.Title))
	if len(title) == 0 {
		return domain.Manga{}, fmt.Errorf("getting title for series %s", slug)
	}

	var chapterList asurascansChapterListResponse
	if err := a.getJSON(ctx, a.seriesAPIURL(slug, "chapters"), &chapterList); err != nil {
		return domain.Manga{}, fmt.Errorf("getting chapters for series %s: %w", slug, err)
	}

	manga := domain.Manga{
		URL:      stripURLQueryAndFragment(parsed),
		Title:    title,
		Chapters: make(map[domain.ChapterNumber]domain.Chapter),
		IsManhwa: true,
	}

	for _, item := range chapterList.Data {
		// Premium chapters are early access: their pages are not public yet.
		if item.IsPremium || len(item.Slug) == 0 {
			continue
		}

		chapterNum, err := domain.ParseChapterNumber(strconv.FormatFloat(item.Number, 'f', -1, 64))
		if err != nil {
			continue
		}

		// The chapter list gives the series slug without the hash suffix, which Asura changes from time to time.
		manga.Chapters[chapterNum] = domain.Chapter{
			URL:    a.seriesAPIURL(cmp.Or(item.SeriesSlug, slug), "chapters", item.Slug),
			Number: chapterNum,
			Title:  sanitize.Filename(strings.TrimSpace(item.Title)),
		}
	}

	if len(manga.Chapters) == 0 {
		return domain.Manga{}, fmt.Errorf("getting chapters for manga %s", manga.Title)
	}

	return manga, nil
}

func (a *asurascans) Pages(ctx context.Context, chapter domain.Chapter) ([]domain.ImageInfo, error) {
	var response asurascansChapterResponse
	if err := a.getJSON(ctx, chapter.URL, &response); err != nil {
		return nil, fmt.Errorf("getting chapter %s: %w", chapter.Number, err)
	}

	imageInfos := make([]domain.ImageInfo, 0, len(response.Data.Chapter.Pages))
	for _, page := range response.Data.Chapter.Pages {
		if len(page.URL) != 0 {
			imageInfos = append(imageInfos, browserImage(page.URL))
		}
	}

	if len(imageInfos) == 0 {
		return nil, fmt.Errorf("getting image URLs for chapter %s", chapter.Number)
	}

	return imageInfos, nil
}

// seriesAPIURL returns the API URL for the series slug, followed by the escaped path segments in rest.
func (a *asurascans) seriesAPIURL(slug string, rest ...string) string {
	path := a.APIURL + "/api/series/" + url.PathEscape(slug)
	for _, segment := range rest {
		path += "/" + url.PathEscape(segment)
	}

	return path
}

func (a *asurascans) getJSON(ctx context.Context, rawURL string, target any) error {
	body, err := fetchPage(ctx, a.Client, rawURL, "application/json")
	if err != nil {
		return fmt.Errorf("fetching %s: %w", rawURL, err)
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decoding response from %s: %w", rawURL, err)
	}

	return nil
}

// asurascansSeriesSlug returns <slug> from a /comics/<slug> URL.
// The API accepts a slug with an outdated hash suffix (the series endpoint redirects it), so old configured URLs keep working.
func asurascansSeriesSlug(parsed *url.URL) (string, error) {
	rest, ok := strings.CutPrefix(parsed.Path, "/comics/")
	slug, _, _ := strings.Cut(rest, "/")
	if !ok || len(slug) == 0 {
		return "", fmt.Errorf("the URL for Asura Scans must point to /comics/<series>")
	}

	return slug, nil
}

func stripURLQueryAndFragment(parsed *url.URL) string {
	clean := parsed.Clone()
	clean.RawQuery = ""
	clean.Fragment = ""
	return clean.String()
}
