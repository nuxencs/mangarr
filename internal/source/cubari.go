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

const cubariURL = "https://cubari.moe"

type cubari struct {
	MangaURL string
	GroupID  string
	BaseURL  string
	Client   *http.Client
}

type cubariResponse struct {
	Cover       string                     `json:"cover"`
	Description string                     `json:"description"`
	Title       string                     `json:"title"`
	Chapters    map[string]json.RawMessage `json:"chapters"`
}

type cubariChapter struct {
	Title  string                     `json:"title"`
	Groups map[string]json.RawMessage `json:"groups"`
}

func NewCubari(mangaURL, groupID string) domain.Source {
	client := http.Client{
		Timeout:   60 * time.Second,
		Transport: sharedhttp.Transport,
	}

	return &cubari{
		MangaURL: mangaURL,
		GroupID:  groupID,
		BaseURL:  cubariURL,
		Client:   &client,
	}
}

func (c *cubari) String() string {
	return "Cubari"
}

func (c *cubari) ValidateInput() error {
	if _, err := url.Parse(c.MangaURL); err != nil {
		return fmt.Errorf("parsing URL %s: %w", c.MangaURL, err)
	}

	if len(c.GroupID) == 0 {
		return fmt.Errorf("cubari group is required")
	}

	return nil
}

func (c *cubari) Discover(ctx context.Context) (domain.Manga, error) {
	var cubariResp cubariResponse

	if err := c.fetchJSON(ctx, c.MangaURL, &cubariResp); err != nil {
		return domain.Manga{}, err
	}

	title := cubariResp.Title
	if len(title) == 0 {
		return domain.Manga{}, fmt.Errorf("getting manga for URL %s", c.MangaURL)
	}

	manga := domain.Manga{
		Title:    sanitize.Filename(title),
		Chapters: make(map[domain.ChapterNumber]domain.Chapter),
	}

	for num, chapter := range cubariResp.Chapters {
		chapterNum, err := domain.ParseChapterNumber(num)
		if err != nil {
			return domain.Manga{}, fmt.Errorf("parsing chapter number from %s: %w", num, err)
		}
		var chapterData cubariChapter
		if err := json.Unmarshal(chapter, &chapterData); err != nil {
			return domain.Manga{}, fmt.Errorf("decoding chapter %s: %w", num, err)
		}

		rawURLs, ok := chapterData.Groups[c.GroupID]
		if !ok {
			continue
		}

		imageURLs, err := c.chapterImageURLs(ctx, rawURLs)
		if err != nil {
			return domain.Manga{}, fmt.Errorf("decoding chapter %s image URLs for group %s: %w", num, c.GroupID, err)
		}
		if len(imageURLs) == 0 {
			continue
		}

		imageInfos := make([]domain.ImageInfo, 0, len(imageURLs))
		for _, imageURL := range imageURLs {
			imageInfos = append(imageInfos, domain.ImageInfo{ImageURL: imageURL})
		}

		manga.Chapters[chapterNum] = domain.Chapter{
			Number:    chapterNum,
			Title:     sanitize.Filename(c.getChapterName(chapterData.Title)),
			ImageInfo: imageInfos,
		}
	}

	if len(manga.Chapters) == 0 {
		return domain.Manga{}, fmt.Errorf("getting chapters for manga: %s", manga.Title)
	}

	return manga, nil
}

// chapterImageURLs resolves a group entry, which Cubari gists write either as a
// list of image URLs or as a proxy path that has to be fetched to get that list.
func (c *cubari) chapterImageURLs(ctx context.Context, raw json.RawMessage) ([]string, error) {
	var proxyPath string
	if err := json.Unmarshal(raw, &proxyPath); err != nil {
		var imageURLs []string
		if err := json.Unmarshal(raw, &imageURLs); err != nil {
			return nil, err
		}

		return imageURLs, nil
	}

	proxyURL, err := c.proxyURL(proxyPath)
	if err != nil {
		return nil, err
	}

	var imageURLs []string
	if err := c.fetchJSON(ctx, proxyURL, &imageURLs); err != nil {
		return nil, err
	}

	return imageURLs, nil
}

func (c *cubari) proxyURL(path string) (string, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path, nil
	}

	proxyURL, err := url.JoinPath(c.BaseURL, path)
	if err != nil {
		return "", fmt.Errorf("building proxy URL for %s: %w", path, err)
	}

	return proxyURL, nil
}

func (c *cubari) fetchJSON(ctx context.Context, rawURL string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", "mangarr")

	retryErr := retry.Do(func() error {
		resp, err := sharedhttp.ExecRequest(*c.Client, req)
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

func (c *cubari) Pages(_ context.Context, chapter domain.Chapter) ([]domain.ImageInfo, error) {
	if len(chapter.ImageInfo) == 0 {
		return nil, fmt.Errorf("getting image URLs for chapter %s", chapter.Number)
	}

	return chapter.ImageInfo, nil
}

func (c *cubari) getChapterName(chapterString string) string {
	_, after, _ := strings.Cut(chapterString, ":")

	return strings.TrimSpace(after)
}
