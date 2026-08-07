package source

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/sanitize"
	"mangarr/internal/sharedhttp"

	"github.com/avast/retry-go"
	"github.com/google/uuid"
)

const (
	mangadexURL         = "https://api.mangadex.org"
	mangadexResultLimit = 500

	// tag ID for the "Long Strip" format to identify Manhwa
	// https://mangadex.org/titles?include=3e2b8dae-350e-4ab8-a8ce-016e844b9f0d
	mangadexLongStripTagID = "3e2b8dae-350e-4ab8-a8ce-016e844b9f0d"
)

type mangadex struct {
	MangaID  string
	GroupID  string
	Language string
	Client   *http.Client
	BaseURL  string
}

type mangadexManga struct {
	Data struct {
		ID         string `json:"id"`
		Attributes struct {
			Title map[string]string `json:"title"`
			Tags  []struct {
				ID string `json:"id"`
			} `json:"tags"`
		} `json:"attributes"`
	} `json:"data"`
}

type mangadexChapters struct {
	Data  []mangadexChaptersData `json:"data"`
	Total int                    `json:"total"`
}

type mangadexChaptersData struct {
	ID         string `json:"id"`
	Attributes struct {
		Volume  *string `json:"volume"`
		Chapter string  `json:"chapter"`
		Title   *string `json:"title"`
	} `json:"attributes"`
	Relationships []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	} `json:"relationships"`
}

type mangadexChapter struct {
	BaseURL string `json:"baseUrl"`
	Chapter struct {
		Hash string   `json:"hash"`
		Data []string `json:"data"`
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
		BaseURL:  mangadexURL,
	}
}

func (m *mangadex) String() string {
	return "MangaDex"
}

func (m *mangadex) ValidateInput() error {
	if _, err := uuid.Parse(m.MangaID); err != nil {
		return fmt.Errorf("parsing MangaDex manga id: %w", err)
	}

	if m.GroupID != "" {
		if _, err := uuid.Parse(m.GroupID); err != nil {
			return fmt.Errorf("parsing MangaDex group id: %w", err)
		}
	}

	if len(m.Language) == 0 {
		m.Language = "en"
	}

	return nil
}

