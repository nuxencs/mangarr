package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/sanitize"
	"mangarr/internal/sharedhttp"

	"github.com/avast/retry-go"
)

const atsumaruURL = "https://atsu.moe"

type atsumaru struct {
	MangaURL string
	ScanID   string
	Client   *http.Client
	BaseURL  string
}

type atsumaruMangaResponse struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	ForceStrip bool              `json:"forceStrip"`
	Chapters   []atsumaruChapter `json:"chapters"`
}

type atsumaruChapter struct {
	ID     string               `json:"id"`
	Title  string               `json:"title"`
	Number domain.ChapterNumber `json:"number"`
	ScanID string               `json:"scanId"`
}

type atsumaruReadChapterResponse struct {
	ReadChapter struct {
		ID    string         `json:"id"`
		Title string         `json:"title"`
		Pages []atsumaruPage `json:"pages"`
	} `json:"readChapter"`
}

type atsumaruPage struct {
	Image string `json:"image"`
}

func NewAtsumaru(mangaURL, scanID string) domain.Source {
	client := http.Client{
		Timeout:   60 * time.Second,
		Transport: sharedhttp.Transport,
	}

	return &atsumaru{
		MangaURL: mangaURL,
		ScanID:   scanID,
		Client:   &client,
		BaseURL:  atsumaruURL,
	}
}

func (a *atsumaru) String() string {
	return "Atsumaru"
}

func (a *atsumaru) ValidateInput() error {
	if len(a.MangaURL) == 0 {
		return fmt.Errorf("atsumaru manga URL is required")
	}

	if len(a.ScanID) == 0 {
		return fmt.Errorf("atsumaru scan ID is required")
	}

	parsed, err := url.Parse(a.MangaURL)
	if err != nil {
		return fmt.Errorf("parsing URL %s: %w", a.MangaURL, err)
	}

	if parsed.Scheme != "https" || strings.ToLower(parsed.Host) != "atsu.moe" {
		return fmt.Errorf("the URL for Atsumaru must start with %s", atsumaruURL)
	}

	if _, err := a.extractMangaID(); err != nil {
		return err
	}

	return nil
}

func (a *atsumaru) Discover(ctx context.Context) (domain.Manga, error) {
	mangaID, err := a.extractMangaID()
	if err != nil {
		return domain.Manga{}, err
	}

	apiURL, err := a.apiURL("api/manga/info", url.Values{"mangaId": []string{mangaID}})
	if err != nil {
		return domain.Manga{}, err
	}

	var mangaResp atsumaruMangaResponse
	if err := a.getJSON(ctx, apiURL, &mangaResp); err != nil {
		return domain.Manga{}, fmt.Errorf("getting manga info from %s: %w", apiURL, err)
	}

	if len(mangaResp.Title) == 0 {
		return domain.Manga{}, fmt.Errorf("getting manga for ID %s", mangaID)
	}

	manga := domain.Manga{
		ID:       mangaResp.ID,
		URL:      a.MangaURL,
		Title:    sanitize.Filename(mangaResp.Title),
		Chapters: make(map[domain.ChapterNumber]domain.Chapter),
		IsManhwa: mangaResp.ForceStrip,
	}
	if manga.ID == "" {
		manga.ID = mangaID
	}

	for _, chapter := range mangaResp.Chapters {
		if chapter.ScanID != a.ScanID {
			continue
		}

		manga.Chapters[chapter.Number] = domain.Chapter{
			ID:     chapter.ID,
			Number: chapter.Number,
			Title:  sanitize.Filename(chapter.Title),
		}
	}

	if len(manga.Chapters) == 0 {
		return domain.Manga{}, fmt.Errorf("getting chapters for manga %s", manga.Title)
	}

	return manga, nil
}

func (a *atsumaru) Pages(ctx context.Context, chapter domain.Chapter) ([]domain.ImageInfo, error) {
	mangaID, err := a.extractMangaID()
	if err != nil {
		return nil, err
	}

	apiURL, err := a.apiURL("api/read/chapter", url.Values{
		"mangaId":   []string{mangaID},
		"chapterId": []string{chapter.ID},
	})
	if err != nil {
		return nil, err
	}

	var chapterResp atsumaruReadChapterResponse
	if err := a.getJSON(ctx, apiURL, &chapterResp); err != nil {
		return nil, fmt.Errorf("getting chapter pages from %s: %w", apiURL, err)
	}

	imageInfos := make([]domain.ImageInfo, 0, len(chapterResp.ReadChapter.Pages))
	for _, page := range chapterResp.ReadChapter.Pages {
		imageURL, err := a.resolveImageURL(page.Image)
		if err != nil {
			return nil, fmt.Errorf("resolving image URL %s: %w", page.Image, err)
		}

		imageInfos = append(imageInfos, domain.ImageInfo{ImageURL: imageURL})
	}

	if len(imageInfos) == 0 {
		return nil, fmt.Errorf("getting image URLs for chapter ID %s", chapter.ID)
	}

	return imageInfos, nil
}

func (a *atsumaru) extractMangaID() (string, error) {
	parsed, err := url.Parse(a.MangaURL)
	if err != nil {
		return "", fmt.Errorf("parsing URL %s: %w", a.MangaURL, err)
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "manga" || parts[1] == "" {
		return "", fmt.Errorf("invalid Atsumaru manga URL path")
	}

	return parts[1], nil
}

func (a *atsumaru) apiURL(path string, params url.Values) (string, error) {
	u, err := url.JoinPath(a.BaseURL, path)
	if err != nil {
		return "", fmt.Errorf("building URL: %w", err)
	}

	parsed, err := url.Parse(u)
	if err != nil {
		return "", fmt.Errorf("parsing URL %s: %w", u, err)
	}

	parsed.RawQuery = params.Encode()
	return parsed.String(), nil
}

func (a *atsumaru) getJSON(ctx context.Context, rawURL string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", "mangarr")

	retryErr := retry.Do(func() error {
		resp, err := sharedhttp.ExecRequest(*a.Client, req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			return retry.Unrecoverable(fmt.Errorf("decoding response: %w", err))
		}

		return nil
	},
		sharedhttp.RetryOptions(ctx)...,
	)
	if retryErr != nil {
		return fmt.Errorf("executing request %s: %w", req.URL, retryErr)
	}

	return nil
}

func (a *atsumaru) resolveImageURL(rawImageURL string) (string, error) {
	base, err := url.Parse(a.BaseURL)
	if err != nil {
		return "", err
	}

	imageURL, err := url.Parse(rawImageURL)
	if err != nil {
		return "", err
	}

	return base.ResolveReference(imageURL).String(), nil
}
