package source

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/sanitize"

	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
)

const (
	flamecomicsURL    = "https://flamecomics.xyz"
	flamecomicsCDNURL = "https://cdn.flamecomics.xyz"
)

type flamecomicsResponse struct {
	Props struct {
		PageProps struct {
			Series struct {
				SeriesId    int      `json:"series_id"`
				Title       string   `json:"title"`
				AltTitles   []string `json:"altTitles"`
				Description string   `json:"description"`
				Language    string   `json:"language"`
				Type        string   `json:"type"`
				Year        int      `json:"year"`
				Status      string   `json:"status"`
				Schedule    string   `json:"schedule"`
				LastEdit    int      `json:"last_edit"`
				Time        int      `json:"time"`
			} `json:"series"`
			Chapters []flamescansChapter `json:"chapters"`
		} `json:"pageProps"`
	} `json:"props"`
}

type flamescansChapter struct {
	ChapterID   int    `json:"chapter_id"`
	SeriesID    int    `json:"series_id"`
	Chapter     string `json:"chapter"`
	Title       string `json:"title"`
	ReleaseDate int64  `json:"release_date"`
	Token       string `json:"token"`
}

type flamecomicsChapterResponse struct {
	Props struct {
		PageProps struct {
			Chapter struct {
				SeriesId      int                        `json:"series_id"`
				ChapterId     int                        `json:"chapter_id"`
				Chapter       string                     `json:"chapter"`
				ChapterTitle  string                     `json:"chapter_title"`
				Images        map[string]flamescansImage `json:"images"`
				Token         string                     `json:"token"`
				ReleaseDate   int                        `json:"release_date"`
				EditTime      int                        `json:"edit_time"`
				UnixTimestamp int                        `json:"unix_timestamp"`
				Title         string                     `json:"title"`
			} `json:"chapter"`
		} `json:"pageProps"`
	} `json:"props"`
}

type flamescansImage struct {
	Size     int64  `json:"size"`
	Name     string `json:"name"`
	Modified string `json:"modified"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

var jsonRegex = regexp.MustCompile(`type="application/json">((?s).*?)</script>`)

type flamecomics struct {
	MangaURL  string
	Collector colly.Collector
}

func NewFlamecomics(mangaURL string) domain.Source {
	collector := colly.NewCollector(
		colly.AllowURLRevisit(),
	)
	extensions.RandomUserAgent(collector)

	collector.SetRequestTimeout(120 * time.Second)

	return &flamecomics{
		MangaURL:  mangaURL,
		Collector: *collector,
	}
}

func (f *flamecomics) String() string {
	return "Flame Comics"
}

func (f *flamecomics) ValidateInput() error {
	if !strings.HasPrefix(f.MangaURL, "https://flamecomics.xyz") {
		return fmt.Errorf("the URL for Flame Comics must start with https://flamecomics.xyz")
	}

	if _, err := url.Parse(f.MangaURL); err != nil {
		return fmt.Errorf("parsing URL %s: %w", f.MangaURL, err)
	}

	return nil
}

func (f *flamecomics) GetManga(_ context.Context) (domain.Manga, error) {
	var responseData flamecomicsResponse
	var errors []error

	manga := domain.Manga{
		Chapters: make(map[domain.ChapterNumber]domain.Chapter),
		IsManhwa: true,
	}

	c := f.Collector.Clone()

	c.OnError(func(r *colly.Response, err error) {
		errors = append(errors, fmt.Errorf("requesting URL %s: %w", r.Request.URL, err))
	})

	c.OnResponse(func(r *colly.Response) {
		match := jsonRegex.FindSubmatch(r.Body)
		if len(match) < 2 {
			errors = append(errors, fmt.Errorf("finding chapter data in response from URL %s", r.Request.URL))
			return
		}

		buf := bytes.NewReader(match[1])

		err := json.NewDecoder(buf).Decode(&responseData)
		if err != nil {
			errors = append(errors, fmt.Errorf("decoding chapter data from json: %w", err))
			return
		}
	})

	err := c.Visit(f.MangaURL)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("visiting URL %s: %w", f.MangaURL, err)
	}

	if len(errors) > 0 {
		return domain.Manga{}, fmt.Errorf("processing %d URLs: %w", len(errors), errors[0])
	}

	manga.Title = sanitize.Filename(responseData.Props.PageProps.Series.Title)

	for _, responseChapter := range responseData.Props.PageProps.Chapters {
		chapterNum, err := domain.ParseChapterNumber(responseChapter.Chapter)
		if err != nil {
			return domain.Manga{}, fmt.Errorf("parsing chapter number %s: %w", responseChapter.Chapter, err)
		}

		path, err := url.JoinPath(flamecomicsURL, "series", fmt.Sprintf("%d", responseChapter.SeriesID), responseChapter.Token)
		if err != nil {
			return domain.Manga{}, fmt.Errorf("building URL: %w", err)
		}

		manga.Chapters[chapterNum] = domain.Chapter{
			URL:    path,
			Number: chapterNum,
			Title:  responseChapter.Title,
		}
	}

	if len(manga.Title) == 0 {
		return domain.Manga{}, fmt.Errorf("getting manga for URL %s", f.MangaURL)
	}

	if len(manga.Chapters) == 0 {
		return domain.Manga{}, fmt.Errorf("getting chapters for manga %s", manga.Title)
	}

	return manga, nil
}

func (f *flamecomics) GetChapters(_ context.Context, _ domain.Manga) error {
	return nil
}

func (f *flamecomics) GetImageURLs(_ context.Context, chapter *domain.Chapter) error {
	var chapterResponse flamecomicsChapterResponse
	var errors []error
	c := f.Collector.Clone()

	c.OnError(func(r *colly.Response, err error) {
		errors = append(errors, fmt.Errorf("requesting URL %s: %w", r.Request.URL, err))
	})

	c.OnResponse(func(r *colly.Response) {
		match := jsonRegex.FindSubmatch(r.Body)
		if len(match) < 2 {
			errors = append(errors, fmt.Errorf("finding chapter data in response from URL %s", r.Request.URL))
			return
		}

		buf := bytes.NewReader(match[1])

		err := json.NewDecoder(buf).Decode(&chapterResponse)
		if err != nil {
			errors = append(errors, fmt.Errorf("decoding chapter data from json: %w", err))
			return
		}
	})

	err := c.Visit(chapter.URL)
	if err != nil {
		return fmt.Errorf("visiting URL %s: %w", chapter.URL, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("processing %d URLs: %w", len(errors), errors[0])
	}

	responseChapter := chapterResponse.Props.PageProps.Chapter

	keys := make([]string, 0, len(responseChapter.Images))
	for k := range responseChapter.Images {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		iNum, _ := strconv.ParseFloat(keys[i], 32)
		jNum, _ := strconv.ParseFloat(keys[j], 32)
		return iNum < jNum
	})

	imageURLs := make([]domain.ImageInfo, 0, len(responseChapter.Images))
	for _, key := range keys {
		i := responseChapter.Images[key]

		imageURLs = append(imageURLs, domain.ImageInfo{
			ImageURL: fmt.Sprintf("%s/uploads/images/series/%d/%s/%s",
				flamecomicsCDNURL,
				chapterResponse.Props.PageProps.Chapter.SeriesId,
				responseChapter.Token,
				i.Name,
			),
		})
	}

	if len(imageURLs) == 0 {
		return fmt.Errorf("getting image URLs for chapter %s", chapter.Number)
	}

	chapter.ImageInfo = imageURLs
	return nil
}
