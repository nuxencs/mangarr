package source

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"

	"mangarr/internal/domain"
	"mangarr/internal/sanitize"
	"mangarr/internal/sharedhttp"

	"github.com/avast/retry-go"
)

const (
	comixURL         = "https://comix.to"
	comixResultLimit = 100

	comixOfficialGroupID = "9275"
)

type comix struct {
	MangaURL  string
	GroupID   string
	Client    *http.Client
	Collector colly.Collector
}

type comixChaptersResponse struct {
	Status int `json:"status"`
	Result struct {
		Items      []comixChapter `json:"items"`
		Pagination struct {
			Count       int `json:"count"`
			Total       int `json:"total"`
			PerPage     int `json:"per_page"`
			CurrentPage int `json:"current_page"`
			LastPage    int `json:"last_page"`
			From        int `json:"from"`
			To          int `json:"to"`
		} `json:"pagination"`
	} `json:"result"`
}

type comixChapterResponse struct {
	Status int `json:"status"`
	Result struct {
		ChapterID         int     `json:"chapter_id"`
		MangaID           int     `json:"manga_id"`
		ScanlationGroupID int     `json:"scanlation_group_id"`
		Number            float32 `json:"number"`
		Name              string  `json:"name"`
		Language          string  `json:"language"`
		Volume            int     `json:"volume"`
		Votes             int     `json:"votes"`
		CreatedAt         int     `json:"created_at"`
		UpdatedAt         int     `json:"updated_at"`
		ScanlationGroup   struct {
			ScanlationGroupID int    `json:"scanlation_group_id"`
			Name              string `json:"name"`
			Slug              string `json:"slug"`
		} `json:"scanlation_group"`
		Images []string     `json:"images"`
		Prev   comixChapter `json:"prev"`
		Next   comixChapter `json:"next"`
	} `json:"result"`
}

type comixChapter struct {
	ChapterID         int     `json:"chapter_id"`
	MangaID           int     `json:"manga_id"`
	ScanlationGroupID int     `json:"scanlation_group_id"`
	Number            float32 `json:"number"`
	Name              string  `json:"name"`
	Language          string  `json:"language"`
	Volume            int     `json:"volume"`
	Votes             int     `json:"votes"`
	CreatedAt         int     `json:"created_at"`
	UpdatedAt         int     `json:"updated_at"`
	ScanlationGroup   struct {
		ScanlationGroupID int    `json:"scanlation_group_id"`
		Name              string `json:"name"`
		Slug              string `json:"slug"`
	} `json:"scanlation_group"`
	Images []string `json:"images"`
}

func NewComix(manga, group string) domain.Source {
	client := http.Client{
		Timeout:   60 * time.Second,
		Transport: sharedhttp.Transport,
	}

	collector := colly.NewCollector(
		colly.AllowURLRevisit(),
	)
	extensions.RandomUserAgent(collector)

	collector.SetRequestTimeout(120 * time.Second)

	return &comix{
		MangaURL:  manga,
		GroupID:   group,
		Client:    &client,
		Collector: *collector,
	}
}

func (c *comix) String() string {
	return "Comix"
}

func (c *comix) ValidateInput() error {
	if !strings.HasPrefix(c.MangaURL, "https://comix.to/title") {
		return fmt.Errorf("the URL for Comix must start with https://comix.to/title")
	}

	if _, err := url.Parse(c.MangaURL); err != nil {
		return fmt.Errorf("parsing URL %s: %w", c.MangaURL, err)
	}

	return nil
}

func (c *comix) GetManga(_ context.Context) (domain.Manga, error) {
	var manga domain.Manga
	var errors []error
	col := c.Collector.Clone()

	col.OnError(func(r *colly.Response, err error) {
		errors = append(errors, fmt.Errorf("requesting URL %s: %w", r.Request.URL, err))
	})

	col.OnHTML(".comic-info .title", func(e *colly.HTMLElement) {
		manga = domain.Manga{
			Title:    sanitize.Filename(e.Text),
			Chapters: make(map[float32]domain.Chapter),
		}
	})

	err := col.Visit(c.MangaURL)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("visiting URL %s: %w", c.MangaURL, err)
	}

	if len(errors) > 0 {
		return domain.Manga{}, fmt.Errorf("processing %d URLs: %w", len(errors), errors[0])
	}

	mangaID, err := c.extractIDFromURL(c.MangaURL)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("extracting ID from URL %s: %w", c.MangaURL, err)
	}

	manga.ID = mangaID

	return manga, nil
}

