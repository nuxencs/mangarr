package source

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/avast/retry-go"
	"mangarr/internal/sanitize"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/sharedhttp"
)

const (
	comickURL         = "https://api.comick.fun"
	comickResultLimit = 500

	// tag name for the "Long Strip" format to identify Manhwa
	comickLongStrip = "Long Strip"
)

type comick struct {
	MangaURL string
	GroupID  string
	Language string
	Client   *http.Client
}

type comickManga struct {
	FirstChap struct {
		Chap      string      `json:"chap"`
		Hid       string      `json:"hid"`
		Lang      string      `json:"lang"`
		GroupName []string    `json:"group_name"`
		Vol       interface{} `json:"vol"`
	} `json:"firstChap"`
	Comic struct {
		Id      int    `json:"id"`
		Hid     string `json:"hid"`
		Title   string `json:"title"`
		Country string `json:"country"`
		Status  int    `json:"status"`
		Links   struct {
			Al  string `json:"al"`
			Ap  string `json:"ap"`
			Mu  string `json:"mu"`
			Mal string `json:"mal"`
			Raw string `json:"raw"`
		} `json:"links"`
		LastChapter                          int         `json:"last_chapter"`
		ChapterCount                         int         `json:"chapter_count"`
		Demographic                          int         `json:"demographic"`
		UserFollowCount                      int         `json:"user_follow_count"`
		FollowRank                           int         `json:"follow_rank"`
		FollowCount                          int         `json:"follow_count"`
		Desc                                 string      `json:"desc"`
		Parsed                               string      `json:"parsed"`
		Slug                                 string      `json:"slug"`
		Mismatch                             interface{} `json:"mismatch"`
		Year                                 int         `json:"year"`
		BayesianRating                       interface{} `json:"bayesian_rating"`
		RatingCount                          int         `json:"rating_count"`
		ContentRating                        string      `json:"content_rating"`
		TranslationCompleted                 bool        `json:"translation_completed"`
		ChapterNumbersResetOnNewVolumeManual bool        `json:"chapter_numbers_reset_on_new_volume_manual"`
		FinalChapter                         interface{} `json:"final_chapter"`
		FinalVolume                          interface{} `json:"final_volume"`
		Noindex                              bool        `json:"noindex"`
		Adsense                              bool        `json:"adsense"`
		LoginRequired                        bool        `json:"login_required"`
		Recommendations                      []struct {
			Up      int `json:"up"`
			Down    int `json:"down"`
			Total   int `json:"total"`
			Relates struct {
				Title    string `json:"title"`
				Slug     string `json:"slug"`
				Hid      string `json:"hid"`
				MdCovers []struct {
					Vol   string `json:"vol"`
					W     int    `json:"w"`
					H     int    `json:"h"`
					B2Key string `json:"b2key"`
				} `json:"md_covers"`
			} `json:"relates"`
		} `json:"recommendations"`
		RelateFrom []struct {
			RelateTo struct {
				Slug  string `json:"slug"`
				Title string `json:"title"`
			} `json:"relate_to"`
			MdRelates struct {
				Name string `json:"name"`
			} `json:"md_relates"`
		} `json:"relate_from"`
		MdTitles []struct {
			Title string `json:"title"`
			Lang  string `json:"lang"`
		} `json:"md_titles"`
		MdComicMdGenres []struct {
			MdGenres struct {
				Name  string  `json:"name"`
				Type  *string `json:"type"`
				Slug  string  `json:"slug"`
				Group string  `json:"group"`
			} `json:"md_genres"`
		} `json:"md_comic_md_genres"`
		MdCovers []struct {
			Vol   string `json:"vol"`
			W     int    `json:"w"`
			H     int    `json:"h"`
			B2Key string `json:"b2key"`
		} `json:"md_covers"`
		MuComics struct {
			MuComicPublishers []struct {
				MuPublishers struct {
					Title string `json:"title"`
					Slug  string `json:"slug"`
				} `json:"mu_publishers"`
			} `json:"mu_comic_publishers"`
			LicensedInEnglish interface{} `json:"licensed_in_english"`
			MuComicCategories []struct {
				MuCategories struct {
					Title string `json:"title"`
					Slug  string `json:"slug"`
				} `json:"mu_categories"`
				PositiveVote int `json:"positive_vote"`
				NegativeVote int `json:"negative_vote"`
			} `json:"mu_comic_categories"`
		} `json:"mu_comics"`
		Iso6391    string `json:"iso639_1"`
		LangName   string `json:"lang_name"`
		LangNative string `json:"lang_native"`
	} `json:"comic"`
	Artists []struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"artists"`
	Authors []struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"authors"`
	LangList       []string    `json:"langList"`
	Recommendable  bool        `json:"recommendable"`
	Demographic    string      `json:"demographic"`
	EnglishLink    interface{} `json:"englishLink"`
	MatureContent  bool        `json:"matureContent"`
	CheckVol2Chap1 bool        `json:"checkVol2Chap1"`
}
type comickChapters struct {
	Chapters []comickChapterData `json:"chapters"`
	Total    int                 `json:"total"`
	Limit    int                 `json:"limit"`
}

