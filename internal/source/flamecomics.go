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

const flamecomicsCDNURL = "https://cdn.flamecomics.xyz"

type flamecomicsResponse struct {
	Props struct {
		PageProps struct {
			Series struct {
				SeriesId    int    `json:"series_id"`
				Title       string `json:"title"`
				AltTitles   string `json:"altTitles"`
				Description string `json:"description"`
				Language    string `json:"language"`
				Type        string `json:"type"`
				Year        int    `json:"year"`
				Status      string `json:"status"`
				Schedule    string `json:"schedule"`
				LastEdit    string `json:"last_edit"`
				Time        int    `json:"time"`
			} `json:"series"`
			Chapters []flamescansChapter `json:"chapters"`
		} `json:"pageProps"`
	} `json:"props"`
	Query struct {
		Id string `json:"id"`
	} `json:"query"`
}

type flamescansChapter struct {
	ChapterID     int                        `json:"chapter_id"`
	SeriesID      int                        `json:"series_id"`
	Chapter       string                     `json:"chapter"`
	Title         string                     `json:"title"`
	Images        map[string]flamescansImage `json:"images"`
	Language      string                     `json:"language"`
	ReleaseDate   int64                      `json:"release_date"`
	Token         string                     `json:"token"`
	UnixTimestamp int64                      `json:"unix_timestamp"`
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
		Chapters: make(map[float32]domain.Chapter),
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
		chapterNumF64, err := strconv.ParseFloat(responseChapter.Chapter, 32)
		if err != nil {
			return domain.Manga{}, fmt.Errorf("parsing chapter number %s: %w", responseChapter.Chapter, err)
		}

		chapterNum := float32(chapterNumF64)

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
					responseData.Props.PageProps.Series.SeriesId,
					responseChapter.Token,
					i.Name,
				),
			})
		}

		manga.Chapters[chapterNum] = domain.Chapter{
			ID:        responseChapter.Token,
			Number:    chapterNum,
			Title:     responseChapter.Title,
			ImageInfo: imageURLs,
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

func (f *flamecomics) GetImageURLs(_ context.Context, _ *domain.Chapter) error {
	return nil
}
