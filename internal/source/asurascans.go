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
		return fmt.Errorf("the url for asurascans must start with https://asuracomic.net")
	}

	if _, err := url.Parse(a.MangaURL); err != nil {
		return err
	}

	return nil
}

func (a *asurascans) GetManga(_ context.Context) (domain.Manga, error) {
	var manga domain.Manga
	manga.Chapters = make(map[float32]domain.Chapter)

	c := a.Collector.Clone()

	c.OnHTML("span.text-xl.font-bold", func(e *colly.HTMLElement) {
		manga.Title = sanitize.Filename(e.Text)
	})

	c.OnHTML(".pl-4.pr-2.pb-4 a", func(e *colly.HTMLElement) {
		chapterNum, chapterTitle, err := a.splitChapterInfo(e.Text)
		if err != nil {
			return
		}

		chapterURL := e.Attr("href")

		manga.Chapters[chapterNum] = domain.Chapter{
			URL:      chapterURL,
			Number:   chapterNum,
			Title:    chapterTitle,
			IsManhwa: true,
		}
	})

	err := c.Visit(a.MangaURL)
	if err != nil {
		return domain.Manga{}, err
	}

	if len(manga.Title) == 0 {
		return domain.Manga{}, fmt.Errorf("failed to get manga for provided url: %s", a.MangaURL)
	}

	if len(manga.Chapters) == 0 {
		return domain.Manga{}, fmt.Errorf("failed to get chapters for manga: %s", manga.Title)
	}

	return manga, nil
}

func (a *asurascans) GetChapters(_ context.Context, _ domain.Manga) error {
	return nil
}

func (a *asurascans) GetImageURLs(_ context.Context, chapter *domain.Chapter) error {
	c := a.Collector.Clone()

	var imageInfos []domain.ImageInfo

	c.OnHTML(".w-full.mx-auto img", func(e *colly.HTMLElement) {
		imgURL := e.Attr("src")
		if strings.HasPrefix(imgURL, "https://gg.asuracomic.net") {
			imageInfos = append(imageInfos, domain.ImageInfo{ImageURL: imgURL})
		}
	})

	err := c.Visit(asurascansURL + chapter.URL)
	if err != nil {
		return err
	}

	if len(imageInfos) == 0 {
		return fmt.Errorf("failed to get image urls for chapter number: %g", chapter.Number)
	}

	chapter.ImageInfo = imageInfos
	return nil
}

func (a *asurascans) splitChapterInfo(input string) (float32, string, error) {
	parts := strings.SplitN(input, " ", 3)
	chapterNumberStr := parts[1]
	chapterNumber, err := strconv.ParseFloat(chapterNumberStr, 32)
	if err != nil {
		return 0, "", err
	}

	chapterTitle := strings.TrimSpace(strings.Join(parts[2:], " "))
	chapterTitle = sanitize.Filename(chapterTitle)

	return float32(chapterNumber), chapterTitle, nil
}
