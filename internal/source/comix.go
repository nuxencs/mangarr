package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/sanitize"
	"mangarr/internal/sharedhttp"

	"github.com/avast/retry-go"
)

const (
	comixURL         = "https://comix.to"
	comixResultLimit = 100
)

type comix struct {
	MangaURL string
	GroupID  string
	BaseURL  string
	Client   *http.Client
	Codec    comixCodec
	CodecErr error
}

type comixManga struct {
	ID    int    `json:"id"`
	HID   string `json:"hid"`
	Title string `json:"title"`
	Type  string `json:"type"`
	URL   string `json:"url"`
}

type comixChapterList struct {
	Items []comixChapter `json:"items"`
	Meta  struct {
		Page     int `json:"page"`
		LastPage int `json:"lastPage"`
	} `json:"meta"`
}

type comixChapter struct {
	ID      int                  `json:"id"`
	MangaID int                  `json:"mangaId"`
	Number  domain.ChapterNumber `json:"number"`
	Name    string               `json:"name"`
	GroupID int                  `json:"groupId"`
	URL     string               `json:"url"`
}

type comixChapterDetail struct {
	comixChapter
	Pages comixPages `json:"pages"`
}

type comixPages struct {
	BaseURL string      `json:"baseUrl"`
	Items   []comixPage `json:"items"`
}

type comixPage struct {
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	URL       string `json:"url"`
	Scrambled int    `json:"s"`
}

func (p *comixPages) UnmarshalJSON(data []byte) error {
	var direct []comixPage
	if err := json.Unmarshal(data, &direct); err == nil {
		p.Items = direct
		return nil
	}

	type pagesAlias comixPages
	var compact pagesAlias
	if err := json.Unmarshal(data, &compact); err != nil {
		return fmt.Errorf("decoding Comix pages: %w", err)
	}
	*p = comixPages(compact)

	return nil
}

func NewComix(mangaURL, groupID string) domain.Source {
	jar, _ := cookiejar.New(nil)
	codec, codecErr := newComixCodec()
	client := http.Client{
		Timeout:   60 * time.Second,
		Transport: sharedhttp.Transport,
		Jar:       jar,
	}

	return &comix{
		MangaURL: mangaURL,
		GroupID:  groupID,
		BaseURL:  comixURL,
		Client:   &client,
		Codec:    codec,
		CodecErr: codecErr,
	}
}

func (c *comix) String() string {
	return "Comix"
}

func (c *comix) ValidateInput() error {
	if _, err := c.extractIDFromURL(c.MangaURL); err != nil {
		return err
	}
	if c.GroupID != "" {
		if _, err := strconv.ParseUint(c.GroupID, 10, 64); err != nil {
			return fmt.Errorf("Comix group ID must be numeric: %w", err)
		}
	}

	return nil
}

func (c *comix) Discover(ctx context.Context) (domain.Manga, error) {
	manga, err := c.getManga(ctx)
	if err != nil {
		return domain.Manga{}, err
	}
	if err := c.getChapters(ctx, manga); err != nil {
		return domain.Manga{}, err
	}

	return manga, nil
}

func (c *comix) getManga(ctx context.Context) (domain.Manga, error) {
	mangaID, err := c.extractIDFromURL(c.MangaURL)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("extracting Comix ID from URL %s: %w", c.MangaURL, err)
	}

	var response comixManga
	if err := c.get(ctx, comixAPIPath+"/manga/"+url.PathEscape(mangaID), nil, &response); err != nil {
		return domain.Manga{}, fmt.Errorf("getting Comix manga %s: %w", mangaID, err)
	}
	if response.HID == "" || response.Title == "" {
		return domain.Manga{}, fmt.Errorf("getting Comix manga %s: response is missing hid or title", mangaID)
	}

	return domain.Manga{
		ID:       response.HID,
		URL:      resolveComixURL(c.BaseURL, response.URL),
		Title:    sanitize.Filename(response.Title),
		Chapters: make(map[domain.ChapterNumber]domain.Chapter),
		IsManhwa: response.Type == "manhwa" || response.Type == "manhua",
	}, nil
}

func (c *comix) getChapters(ctx context.Context, manga domain.Manga) error {
	if manga.ID == "" {
		return fmt.Errorf("getting Comix chapters: manga ID is empty")
	}
	if manga.Chapters == nil {
		return fmt.Errorf("getting Comix chapters for manga %s: chapter map is nil", manga.ID)
	}

	processed := make(map[domain.ChapterNumber]struct{})
	for page := 1; ; page++ {
		params := comixParams{
			"limit": comixResultLimit,
			"order": map[string]any{"number": "desc"},
			"page":  page,
		}
		if c.GroupID != "" {
			params["group_id"] = c.GroupID
		}

		var response comixChapterList
		path := comixAPIPath + "/manga/" + url.PathEscape(manga.ID) + "/chapters"
		if err := c.get(ctx, path, params, &response); err != nil {
			return fmt.Errorf("getting Comix chapters for manga %s page %d: %w", manga.ID, page, err)
		}

		for _, item := range response.Items {
			if !c.shouldProcessChapter(item, c.GroupID) {
				continue
			}
			if _, exists := processed[item.Number]; exists {
				continue
			}

			manga.Chapters[item.Number] = domain.Chapter{
				ID:     strconv.Itoa(item.ID),
				URL:    resolveComixURL(c.BaseURL, item.URL),
				Number: item.Number,
				Title:  sanitize.Filename(item.Name),
			}
			processed[item.Number] = struct{}{}
		}

		if response.Meta.LastPage <= page {
			break
		}
	}

	if len(manga.Chapters) == 0 {
		return fmt.Errorf("getting Comix chapters for manga %s: response has no matching chapters", manga.ID)
	}

	return nil
}

