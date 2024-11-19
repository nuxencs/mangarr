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

const asurascansURL = "https://asuracomic.net/series/"

type asurascans struct {
	MangaURL  string
	Collector colly.Collector
}

func NewAsurascans(mangaURL string) domain.Source {
	collector := colly.NewCollector(
		colly.AllowURLRevisit(),
	)
	extensions.RandomUserAgent(collector)

	collector.SetRequestTimeout(120 * time.Second)

	return &asurascans{
		MangaURL:  mangaURL,
		Collector: *collector,
	}
}

func (a *asurascans) String() string {
	return "Asura Scans"
}

func (a *asurascans) ValidateInput() error {
	if !strings.HasPrefix(a.MangaURL, "https://asuracomic.net") {
		return fmt.Errorf("the URL for Asura Scans must start with https://asuracomic.net")
	}

	if _, err := url.Parse(a.MangaURL); err != nil {
		return fmt.Errorf("failed to parse URL %s: %w", a.MangaURL, err)
	}

	return nil
}

func (a *asurascans) GetManga(_ context.Context) (domain.Manga, error) {
	var errors []error

	manga := domain.Manga{
		Chapters: make(map[float32]domain.Chapter),
		IsManhwa: true,
	}

	c := a.Collector.Clone()

	c.OnError(func(r *colly.Response, err error) {
		errors = append(errors, fmt.Errorf("failed to request URL %s: %w", r.Request.URL, err))
	})

	c.OnHTML("span.text-xl.font-bold", func(e *colly.HTMLElement) {
		manga.Title = sanitize.Filename(e.Text)
	})

	c.OnHTML(".pl-4.pr-2.pb-4 a", func(e *colly.HTMLElement) {
		chapterTitle := e.ChildText("span")

		chapterNum, err := a.splitChapterInfo(e.Text, chapterTitle)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to parse chapter info %q from URL %s: %w", e.Text, e.Request.URL, err))
			return
		}

		chapterURL := e.Attr("href")
		chapterTitle = sanitize.Filename(chapterTitle)

		manga.Chapters[chapterNum] = domain.Chapter{
			URL:    chapterURL,
			Number: chapterNum,
			Title:  chapterTitle,
		}
	})

	err := c.Visit(a.MangaURL)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("failed to visit URL %s: %w", a.MangaURL, err)
	}

	if len(errors) > 0 {
		return domain.Manga{}, fmt.Errorf("failed to process %d URLs: %w", len(errors), errors[0])
	}

	if len(manga.Title) == 0 {
		return domain.Manga{}, fmt.Errorf("failed to get manga for URL %s", a.MangaURL)
	}

	if len(manga.Chapters) == 0 {
		return domain.Manga{}, fmt.Errorf("failed to get chapters for manga %s", manga.Title)
	}

	return manga, nil
}

func (a *asurascans) GetChapters(_ context.Context, _ domain.Manga) error {
	return nil
}

func (a *asurascans) GetImageURLs(_ context.Context, chapter *domain.Chapter) error {
	var imageInfos []domain.ImageInfo
	var errors []error

	c := a.Collector.Clone()

	c.OnError(func(r *colly.Response, err error) {
		errors = append(errors, fmt.Errorf("failed to request URL %s: %w", r.Request.URL, err))
	})

	c.OnHTML(".w-full.mx-auto img", func(e *colly.HTMLElement) {
		imgURL := e.Attr("src")

		// skip images that are not hosted on https://gg.asuracomic.net
		if strings.HasPrefix(imgURL, "https://gg.asuracomic.net") {
			imageInfos = append(imageInfos, domain.ImageInfo{ImageURL: imgURL})
		}
	})

	err := c.Visit(asurascansURL + chapter.URL)
	if err != nil {
		return fmt.Errorf("failed to visit URL %s: %w", chapter.URL, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to process %d URLs: %w", len(errors), errors[0])
	}

	if len(imageInfos) == 0 {
		return fmt.Errorf("failed to get image URLs for chapter %g", chapter.Number)
	}

	chapter.ImageInfo = imageInfos
	return nil
}

func (a *asurascans) splitChapterInfo(chapterLine string, chapterTitle string) (float32, error) {
	cutChapterLine := chapterLine

	if len(chapterTitle) != 0 {
		// not checking for found, because if chapterTitle is not found in chapterLine, cutChapterLine will be set
		// to chapterLine which already is in the format "Chapter Number"
		cutChapterLine, _, _ = strings.Cut(chapterLine, chapterTitle)
	}

	_, cutChapterLine, ok := strings.Cut(cutChapterLine, "Chapter ")
	if !ok {
		return 0, fmt.Errorf("failed to split chapter string %q", cutChapterLine)
	}

	chapterNumber, err := strconv.ParseFloat(cutChapterLine, 32)
	if err != nil {
		return 0, fmt.Errorf("failed to parse chapter number from %s: %w", cutChapterLine, err)
	}

	chapterTitle = sanitize.Filename(chapterTitle)

	return float32(chapterNumber), nil
}
