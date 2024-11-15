package source

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/sanitize"

	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
)

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
		return fmt.Errorf("the url for flamecomics must start with https://flamecomics.xyz")
	}

	if _, err := url.Parse(f.MangaURL); err != nil {
		return err
	}

	return nil
}

func (f *flamecomics) GetManga(_ context.Context) (domain.Manga, error) {
	var manga domain.Manga
	manga.Chapters = make(map[float32]domain.Chapter)

	c := f.Collector.Clone()

	c.OnHTML(".entry-title", func(e *colly.HTMLElement) {
		manga.Title = sanitize.Filename(e.Text)
	})

	c.OnHTML(".eplister li", func(e *colly.HTMLElement) {
		chapterNum64, err := strconv.ParseFloat(e.Attr("data-num"), 32)
		if err != nil {
			return
		}

		chapterURL := e.ChildAttr("a", "href")
		chapterNum := float32(chapterNum64)

		manga.Chapters[chapterNum] = domain.Chapter{
			URL:      chapterURL,
			Number:   chapterNum,
			IsManhwa: true,
		}
	})

	err := c.Visit(f.MangaURL)
	if err != nil {
		return domain.Manga{}, err
	}

	if len(manga.Title) == 0 {
		return domain.Manga{}, fmt.Errorf("failed to get manga for provided url: %s", f.MangaURL)
	}

	if len(manga.Chapters) == 0 {
		return domain.Manga{}, fmt.Errorf("failed to get chapters for manga: %s", manga.Title)
	}

	return manga, nil
}

func (f *flamecomics) GetChapters(_ context.Context, _ domain.Manga) error {
	return nil
}

func (f *flamecomics) GetImageURLs(_ context.Context, chapter *domain.Chapter) error {
	c := f.Collector.Clone()

	var imageInfos []domain.ImageInfo

	c.OnHTML("#readerarea img", func(e *colly.HTMLElement) {
		imgURL := e.Attr("src")
		if strings.HasPrefix(imgURL, "https://flamecomics") {
			imageInfos = append(imageInfos, domain.ImageInfo{ImageURL: imgURL})
		}
	})

	err := c.Visit(chapter.URL)
	if err != nil {
		return err
	}

	if len(imageInfos) == 0 {
		return fmt.Errorf("failed to get image urls for chapter number: %g", chapter.Number)
	}

	chapter.ImageInfo = imageInfos
	return nil
}