func (c *comix) Pages(ctx context.Context, chapter domain.Chapter) ([]domain.ImageInfo, error) {
	if chapter.ID == "" {
		return nil, fmt.Errorf("getting Comix image URLs: chapter ID is empty")
	}

	var response comixChapterDetail
	if err := c.get(ctx, comixAPIPath+"/chapters/"+url.PathEscape(chapter.ID), nil, &response); err != nil {
		return nil, fmt.Errorf("getting Comix image URLs for chapter %s: %w", chapter.ID, err)
	}
	if len(response.Pages.Items) == 0 {
		return nil, fmt.Errorf("getting Comix image URLs for chapter %s: response has no pages", chapter.ID)
	}

	imageInfos := make([]domain.ImageInfo, 0, len(response.Pages.Items))
	for i, image := range response.Pages.Items {
		imageURL := resolveComixURL(response.Pages.BaseURL, image.URL)
		if imageURL == "" {
			return nil, fmt.Errorf("getting Comix image URLs for chapter %s: page %d URL is empty", chapter.ID, i+1)
		}
		imageInfo := domain.ImageInfo{
			ImageURL: imageURL,
			RequestHeaders: map[string]string{
				"Referer": resolveComixURL(c.BaseURL, "/"),
			},
		}
		if image.Scrambled != 0 {
			imageInfo.Processor = comixImageProcessor{}
		}
		imageInfos = append(imageInfos, imageInfo)
	}

	return imageInfos, nil
}

func (c *comix) get(ctx context.Context, path string, params comixParams, destination any) error {
	if c.CodecErr != nil {
		return fmt.Errorf("loading Comix frontend build %s codec: %w", comixFrontendBuild, c.CodecErr)
	}

	requestURL, err := url.JoinPath(c.BaseURL, path)
	if err != nil {
		return fmt.Errorf("building Comix request URL: %w", err)
	}
	token, err := c.Codec.token(requestURL, params)
	if err != nil {
		return fmt.Errorf("generating Comix request token: %w", err)
	}
	query, err := comixQuery(params)
	if err != nil {
		return err
	}
	query.Set(comixTokenParameter, token)

	parsed, err := url.Parse(requestURL)
	if err != nil {
		return fmt.Errorf("parsing Comix request URL: %w", err)
	}
	parsed.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return fmt.Errorf("creating Comix request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "mangarr")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	return retry.Do(func() error {
		resp, err := sharedhttp.ExecRequest(*c.Client, req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		encrypted := resp.Header.Get(comixResponseEncryptionHeader) == comixResponseEncryptionEnabled
		if err := c.Codec.decodeResponse(resp.Body, encrypted, destination); err != nil {
			return retry.Unrecoverable(err)
		}

		return nil
	}, sharedhttp.RetryOptions(ctx)...)
}

func comixQuery(params comixParams) (url.Values, error) {
	pairs := make([]string, 0, len(params))
	if err := flattenComixParams(&pairs, "", reflect.ValueOf(map[string]any(params))); err != nil {
		return nil, fmt.Errorf("encoding Comix query: %w", err)
	}

	values := make(url.Values, len(pairs))
	for _, pair := range pairs {
		key, value, _ := strings.Cut(pair, "=")
		values.Add(key, value)
	}

	return values, nil
}

func (c *comix) shouldProcessChapter(data comixChapter, groupID string) bool {
	return groupID == "" || strconv.Itoa(data.GroupID) == groupID
}

func (c *comix) extractIDFromURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parsing URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host != "comix.to" {
		return "", fmt.Errorf("URL must use https://comix.to")
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "title" || parts[1] == "" {
		return "", fmt.Errorf("URL must match https://comix.to/title/{id}-{slug}")
	}
	if mangaID, _, found := strings.Cut(parts[1], "-"); found && mangaID != "" {
		return mangaID, nil
	}

	return parts[1], nil
}

func resolveComixURL(baseURL, reference string) string {
	if reference == "" {
		return ""
	}
	parsedReference, err := url.Parse(reference)
	if err != nil {
		return ""
	}
	if parsedReference.IsAbs() {
		return parsedReference.String()
	}
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	return parsedBase.ResolveReference(parsedReference).String()
}
