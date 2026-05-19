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

	"mangarr/internal/domain"
	"mangarr/internal/sanitize"
	"mangarr/internal/sharedhttp"

	"github.com/avast/retry-go"
)

const (
	comixURL         = "https://comix.to"
	comixResultLimit = 100

	comixOfficialGroupID = "9275"
	comixDeprecatedError = "Comix is deprecated because chapter pages now require a browser-generated token"
)

type comix struct {
	MangaURL string
	GroupID  string
	Client   *http.Client
}

type comixMangaResponse struct {
	Status int `json:"status"`
	Result struct {
		MangaID   int      `json:"manga_id"`
		HashID    string   `json:"hash_id"`
		Title     string   `json:"title"`
		AltTitles []string `json:"alt_titles"`
		Synopsis  string   `json:"synopsis"`
		Slug      string   `json:"slug"`
		Rank      int      `json:"rank"`
		Type      string   `json:"type"`
		Poster    struct {
			Small  string `json:"small"`
			Medium string `json:"medium"`
			Large  string `json:"large"`
		} `json:"poster"`
		OriginalLanguage string  `json:"original_language"`
		Status           string  `json:"status"`
		FinalVolume      int     `json:"final_volume"`
		FinalChapter     int     `json:"final_chapter"`
		HasChapters      bool    `json:"has_chapters"`
		LatestChapter    int     `json:"latest_chapter"`
		ChapterUpdatedAt int     `json:"chapter_updated_at"`
		StartDate        int     `json:"start_date"`
		EndDate          string  `json:"end_date"`
		CreatedAt        int     `json:"created_at"`
		UpdatedAt        int     `json:"updated_at"`
		RatedAvg         float64 `json:"rated_avg"`
		RatedCount       int     `json:"rated_count"`
		FollowsTotal     int     `json:"follows_total"`
		Links            struct {
			Al  string `json:"al"`
			Mal string `json:"mal"`
			Mu  string `json:"mu"`
		} `json:"links"`
		IsNsfw  bool  `json:"is_nsfw"`
		Year    int   `json:"year"`
		TermIds []int `json:"term_ids"`
	} `json:"result"`
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
		ChapterID         int                  `json:"chapter_id"`
		MangaID           int                  `json:"manga_id"`
		ScanlationGroupID int                  `json:"scanlation_group_id"`
		Number            domain.ChapterNumber `json:"number"`
		Name              string               `json:"name"`
		Language          string               `json:"language"`
		Volume            int                  `json:"volume"`
		Votes             int                  `json:"votes"`
		CreatedAt         int                  `json:"created_at"`
		UpdatedAt         int                  `json:"updated_at"`
		ScanlationGroup   struct {
			ScanlationGroupID int    `json:"scanlation_group_id"`
			Name              string `json:"name"`
			Slug              string `json:"slug"`
		} `json:"scanlation_group"`
		Images []comixImage `json:"images"`
		Prev   comixChapter `json:"prev"`
		Next   comixChapter `json:"next"`
	} `json:"result"`
}

type comixChapter struct {
	ChapterID         int                  `json:"chapter_id"`
	MangaID           int                  `json:"manga_id"`
	ScanlationGroupID int                  `json:"scanlation_group_id"`
	Number            domain.ChapterNumber `json:"number"`
	Name              string               `json:"name"`
	Language          string               `json:"language"`
	Volume            int                  `json:"volume"`
	Votes             int                  `json:"votes"`
	CreatedAt         int                  `json:"created_at"`
	UpdatedAt         int                  `json:"updated_at"`
	ScanlationGroup   struct {
		ScanlationGroupID int    `json:"scanlation_group_id"`
		Name              string `json:"name"`
		Slug              string `json:"slug"`
	} `json:"scanlation_group"`
	Images []comixImage `json:"images"`
}

type comixImage struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	URL    string `json:"url"`
}

func NewComix(mangaURL, groupID string) domain.Source {
	client := http.Client{
		Timeout:   60 * time.Second,
		Transport: sharedhttp.Transport,
	}

	return &comix{
		MangaURL: mangaURL,
		GroupID:  groupID,
		Client:   &client,
	}
}

func (c *comix) String() string {
	return "Comix"
}

func (c *comix) ValidateInput() error {
	return fmt.Errorf(comixDeprecatedError)
}

func (c *comix) GetManga(ctx context.Context) (domain.Manga, error) {
	var (
		mangaResp comixMangaResponse
		manga     domain.Manga
	)

	mangaID, err := c.extractIDFromURL(c.MangaURL)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("extracting ID from URL %s: %w", c.MangaURL, err)
	}

	path, err := url.JoinPath(comixURL, "api/v2/manga/", mangaID)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("building URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
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

	if mangaResp.Status != http.StatusOK {
		return domain.Manga{}, fmt.Errorf("unexpected status code: %d", mangaResp.Status)
	}

	manga = domain.Manga{
		ID:       mangaID,
		Title:    sanitize.Filename(mangaResp.Result.Title),
		Chapters: make(map[domain.ChapterNumber]domain.Chapter),
	}

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

	processedChapters := make(map[domain.ChapterNumber]bool)

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

	for _, image := range chapterResp.Result.Images {
		imageInfos = append(imageInfos, domain.ImageInfo{ImageURL: image.URL})
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
	if before, _, ok := strings.Cut(slug, "-"); ok {
		return before, nil
	}

	return slug, nil
}
