package source

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mangarr/internal/browser"
	"mangarr/internal/domain"
	"mangarr/internal/sanitize"

	"github.com/go-rod/rod"
	"github.com/go-rod/stealth"
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
		return fmt.Errorf("parsing URL %s: %w", a.MangaURL, err)
	}

	return nil
}

func (a *asurascans) GetManga(_ context.Context) (domain.Manga, error) {
	var errors []error

	manga := domain.Manga{
		Chapters: make(map[float32]domain.Chapter),
		IsManhwa: true,
	}

	page := stealth.MustPage(a.Browser.Get())
	defer page.MustClose()

	pageWithTimeout := page.Timeout(browser.Timeout)

	err := rod.Try(func() {
		_ = pageWithTimeout.MustNavigate(a.MangaURL).WaitDOMStable(time.Second, 1)

		titleElement := pageWithTimeout.MustElement(".font-bold.pb-3\\.5")
		manga.Title = sanitize.Filename(strings.TrimPrefix(titleElement.MustText(), "Chapter "))
	})
	if err != nil {
		return domain.Manga{}, fmt.Errorf("opening manga page: %w", browser.HandleError(err))
	}

	chapterElements, err := pageWithTimeout.Elements(".pl-4.py-2")
	if err != nil {
		return domain.Manga{}, fmt.Errorf("finding chapter elements: %w", err)
	}

	for _, e := range chapterElements {
		// skip early access chapters as they are only available to ASURA+ Premium members
		isEarlyAccess, _, err := e.Has("svg")
		if err != nil {
			errors = append(errors, fmt.Errorf("determine if chapter is early access: %w", err))
			continue
		}
		if isEarlyAccess {
			continue
		}

		chapterElement, err := e.Element(".flex")
		if err != nil {
			errors = append(errors, fmt.Errorf("getting chapter element from URL %s: %w", a.MangaURL, err))
			continue
		}

		chapterTitleElement, err := chapterElement.Element("span")
		if err != nil {
			errors = append(errors, fmt.Errorf("getting chapter title element from URL %s: %w", a.MangaURL, err))
			continue
		}

		chapterLine, err := chapterElement.Text()
		if err != nil {
			errors = append(errors, fmt.Errorf("getting chapter line from URL %s: %w", a.MangaURL, err))
			continue
		}

		chapterTitleLine, err := chapterTitleElement.Text()
		if err != nil {
			errors = append(errors, fmt.Errorf("getting chapter line from URL %s: %w", a.MangaURL, err))
			continue
		}

		linkElement, err := e.Element("a")
		if err != nil {
			errors = append(errors, fmt.Errorf("getting link element from URL %s: %w", a.MangaURL, err))
			continue
		}

		link, err := linkElement.Attribute("href")
		if err != nil {
			errors = append(errors, fmt.Errorf("getting link from URL %s: %w", a.MangaURL, err))
			continue
		}
		chapterURL := *link

		chapterNum, err := a.separateChapterNum(chapterLine, chapterTitleLine)
		if err != nil {
			errors = append(errors, fmt.Errorf("splitting chapter info %s: %w", chapterLine, err))
			continue
		}

		manga.Chapters[chapterNum] = domain.Chapter{
			URL:    chapterURL,
			Number: chapterNum,
			Title:  sanitize.Filename(chapterTitleLine),
		}
	}

	if len(errors) > 0 {
		return domain.Manga{}, fmt.Errorf("processing %d URLs: %w", len(errors), errors[0])
	}

	if len(manga.Title) == 0 {
		return domain.Manga{}, fmt.Errorf("getting manga for URL %s", a.MangaURL)
	}

	if len(manga.Chapters) == 0 {
		return domain.Manga{}, fmt.Errorf("getting chapters for manga %s", manga.Title)
	}

	return manga, nil
}

func (a *asurascans) GetChapters(_ context.Context, _ domain.Manga) error {
	return nil
}

func (a *asurascans) GetImageURLs(_ context.Context, chapter *domain.Chapter) error {
	var imageInfos []domain.ImageInfo
	var imageElements rod.Elements
	var errors []error

	page := stealth.MustPage(a.Browser.Get())
	defer page.MustClose()

	pageWithTimeout := page.Timeout(browser.Timeout)

	err := rod.Try(func() {
		_ = pageWithTimeout.MustNavigate(asurascansURL+chapter.URL).WaitDOMStable(time.Second, 1)

		imageElements = pageWithTimeout.MustElements(".w-full.mx-auto img")
	})
	if err != nil {
		return fmt.Errorf("opening image page: %w", browser.HandleError(err))
	}

	for _, e := range imageElements {
		imgURL, err := e.Attribute("src")
		if err != nil {
			errors = append(errors, fmt.Errorf("getting image URL: %w", err))
		}

		if imgURL == nil {
			continue
		}

		// skip images that are not hosted on https://gg.asuracomic.net
		if strings.HasPrefix(*imgURL, "https://gg.asuracomic.net") {
			imageInfos = append(imageInfos, domain.ImageInfo{ImageURL: *imgURL})
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("processing %d URLs: %w", len(errors), errors[0])
	}

	if len(imageInfos) == 0 {
		return fmt.Errorf("getting image URLs for chapter %g", chapter.Number)
	}

	chapter.ImageInfo = imageInfos
	return nil
}

func (a *asurascans) separateChapterNum(chapterLine, chapterTitleLine string) (float32, error) {
	if len(chapterLine) == 0 {
		return 0, fmt.Errorf("chapter line is empty")
	}

	trimmed := strings.TrimSuffix(chapterLine, chapterTitleLine)
	trimmed = strings.TrimSuffix(trimmed, "\n")
	trimmed = strings.TrimPrefix(trimmed, "Chapter ")

	chapterNumber, err := strconv.ParseFloat(trimmed, 32)
	if err != nil {
		return 0, fmt.Errorf("parsing chapter number from %s: %w", trimmed, err)
	}

	return float32(chapterNumber), nil
}
