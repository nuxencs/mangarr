package source

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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
	Cover       string `json:"cover"`
	Description string `json:"description"`
	Title       string `json:"title"`
	Chapters    map[string]struct {
		Groups      map[string][]string `json:"groups"`
		LastUpdated int64               `json:"last_updated"`
		Title       string              `json:"title"`
		Volume      string              `json:"volume"`
	} `json:"chapters"`
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
		return err
	}

	if len(c.GroupID) == 0 {
		return fmt.Errorf("cubari group id is required")
	}

	return nil
}

func (c *cubari) GetManga(ctx context.Context) (domain.Manga, error) {
	var cubariResp cubariResponse

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.MangaURL, nil)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "mangarr")

	retryErr := retry.Do(func() error {
		resp, err := sharedhttp.ExecRequest(*c.Client, req)
		if err != nil {
			return err
		}

		buf := bufio.NewReader(resp.Body)

		err = json.NewDecoder(buf).Decode(&cubariResp)
		if err != nil {
			return retry.Unrecoverable(err)
		}

		return nil
	},
		retry.Delay(time.Second*3),
		retry.Attempts(3),
		retry.MaxJitter(time.Second*1),
	)

	title := cubariResp.Title
	if len(title) == 0 {
		return domain.Manga{}, fmt.Errorf("failed to get manga for provided url: %s", c.MangaURL)
	}

	manga := domain.Manga{
		Title:    sanitize.Filename(title),
		Chapters: make(map[float32]domain.Chapter),
	}

	for num, chapter := range cubariResp.Chapters {
		chapterNum64, err := strconv.ParseFloat(num, 32)
		if err != nil {
			return domain.Manga{}, err
		}

		chapterNum := float32(chapterNum64)
		chapterTitle := c.getChapterName(chapter.Title)

		if imageURLs, ok := chapter.Groups[c.GroupID]; ok {
			var imageInfos []domain.ImageInfo

			for _, imageURL := range imageURLs {
				imageInfos = append(imageInfos, domain.ImageInfo{ImageURL: imageURL})
			}

			manga.Chapters[chapterNum] = domain.Chapter{
				Number:    chapterNum,
				Title:     sanitize.Filename(chapterTitle),
				ImageInfo: imageInfos,
			}
		}
	}

	if len(manga.Chapters) == 0 {
		return domain.Manga{}, fmt.Errorf("failed to get chapters for manga: %s", manga.Title)
	}

	return manga, retryErr
}

func (c *cubari) GetChapters(_ context.Context, _ domain.Manga) error {
	return nil
}

func (c *cubari) GetImageURLs(_ context.Context, _ *domain.Chapter) error {
	return nil
}

func (c *cubari) getChapterName(chapterString string) string {
	colonIndex := strings.Index(chapterString, ":")

	return strings.TrimSpace(chapterString[colonIndex+1:])
}
