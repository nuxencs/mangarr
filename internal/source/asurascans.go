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

	"github.com/go-rod/stealth"
)

const asurascansURL = "https://asuracomic.net/series/"

type asurascans struct {
	MangaURL string
	Browser  *browser.Manager
}

type asurascansPageData struct {
	Title    string                    `json:"title"`
	Chapters []asurascansChapterRecord `json:"chapters"`
}

type asurascansChapterRecord struct {
	ChapterLine  string `json:"chapterLine"`
	ChapterTitle string `json:"chapterTitle"`
	URL          string `json:"url"`
	EarlyAccess  bool   `json:"earlyAccess"`
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
	manga := domain.Manga{
		Chapters: make(map[float32]domain.Chapter),
		IsManhwa: true,
	}

	page := stealth.MustPage(a.Browser.Get())
	defer page.MustClose()

	pageWithTimeout := page.Timeout(browser.Timeout)
	var pageData asurascansPageData

	if err := pageWithTimeout.Navigate(a.MangaURL); err != nil {
		return domain.Manga{}, fmt.Errorf("navigating to manga page: %w", browser.HandleError(err))
	}

	if err := pageWithTimeout.WaitLoad(); err != nil {
		return domain.Manga{}, fmt.Errorf("waiting for manga page load: %w", browser.HandleError(err))
	}

	if err := pageWithTimeout.WaitElementsMoreThan(".pl-4.py-2", 0); err != nil {
		return domain.Manga{}, fmt.Errorf("waiting for manga chapter list: %w", browser.HandleError(err))
	}

	raw, err := pageWithTimeout.Eval(`() => {
		const title = document.querySelector('.font-bold.pb-3\\.5')?.textContent ?? ''
		const chapters = Array.from(document.querySelectorAll('.pl-4.py-2')).map((el) => {
			const chapterEl = el.querySelector('.flex')
			const chapterTitleEl = chapterEl?.querySelector('span')
			const linkEl = el.querySelector('a')
			return {
				chapterLine: chapterEl?.textContent?.trim() ?? '',
				chapterTitle: chapterTitleEl?.textContent?.trim() ?? '',
				url: linkEl?.getAttribute('href') ?? '',
				earlyAccess: Boolean(el.querySelector('svg')),
			}
		})

		return { title, chapters }
	}`)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("extracting manga page data: %w", browser.HandleError(err))
	}

	if err := raw.Value.Unmarshal(&pageData); err != nil {
		return domain.Manga{}, fmt.Errorf("decoding manga page data: %w", err)
	}

	manga.Title = sanitize.Filename(strings.TrimPrefix(pageData.Title, "Chapter "))
	for _, chapter := range pageData.Chapters {
		// skip early access chapters, they are only available to ASURA+ Premium members
		if chapter.EarlyAccess || chapter.URL == "" {
			continue
		}

		chapterNum, err := a.separateChapterNum(chapter.ChapterLine, chapter.ChapterTitle)
		if err != nil {
			continue
		}

		manga.Chapters[chapterNum] = domain.Chapter{
			URL:    chapter.URL,
			Number: chapterNum,
			Title:  sanitize.Filename(chapter.ChapterTitle),
		}
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
	var imageURLs []string

	page := stealth.MustPage(a.Browser.Get())
	defer page.MustClose()

	pageWithTimeout := page.Timeout(browser.Timeout)

	if err := pageWithTimeout.Navigate(asurascansURL + chapter.URL); err != nil {
		return fmt.Errorf("navigating to chapter page: %w", browser.HandleError(err))
	}

	if err := pageWithTimeout.WaitLoad(); err != nil {
		return fmt.Errorf("waiting for chapter page load: %w", browser.HandleError(err))
	}

	if err := pageWithTimeout.WaitElementsMoreThan(".w-full.mx-auto img", 0); err != nil {
		return fmt.Errorf("waiting for chapter images: %w", browser.HandleError(err))
	}

	raw, err := pageWithTimeout.Eval(`() => {
		return Array.from(document.querySelectorAll('.w-full.mx-auto img'))
			.map((img) => img.getAttribute('src') ?? '')
			.filter((src) => src.startsWith('https://gg.asuracomic.net'))
	}`)
	if err != nil {
		return fmt.Errorf("extracting chapter image URLs: %w", browser.HandleError(err))
	}

	if err := raw.Value.Unmarshal(&imageURLs); err != nil {
		return fmt.Errorf("decoding chapter image URLs: %w", err)
	}

	if len(imageURLs) == 0 {
		return fmt.Errorf("getting image URLs for chapter %g", chapter.Number)
	}

	imageInfos := make([]domain.ImageInfo, 0, len(imageURLs))
	for _, imageURL := range imageURLs {
		imageInfos = append(imageInfos, domain.ImageInfo{ImageURL: imageURL})
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
