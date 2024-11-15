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
	return "MangaPlus"
}

func (m *mangaplus) ValidateInput() error {
	if len(m.MangaID) == 0 {
		return fmt.Errorf("mangaplus manga id is required")
	}

	if !mangaplusID.MatchString(m.MangaID) {
		return fmt.Errorf("invalid mangaplus id")
	}

	return nil
}

func (m *mangaplus) GetManga(ctx context.Context) (domain.Manga, error) {
	params := url.Values{
		"title_id": []string{m.MangaID},
	}

	path, err := url.JoinPath(mangaplusURL, "title_detailV3")
	if err != nil {
		return domain.Manga{}, err
	}

	u, err := url.Parse(path)
	if err != nil {
		return domain.Manga{}, err
	}

	u.RawQuery = params.Encode()

	protoResp, err := m.getProtoResponse(ctx, u.String())
	if err != nil {
		return domain.Manga{}, err
	}

	chaptersGroup := protoResp.GetSuccess().GetTitleDetailView().GetChapterListGroup()

	c := make(map[float32]domain.Chapter)

	for _, chapters := range chaptersGroup {
		err := m.addChapters(c, chapters.GetFirstChapterList(), chapters.GetLastChapterList())
		if err != nil {
			return domain.Manga{}, err
		}
	}

	title := protoResp.GetSuccess().GetTitleDetailView().GetTitle().GetName()
	if len(title) == 0 {
		return domain.Manga{}, fmt.Errorf("failed to get manga for id: %s", m.MangaID)
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
		return err
	}

	u, err := url.Parse(path)
	if err != nil {
		return err
	}

	u.RawQuery = params.Encode()

	protoResp, err := m.getProtoResponse(ctx, u.String())
	if err != nil {
		return err
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
		return fmt.Errorf("failed to get image urls for chapter id: %s", chapter.ID)
	}

	chapter.ImageInfo = imageInfos
	return nil
}

func (m *mangaplus) getProtoResponse(ctx context.Context, path string) (*protobuf.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return &protobuf.Response{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "mangarr")

	var protoResp protobuf.Response

	retryErr := retry.Do(func() error {
		resp, err := sharedhttp.ExecRequest(*m.Client, req)
		if err != nil {
			return err
		}

		body, err := io.ReadAll(bufio.NewReader(resp.Body))
		if err != nil {
			return err
		}

		if err := proto.Unmarshal(body, &protoResp); err != nil {
			return retry.Unrecoverable(err)
		}

		return nil
	},
		retry.Delay(time.Second*3),
		retry.Attempts(3),
		retry.MaxJitter(time.Second*1),
	)

	return &protoResp, retryErr
}

func (m *mangaplus) addChapters(chapters map[float32]domain.Chapter, chapterLists ...[]*protobuf.Chapter) error {
	for _, chapterList := range chapterLists {
		for _, chapter := range chapterList {
			name := strings.Trim(chapter.GetName(), "#")
			number, err := strconv.ParseFloat(name, 32)
			if err != nil {
				return err
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