func (c *comix) GetChapters(ctx context.Context, manga domain.Manga) error {
	var (
		retryErr    error
		chapterResp comixChaptersResponse

		page = 1
	)

	path, err := url.JoinPath(comixURL, "api/v2/manga", manga.ID, "chapters")
	if err != nil {
		return fmt.Errorf("building URL: %w", err)
	}

	u, err := url.Parse(path)
	if err != nil {
		return fmt.Errorf("parsing URL %s: %w", path, err)
	}

	processedChapters := make(map[float32]bool)

	if len(c.GroupID) == 0 {
		c.GroupID = comixOfficialGroupID
	}

	for {
		params := url.Values{
			"order[number]":       []string{"desc"},
			"limit":               []string{fmt.Sprintf("%d", comixResultLimit)},
			"page":                []string{fmt.Sprintf("%d", page)},
			"scanlation_group_id": []string{c.GroupID},
		}

		u.RawQuery = params.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("User-Agent", "mangarr")

		retryErr = retry.Do(func() error {
			resp, err := sharedhttp.ExecRequest(*c.Client, req)
			if err != nil {
				return fmt.Errorf("executing request %s: %w", req.URL, err)
			}

			buf := bufio.NewReader(resp.Body)

			err = json.NewDecoder(buf).Decode(&chapterResp)
			if err != nil {
				return retry.Unrecoverable(fmt.Errorf("decoding response: %w", err))
			}

			return nil
		},
			retry.Delay(time.Second*3),
			retry.Attempts(3),
			retry.MaxJitter(time.Second*1),
		)
		if retryErr != nil {
			return fmt.Errorf("executing request %s: %w", req.URL, retryErr)
		}

		for _, item := range chapterResp.Result.Items {
			if c.shouldProcessChapter(item, c.GroupID) {
				if processedChapters[item.Number] {
					continue
				}

				manga.Chapters[item.Number] = domain.Chapter{
					ID:     fmt.Sprintf("%d", item.ChapterID),
					Number: item.Number,
					Title:  sanitize.Filename(item.Name),
				}

				processedChapters[item.Number] = true
			}
		}

		if len(manga.Chapters) == 0 {
			return fmt.Errorf("getting chapters for manga ID %s", c.MangaURL)
		}

		if page == chapterResp.Result.Pagination.LastPage {
			return nil
		}

		page += 1
	}
}

func (c *comix) GetImageURLs(ctx context.Context, chapter *domain.Chapter) error {
	var chapterResp comixChapterResponse
	var imageInfos []domain.ImageInfo

	path, err := url.JoinPath(comixURL, "api/v2/chapters/", chapter.ID)
	if err != nil {
		return fmt.Errorf("building URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", "mangarr")

	retryErr := retry.Do(func() error {
		resp, err := sharedhttp.ExecRequest(*c.Client, req)
		if err != nil {
			return fmt.Errorf("executing request %s: %w", req.URL, err)
		}

		buf := bufio.NewReader(resp.Body)

		err = json.NewDecoder(buf).Decode(&chapterResp)
		if err != nil {
			return retry.Unrecoverable(fmt.Errorf("decoding response: %w", err))
		}

		return nil
	},
		retry.Delay(time.Second*3),
		retry.Attempts(3),
		retry.MaxJitter(time.Second*1),
	)
	if retryErr != nil {
		return fmt.Errorf("executing request %s: %w", req.URL, retryErr)
	}

	for _, imageURL := range chapterResp.Result.Images {
		imageInfos = append(imageInfos, domain.ImageInfo{ImageURL: imageURL})
	}

	if len(imageInfos) == 0 {
		return fmt.Errorf("getting image URLs for chapter ID %s", chapter.ID)
	}

	chapter.ImageInfo = imageInfos
	return nil
}

func (c *comix) shouldProcessChapter(data comixChapter, groupID string) bool {
	if len(groupID) == 0 {
		return true
	}

	if fmt.Sprintf("%d", data.ScanlationGroupID) == groupID {
		return true
	}

	return false
}

func (c *comix) extractIDFromURL(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("parsing URL: %w", err)
	}

	path := strings.Trim(u.Path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 || parts[0] != "title" {
		return "", fmt.Errorf("invalid URL format")
	}

	slug := parts[1]
	if idx := strings.Index(slug, "-"); idx != -1 {
		return slug[:idx], nil
	}

	return slug, nil
}
