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

type cubari struct {
	MangaURL string
	GroupID  string
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

func (c *cubari) GetManga(ctx context.Context) (domain.Manga, error) {
	var cubariResp cubariResponse

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.MangaURL, nil)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", "mangarr")

	retryErr := retry.Do(func() error {
		resp, err := sharedhttp.ExecRequest(*c.Client, req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		err = json.NewDecoder(resp.Body).Decode(&cubariResp)
		if err != nil {
			return retry.Unrecoverable(fmt.Errorf("decoding response: %w", err))
		}

		return nil
	},
		sharedhttp.RetryOptions(ctx)...,
	)
	if retryErr != nil {
		return domain.Manga{}, fmt.Errorf("executing request %s: %w", req.URL, retryErr)
	}

	title := cubariResp.Title
	if len(title) == 0 {
		return domain.Manga{}, fmt.Errorf("getting manga for URL %s", req.URL)
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

		var imageURLs []string
		if err := json.Unmarshal(rawURLs, &imageURLs); err != nil {
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

func (c *cubari) GetChapters(_ context.Context, _ domain.Manga) error {
	return nil
}

func (c *cubari) GetImageURLs(_ context.Context, _ *domain.Chapter) error {
	return nil
}

func (c *cubari) getChapterName(chapterString string) string {
	_, after, _ := strings.Cut(chapterString, ":")

	return strings.TrimSpace(after)
}