type comickChapterData struct {
	Id               int        `json:"id"`
	Chap             string     `json:"chap"`
	Title            *string    `json:"title"`
	Vol              *string    `json:"vol"`
	Lang             string     `json:"lang"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	UpCount          int        `json:"up_count"`
	DownCount        int        `json:"down_count"`
	IsTheLastChapter bool       `json:"is_the_last_chapter"`
	PublishAt        *time.Time `json:"publish_at"`
	GroupName        []string   `json:"group_name"`
	Hid              string     `json:"hid"`
	Identities       *struct {
		Id     string `json:"id"`
		Traits struct {
			Username string `json:"username"`
			Gravatar string `json:"gravatar"`
		} `json:"traits"`
	} `json:"identities"`
	MdChaptersGroups []struct {
		MdGroups struct {
			Title string `json:"title"`
			Slug  string `json:"slug"`
		} `json:"md_groups"`
	} `json:"md_chapters_groups"`
}

func NewComick(mangaURL, group, language string) domain.Source {
	client := http.Client{
		Timeout:   60 * time.Second,
		Transport: sharedhttp.Transport,
	}

	return &comick{
		MangaURL: mangaURL,
		GroupID:  group,
		Language: language,
		Client:   &client,
	}
}

func (c *comick) String() string {
	return "ComicK"
}

func (c *comick) ValidateInput() error {
	if !strings.HasPrefix(c.MangaURL, "https://comick.io/comic") {
		return fmt.Errorf("the URL for ComicK must start with https://comick.io/comic")
	}

	if len(c.Language) == 0 {
		c.Language = "en"
	}

	return nil
}

func (c *comick) GetManga(ctx context.Context) (domain.Manga, error) {
	var mangaResp comickManga
	var isManhwa bool

	mangaSlug, err := c.extractSlug(c.MangaURL)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("failed to extract manga slug: %w", err)
	}

	path, err := url.JoinPath(comickURL, "comic", mangaSlug)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("failed to build URL: %w", err)
	}

	u, err := url.Parse(path)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("failed to parse URL %s: %w", path, err)
	}

	params := url.Values{
		"tachiyomi": []string{"true"},
	}

	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "mangarr")

	retryErr := retry.Do(func() error {
		resp, err := sharedhttp.ExecRequest(*c.Client, req)
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

	id := mangaResp.Comic.Hid
	if len(id) == 0 {
		return domain.Manga{}, fmt.Errorf("failed to get manga id for slug %s", mangaSlug)
	}

	title := mangaResp.Comic.Title
	if len(title) == 0 {
		return domain.Manga{}, fmt.Errorf("failed to get manga title for slug %s", mangaSlug)
	}

	for _, tag := range mangaResp.Comic.MdComicMdGenres {
		if tag.MdGenres.Name == comickLongStrip {
			isManhwa = true
			break
		}
	}

	return domain.Manga{
		ID:       id,
		Title:    sanitize.Filename(title),
		Chapters: make(map[float32]domain.Chapter),
		IsManhwa: isManhwa,
	}, nil
}

func (c *comick) GetChapters(ctx context.Context, manga domain.Manga) error {
	var chapterResp comickChapters

	path, err := url.JoinPath(comickURL, "comic", manga.ID, "chapters")
	if err != nil {
		return fmt.Errorf("failed to build URL: %w", err)
	}

	u, err := url.Parse(path)
	if err != nil {
		return fmt.Errorf("failed to parse URL %s: %w", path, err)
	}

	params := url.Values{
		"lang":      []string{c.Language},
		"limit":     []string{fmt.Sprintf("%d", comickResultLimit)},
		"tachiyomi": []string{"true"},
	}

	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "mangarr")

	retryErr := retry.Do(func() error {
		resp, err := sharedhttp.ExecRequest(*c.Client, req)
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

	for _, data := range chapterResp.Chapters {
		if len(c.GroupID) == 0 {
			err := c.processChapter(data, &manga)
			if err != nil {
				return fmt.Errorf("failed to process chapter: %w", err)
			}
		} else {
			for _, group := range data.MdChaptersGroups {
				if group.MdGroups.Title == c.GroupID {
					err := c.processChapter(data, &manga)
					if err != nil {
						return fmt.Errorf("failed to process chapter: %w", err)
					}
				}
			}
		}
	}

	if len(manga.Chapters) == 0 {
		return fmt.Errorf("failed to get chapters for manga %s", c.MangaURL)
	}

	return nil
}

func (c *comick) GetImageURLs(ctx context.Context, chapter *domain.Chapter) error {
	//TODO implement me
	panic("implement me")
}

func (c *comick) extractSlug(urlString string) (string, error) {
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return "", fmt.Errorf("error parsing URL: %w", err)
	}

	// Get the path and trim leading/trailing slashes
	path := strings.Trim(parsedURL.Path, "/")

	// Split the path and take the last segment
	pathSegments := strings.Split(path, "/")
	if len(pathSegments) > 0 {
		return pathSegments[len(pathSegments)-1], nil
	}

	return "", fmt.Errorf("no path segment found")
}

func (c *comick) processChapter(data comickChapterData, manga *domain.Manga) error {
	chapter := data.Chap

	chapterNum64, err := strconv.ParseFloat(chapter, 32)
	if err != nil {
		return fmt.Errorf("failed to parse chapter number from %s: %w", chapter, err)
	}
	chapterNum := float32(chapterNum64)

	var title string
	if data.Title != nil {
		title = *data.Title
	}

	manga.Chapters[chapterNum] = domain.Chapter{
		ID:     data.Hid,
		Number: chapterNum,
		Title:  sanitize.Filename(title),
	}

	return nil
}
