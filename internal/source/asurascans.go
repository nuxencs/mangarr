package source

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"mangarr/internal/browser"
	"mangarr/internal/domain"
	"mangarr/internal/sanitize"

	"github.com/go-rod/rod"
)

const asurascansURL = "https://asuracomic.net/series/"

type asurascans struct {
	MangaURL string
	Browser  *browser.Manager
}

func NewAsurascans(mangaURL string, bm *browser.Manager) domain.Source {
	return &asurascans{
		MangaURL: mangaURL,
		Browser:  bm,
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

	page := a.Browser.Get().MustPage().Timeout(browser.Timeout)

	err := rod.Try(func() {
		page.MustNavigate(a.MangaURL).MustWaitDOMStable()

		titleElement := page.MustElement("span.text-xl.font-bold")
		manga.Title = sanitize.Filename(titleElement.MustText())
	})
	if err != nil {
		return domain.Manga{}, fmt.Errorf("failed to open manga page: %w", browser.HandleError(err))
	}
	defer page.MustClose()

	chapterElements, err := page.Elements(".pl-4.py-2")
	if err != nil {
		return domain.Manga{}, fmt.Errorf("failed to find chapter elements: %w", err)
	}

	for _, e := range chapterElements {
		chapterElement, err := e.Element(".flex")
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to get chapter element from URL %s: %w", a.MangaURL, err))
			continue
		}

		linkElement, err := e.Element("a")
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to get link element from URL %s: %w", a.MangaURL, err))
			continue
		}

		link, err := linkElement.Attribute("href")
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to get link from URL %s: %w", a.MangaURL, err))
		}
		chapterURL := *link

		chapterLine, err := chapterElement.Text()
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to get chapter line from URL %s: %w", a.MangaURL, err))
			continue
		}

		chapterNum, chapterTitle, err := a.splitChapterInfo(chapterLine)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to split chapter info %s: %w", chapterLine, err))
			continue
		}

		manga.Chapters[chapterNum] = domain.Chapter{
			URL:    chapterURL,
			Number: chapterNum,
			Title:  chapterTitle,
		}
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

	page := a.Browser.Get().MustPage().Timeout(browser.Timeout)
	var imageElements rod.Elements

	err := rod.Try(func() {
		page.MustNavigate(asurascansURL + chapter.URL).MustWaitDOMStable()

		imageElements = page.MustElements(".w-full.mx-auto img")
	})
	if err != nil {
		return fmt.Errorf("failed to open image page: %w", browser.HandleError(err))
	}
	defer page.MustClose()

	for _, e := range imageElements {
		imgURL, err := e.Attribute("src")
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to get image URL: %w", err))
		}

		// skip images that are not hosted on https://gg.asuracomic.net
		if strings.HasPrefix(*imgURL, "https://gg.asuracomic.net") {
			imageInfos = append(imageInfos, domain.ImageInfo{ImageURL: *imgURL})
		}
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

func (a *asurascans) splitChapterInfo(chapterLine string) (float32, string, error) {
	if len(chapterLine) == 0 {
		return 0, "", fmt.Errorf("chapter line is empty")
	}

	// splits a chapter line into split[0] = chapter part and split[1] = chapter title
	split := strings.Split(chapterLine, "\n")
	if len(split) > 2 {
		return 0, "", fmt.Errorf("chapter line is not in the correct format")
	}

	_, cutChapterLine, ok := strings.Cut(split[0], "Chapter ")
	if !ok {
		return 0, "", fmt.Errorf("failed to split chapter string %q", cutChapterLine)
	}

	chapterNumber, err := strconv.ParseFloat(cutChapterLine, 32)
	if err != nil {
		return 0, "", fmt.Errorf("failed to parse chapter number from %s: %w", cutChapterLine, err)
	}

	var chapterTitle string
	if len(split) == 2 {
		chapterTitle = sanitize.Filename(split[1])
	}

	return float32(chapterNumber), chapterTitle, nil
}
