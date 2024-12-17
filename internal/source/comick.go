package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/sanitize"
	"mangarr/internal/sharedhttp"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

const (
	comickURL         = "https://api.comick.fun"
	comickImageHost   = "https://meo.comick.pictures"
	comickResultLimit = 5000

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
	Comic struct {
		Hid             string `json:"hid"`
		Title           string `json:"title"`
		MdComicMdGenres []struct {
			MdGenres struct {
				Name  string  `json:"name"`
				Type  *string `json:"type"`
				Slug  string  `json:"slug"`
				Group string  `json:"group"`
			} `json:"md_genres"`
		} `json:"md_comic_md_genres"`
	} `json:"comic"`
}

type comickChapters struct {
	Chapters []comickChapterData `json:"chapters"`
	Total    int                 `json:"total"`
	Limit    int                 `json:"limit"`
}

type comickChapterData struct {
	Chap             string     `json:"chap"`
	Title            *string    `json:"title"`
	Vol              *string    `json:"vol"`
	PublishAt        *time.Time `json:"publish_at"`
	GroupName        []string   `json:"group_name"`
	Hid              string     `json:"hid"`
	MdChaptersGroups []struct {
		MdGroups struct {
			Title string `json:"title"`
			Slug  string `json:"slug"`
		} `json:"md_groups"`
	} `json:"md_chapters_groups"`
}

type comickImageData []struct {
	H         int    `json:"h"`
	W         int    `json:"w"`
	Name      string `json:"name"`
	S         int    `json:"s"`
	B2Key     string `json:"b2key"`
	Optimized int    `json:"optimized"`
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

func (c *comick) GetManga(_ context.Context) (domain.Manga, error) {
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

	jsonResp, err := c.fetchJSON(path)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("failed to fetch json from URL %s: %w", path, err)
	}

	err = json.NewDecoder(strings.NewReader(jsonResp)).Decode(&mangaResp)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("failed to decode response: %w", err)
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

func (c *comick) GetChapters(_ context.Context, manga domain.Manga) error {
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
		"lang":  []string{c.Language},
		"limit": []string{fmt.Sprintf("%d", comickResultLimit)},
	}

	u.RawQuery = params.Encode()

	jsonResp, err := c.fetchJSON(u.String())
	if err != nil {
		return fmt.Errorf("failed to fetch json from URL %s: %w", u.String(), err)
	}

	err = json.NewDecoder(strings.NewReader(jsonResp)).Decode(&chapterResp)
	if err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
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

func (c *comick) GetImageURLs(_ context.Context, chapter *domain.Chapter) error {
	var imageResp comickImageData
	var imageInfos []domain.ImageInfo

	path, err := url.JoinPath(comickURL, "chapter", chapter.ID, "get_images")
	if err != nil {
		return fmt.Errorf("failed to build URL: %w", err)
	}

	jsonResp, err := c.fetchJSON(path)
	if err != nil {
		return fmt.Errorf("failed to fetch json from URL %s: %w", path, err)
	}

	err = json.NewDecoder(strings.NewReader(jsonResp)).Decode(&imageResp)
	if err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	for _, image := range imageResp {
		imagePath, err := url.JoinPath(comickImageHost, image.B2Key)
		if err != nil {
			return fmt.Errorf("failed to build URL: %w", err)
		}

		imageInfos = append(imageInfos, domain.ImageInfo{ImageURL: imagePath})
	}

	if len(imageInfos) == 0 {
		return fmt.Errorf("failed to get image URLs for chapter ID %s", chapter.ID)
	}

	chapter.ImageInfo = imageInfos
	return nil
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

func (c *comick) fetchJSON(path string) (string, error) {
	launcherPath, _ := launcher.LookPath()
	launcherURL := launcher.New().Bin(launcherPath).MustLaunch()
	browser := rod.New().ControlURL(launcherURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage(path).MustWaitDOMStable()
	resp, err := page.Element("pre")
	if err != nil {
		return "", fmt.Errorf("failed to fetch chapters page: %w", err)
	}

	jsonResp, err := resp.Text()
	if err != nil {
		return "", fmt.Errorf("failed to get json from html: %w", err)
	}

	return jsonResp, nil
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
