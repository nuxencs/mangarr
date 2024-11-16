package source

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/sanitize"
	"mangarr/internal/sharedhttp"

	"github.com/avast/retry-go"
	"github.com/google/uuid"
)

const (
	mangadexURL   = "https://api.mangadex.org"
	mangadexLimit = 500
)

type mangadex struct {
	MangaID  string
	GroupID  string
	Language string
	Client   *http.Client
}

type mangadexManga struct {
	Data struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Title struct {
				En string `json:"en"`
			} `json:"title"`
		} `json:"attributes"`
	} `json:"data"`
}

type mangadexChapters struct {
	Data []struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Volume  *string `json:"volume"`
			Chapter string  `json:"chapter"`
			Title   *string `json:"title"`
		} `json:"attributes"`
		Relationships []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"relationships"`
	} `json:"data"`
	Total int `json:"total"`
}

type mangadexChapter struct {
	BaseURL string `json:"baseUrl"`
	Chapter struct {
		Hash      string   `json:"hash"`
		Data      []string `json:"data"`
		DataSaver []string `json:"dataSaver"`
	} `json:"chapter"`
}

func NewMangadex(manga, group, language string) domain.Source {
	client := http.Client{
		Timeout:   60 * time.Second,
		Transport: sharedhttp.Transport,
	}

	return &mangadex{
		MangaID:  manga,
		GroupID:  group,
		Language: language,
		Client:   &client,
	}
}

func (m *mangadex) String() string {
	return "MangaDex"
}

func (m *mangadex) ValidateInput() error {
	if _, err := uuid.Parse(m.MangaID); err != nil {
		return fmt.Errorf("failed to parse Manga PLUS manga id: %w", err)
	}

	if _, err := uuid.Parse(m.GroupID); err != nil {
		return fmt.Errorf("failed to parse Manga PLUS group id: %w", err)
	}

	if len(m.Language) == 0 {
		m.Language = "en"
	}

	return nil
}

func (m *mangadex) GetManga(ctx context.Context) (domain.Manga, error) {
	var mangaResp mangadexManga

	path, err := url.JoinPath(mangadexURL, "manga", m.MangaID)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("failed to build URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "mangarr")

	retryErr := retry.Do(func() error {
		resp, err := sharedhttp.ExecRequest(*m.Client, req)
		if err != nil {
			return fmt.Errorf("failed to execute request %s: %w", req.URL, err)
		}

		buf := bufio.NewReader(resp.Body)

		err = json.NewDecoder(buf).Decode(&mangaResp)
		if err != nil {
			return retry.Unrecoverable(fmt.Errorf("failed to decode response: %w", err))
		}

		return nil
	},
		retry.Delay(time.Second*3),
		retry.Attempts(3),
		retry.MaxJitter(time.Second*1),
	)
	if retryErr != nil {
		return domain.Manga{}, fmt.Errorf("failed to execute request %s: %w", req.URL, retryErr)
	}

	title := mangaResp.Data.Attributes.Title.En
	if len(title) == 0 {
		return domain.Manga{}, fmt.Errorf("failed to get manga for ID %s", m.MangaID)
	}

	return domain.Manga{
		Title:    sanitize.Filename(title),
		Chapters: make(map[float32]domain.Chapter),
	}, nil
}

func (m *mangadex) GetChapters(ctx context.Context, manga domain.Manga) error {
	var retryErr error
	var chapterResp mangadexChapters
	var chapterCount int
	var offset int

	path, err := url.JoinPath(mangadexURL, "manga", m.MangaID, "feed")
	if err != nil {
		return fmt.Errorf("failed to build URL: %w", err)
	}

	u, err := url.Parse(path)
	if err != nil {
		return fmt.Errorf("failed to parse URL %s: %w", path, err)
	}

	for {
		params := url.Values{
			"translatedLanguage[]": []string{m.Language},
			"order[volume]":        []string{"desc"},
			"order[chapter]":       []string{"desc"},
			"limit":                []string{fmt.Sprintf("%d", mangadexLimit)},
			"offset":               []string{fmt.Sprintf("%d", offset)},
		}

		u.RawQuery = params.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("User-Agent", "mangarr")

		retryErr = retry.Do(func() error {
			resp, err := sharedhttp.ExecRequest(*m.Client, req)
			if err != nil {
				return fmt.Errorf("failed to execute request %s: %w", req.URL, err)
			}

			buf := bufio.NewReader(resp.Body)

			err = json.NewDecoder(buf).Decode(&chapterResp)
			if err != nil {
				return retry.Unrecoverable(fmt.Errorf("failed to decode response: %w", err))
			}

			return nil
		},
			retry.Delay(time.Second*3),
			retry.Attempts(3),
			retry.MaxJitter(time.Second*1),
		)
		if retryErr != nil {
			return fmt.Errorf("failed to execute request %s: %w", req.URL, retryErr)
		}

		for _, data := range chapterResp.Data {
			for _, rel := range data.Relationships {
				if rel.Type == "scanlation_group" && rel.ID == m.GroupID {
					chapterNum64, err := strconv.ParseFloat(data.Attributes.Chapter, 32)
					if err != nil {
						return fmt.Errorf("failed to parse chapter number from %s: %w", data.Attributes.Chapter, err)
					}
					chapterNum := float32(chapterNum64)

					var title string
					if data.Attributes.Title != nil {
						title = *data.Attributes.Title
					}

					manga.Chapters[chapterNum] = domain.Chapter{
						ID:     data.ID,
						Number: chapterNum,
						Title:  sanitize.Filename(title),
					}
				}
			}
		}

		if len(manga.Chapters) == 0 {
			return fmt.Errorf("failed to get chapters for for manga ID %s", m.MangaID)
		}

		chapterCount += len(chapterResp.Data)

		if chapterCount == chapterResp.Total {
			return nil
		}

		offset += mangadexLimit
	}
}

func (m *mangadex) GetImageURLs(ctx context.Context, chapter *domain.Chapter) error {
	var chapterResp mangadexChapter
	var imageInfos []domain.ImageInfo

	path, err := url.JoinPath(mangadexURL, "at-home/server", chapter.ID)
	if err != nil {
		return fmt.Errorf("failed to build URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "mangarr")

	retryErr := retry.Do(func() error {
		resp, err := sharedhttp.ExecRequest(*m.Client, req)
		if err != nil {
			return fmt.Errorf("failed to execute request %s: %w", req.URL, err)
		}

		buf := bufio.NewReader(resp.Body)

		err = json.NewDecoder(buf).Decode(&chapterResp)
		if err != nil {
			return retry.Unrecoverable(fmt.Errorf("failed to decode response: %w", err))
		}

		return nil
	},
		retry.Delay(time.Second*3),
		retry.Attempts(3),
		retry.MaxJitter(time.Second*1),
	)
	if retryErr != nil {
		return fmt.Errorf("failed to execute request %s: %w", req.URL, retryErr)
	}

	for _, imageURL := range chapterResp.Chapter.Data {
		imagePath, err := url.JoinPath(chapterResp.BaseURL, "data", chapterResp.Chapter.Hash, imageURL)
		if err != nil {
			return fmt.Errorf("failed to build URL: %w", err)
		}

		imageInfos = append(imageInfos, domain.ImageInfo{ImageURL: imagePath})
	}

	if len(imageInfos) == 0 {
		return fmt.Errorf("failed to get image URLs for chapter ID %s", chapter.ID)
	}

	chapter.ImageInfo = imageInfos
	return nil
}