func (m *mangadex) GetManga(ctx context.Context) (domain.Manga, error) {
	var mangaResp mangadexManga
	var isManhwa bool

	path, err := url.JoinPath(m.BaseURL, "manga", m.MangaID)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("building URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", "mangarr")

	retryErr := retry.Do(func() error {
		resp, err := sharedhttp.ExecRequest(*m.Client, req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		buf := bufio.NewReader(resp.Body)

		err = json.NewDecoder(buf).Decode(&mangaResp)
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

	title := m.getMangaTitle(mangaResp.Data.Attributes.Title)
	if len(title) == 0 {
		return domain.Manga{}, fmt.Errorf("getting manga for ID %s", m.MangaID)
	}

	for _, tag := range mangaResp.Data.Attributes.Tags {
		if tag.ID == mangadexLongStripTagID {
			isManhwa = true
			break
		}
	}

	return domain.Manga{
		Title:    sanitize.Filename(title),
		Chapters: make(map[domain.ChapterNumber]domain.Chapter),
		IsManhwa: isManhwa,
	}, nil
}

func (m *mangadex) GetChapters(ctx context.Context, manga domain.Manga) error {
	var retryErr error
	var chapterResp mangadexChapters
	var chapterCount int
	var offset int

	path, err := url.JoinPath(m.BaseURL, "manga", m.MangaID, "feed")
	if err != nil {
		return fmt.Errorf("building URL: %w", err)
	}

	u, err := url.Parse(path)
	if err != nil {
		return fmt.Errorf("parsing URL %s: %w", path, err)
	}

	processedChapters := make(map[string]bool)

	for {
		params := url.Values{
			"translatedLanguage[]": []string{m.Language},
			"order[volume]":        []string{"desc"},
			"order[chapter]":       []string{"desc"},
			"limit":                []string{fmt.Sprintf("%d", mangadexResultLimit)},
			"offset":               []string{fmt.Sprintf("%d", offset)},
		}

		u.RawQuery = params.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("User-Agent", "mangarr")

		retryErr = retry.Do(func() error {
			resp, err := sharedhttp.ExecRequest(*m.Client, req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			buf := bufio.NewReader(resp.Body)

			err = json.NewDecoder(buf).Decode(&chapterResp)
			if err != nil {
				return retry.Unrecoverable(fmt.Errorf("decoding response: %w", err))
			}

			return nil
		},
			sharedhttp.RetryOptions(ctx)...,
		)
		if retryErr != nil {
			return fmt.Errorf("executing request %s: %w", req.URL, retryErr)
		}

		for _, data := range chapterResp.Data {
			if m.shouldProcessChapter(data, m.GroupID) {
				if processedChapters[data.Attributes.Chapter] {
					continue
				}

				if err := m.processChapter(data, &manga); err != nil {
					return fmt.Errorf("processing chapter: %w", err)
				}
				processedChapters[data.Attributes.Chapter] = true
			}
		}

		chapterCount += len(chapterResp.Data)

		if chapterCount >= chapterResp.Total {
			break
		}
		if len(chapterResp.Data) == 0 {
			return fmt.Errorf("getting chapters for manga ID %s: pagination stopped at offset %d of %d", m.MangaID, offset, chapterResp.Total)
		}

		offset += mangadexResultLimit
	}

	if len(manga.Chapters) == 0 {
		return fmt.Errorf("getting chapters for manga ID %s", m.MangaID)
	}

	return nil
}

func (m *mangadex) GetImageURLs(ctx context.Context, chapter *domain.Chapter) error {
	var chapterResp mangadexChapter
	var imageInfos []domain.ImageInfo

	path, err := url.JoinPath(m.BaseURL, "at-home/server", chapter.ID)
	if err != nil {
		return fmt.Errorf("building URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", "mangarr")

	retryErr := retry.Do(func() error {
		resp, err := sharedhttp.ExecRequest(*m.Client, req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		buf := bufio.NewReader(resp.Body)

		err = json.NewDecoder(buf).Decode(&chapterResp)
		if err != nil {
			return retry.Unrecoverable(fmt.Errorf("decoding response: %w", err))
		}

		return nil
	},
		sharedhttp.RetryOptions(ctx)...,
	)
	if retryErr != nil {
		return fmt.Errorf("executing request %s: %w", req.URL, retryErr)
	}

	for _, imageURL := range chapterResp.Chapter.Data {
		imagePath, err := url.JoinPath(chapterResp.BaseURL, "data", chapterResp.Chapter.Hash, imageURL)
		if err != nil {
			return fmt.Errorf("building URL: %w", err)
		}

		imageInfos = append(imageInfos, domain.ImageInfo{ImageURL: imagePath})
	}

	if len(imageInfos) == 0 {
		return fmt.Errorf("getting image URLs for chapter ID %s", chapter.ID)
	}

	chapter.ImageInfo = imageInfos
	return nil
}

func (m *mangadex) getMangaTitle(titles map[string]string) string {
	if title := titles[m.Language]; title != "" {
		return title
	}
	if title := titles["en"]; title != "" {
		return title
	}

	languages := make([]string, 0, len(titles))
	for language := range titles {
		languages = append(languages, language)
	}
	sort.Strings(languages)
	for _, language := range languages {
		if titles[language] != "" {
			return titles[language]
		}
	}

	return ""
}

func (m *mangadex) shouldProcessChapter(data mangadexChaptersData, groupID string) bool {
	if len(groupID) == 0 {
		return true
	}

	for _, rel := range data.Relationships {
		if rel.Type == "scanlation_group" && rel.ID == groupID {
			return true
		}
	}

	return false
}

func (m *mangadex) processChapter(data mangadexChaptersData, manga *domain.Manga) error {
	chapter := data.Attributes.Chapter

	if chapter == "" {
		chapter = "0"
	}

	chapterNum, err := domain.ParseChapterNumber(chapter)
	if err != nil {
		return fmt.Errorf("parsing chapter number from %s: %w", chapter, err)
	}

	var title string
	if data.Attributes.Title != nil {
		title = *data.Attributes.Title
	}

	manga.Chapters[chapterNum] = domain.Chapter{
		ID:     data.ID,
		Number: chapterNum,
		Title:  sanitize.Filename(title),
	}

	return nil
}
