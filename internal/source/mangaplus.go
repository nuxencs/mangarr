package source

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/protobuf"
	"mangarr/internal/sanitize"
	"mangarr/internal/sharedhttp"

	"github.com/avast/retry-go"
	"google.golang.org/protobuf/proto"
)

const mangaplusURL = "https://jumpg-webapi.tokyo-cdn.com/api"

var mangaplusID = regexp.MustCompile(`^[1-9][0-9][0-9][0-9][0-9][0-9]$`)

type mangaplus struct {
	MangaID string
	Client  *http.Client
}

func NewMangaPlus(mangaID string) domain.Source {
	client := &http.Client{
		Timeout:   60 * time.Second,
		Transport: sharedhttp.Transport,
	}

	return &mangaplus{
		MangaID: mangaID,
		Client:  client,
	}
}

func (m *mangaplus) String() string {
	return "MANGA Plus"
}

func (m *mangaplus) ValidateInput() error {
	if len(m.MangaID) == 0 {
		return fmt.Errorf("manga plus manga ID is required")
	}

	if !mangaplusID.MatchString(m.MangaID) {
		return fmt.Errorf("invalid manga plus ID")
	}

	return nil
}

func (m *mangaplus) GetManga(ctx context.Context) (domain.Manga, error) {
	params := url.Values{
		"title_id": []string{m.MangaID},
	}

	path, err := url.JoinPath(mangaplusURL, "title_detailV3")
	if err != nil {
		return domain.Manga{}, fmt.Errorf("building URL: %w", err)
	}

	u, err := url.Parse(path)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("parsing URL %s: %w", path, err)
	}

	u.RawQuery = params.Encode()

	protoResp, err := m.getProtoResponse(ctx, u.String())
	if err != nil {
		return domain.Manga{}, fmt.Errorf("getting protobuf response from %s: %w", u.String(), err)
	}

	chaptersGroup := protoResp.GetSuccess().GetTitleDetailView().GetChapterListGroup()
	c := make(map[float32]domain.Chapter)

	for _, chapters := range chaptersGroup {
		err := m.addChapters(c, chapters.GetFirstChapterList(), chapters.GetLastChapterList())
		if err != nil {
			return domain.Manga{}, fmt.Errorf("adding chapters to chapter map: %w", err)
		}
	}

	title := protoResp.GetSuccess().GetTitleDetailView().GetTitle().GetName()
	if len(title) == 0 {
		return domain.Manga{}, fmt.Errorf("getting manga for ID %s", m.MangaID)
	}

	return domain.Manga{
		Title:    sanitize.Filename(title),
		Chapters: c,
	}, nil
}

func (m *mangaplus) GetChapters(_ context.Context, _ domain.Manga) error {
	return nil
}

func (m *mangaplus) GetImageURLs(ctx context.Context, chapter *domain.Chapter) error {
	params := url.Values{
		"chapter_id":  []string{chapter.ID},
		"split":       []string{"yes"},
		"img_quality": []string{"super_high"},
	}

	path, err := url.JoinPath(mangaplusURL, "manga_viewer")
	if err != nil {
		return fmt.Errorf("building URL: %w", err)
	}

	u, err := url.Parse(path)
	if err != nil {
		return fmt.Errorf("parsing URL %s: %w", path, err)
	}

	u.RawQuery = params.Encode()

	protoResp, err := m.getProtoResponse(ctx, u.String())
	if err != nil {
		return fmt.Errorf("getting protobuf response: %w", err)
	}

	var imageInfos []domain.ImageInfo

	for _, page := range protoResp.GetSuccess().GetMangaViewer().GetPages() {
		if page.GetMangaPage() != nil {
			imageInfos = append(imageInfos, domain.ImageInfo{
				ImageURL:      page.GetMangaPage().GetImageUrl(),
				EncryptionKey: page.GetMangaPage().GetEncryptionKey(),
			})
		}
	}

	if len(imageInfos) == 0 {
		return fmt.Errorf("getting image URLs for chapter ID %s", chapter.ID)
	}

	chapter.ImageInfo = imageInfos
	return nil
}

func (m *mangaplus) getProtoResponse(ctx context.Context, path string) (*protobuf.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return &protobuf.Response{}, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", "mangarr")

	var protoResp protobuf.Response

	retryErr := retry.Do(func() error {
		resp, err := sharedhttp.ExecRequest(*m.Client, req)
		if err != nil {
			return fmt.Errorf("executing request %s: %w", req.URL, err)
		}

		body, err := io.ReadAll(bufio.NewReader(resp.Body))
		if err != nil {
			return fmt.Errorf("reading response body: %w", err)
		}

		if err := proto.Unmarshal(body, &protoResp); err != nil {
			return retry.Unrecoverable(fmt.Errorf("unmarshalling response body: %w", err))
		}

		return nil
	},
		retry.Delay(time.Second*3),
		retry.Attempts(3),
		retry.MaxJitter(time.Second*1),
	)
	if retryErr != nil {
		return &protobuf.Response{}, fmt.Errorf("executing request %s: %w", req.URL, retryErr)
	}

	return &protoResp, nil
}

func (m *mangaplus) addChapters(chapters map[float32]domain.Chapter, chapterLists ...[]*protobuf.Chapter) error {
	for _, chapterList := range chapterLists {
		for _, chapter := range chapterList {
			chapterName := chapter.GetName()

			// logic used to skip extra chapters named "ex"
			if !strings.ContainsAny(chapterName, "0123456789") {
				continue
			}

			name := strings.Trim(chapterName, "#")

			number, err := strconv.ParseFloat(name, 32)
			if err != nil {
				return fmt.Errorf("parsing chapter number from %s: %w", name, err)
			}

			chapters[float32(number)] = domain.Chapter{
				ID:     fmt.Sprintf("%d", chapter.GetChapterId()),
				Number: float32(number),
				Title:  chapter.GetSubTitle(),
			}
		}
	}

	return nil
}
