package source

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/protobuf"
	"mangarr/internal/sanitize"
	"mangarr/internal/sharedhttp"

	"github.com/avast/retry-go"
	"google.golang.org/protobuf/proto"
)

const (
	mangaplusURL          = "https://jumpg-api.tokyo-cdn.com/api"
	mangaplusAppVersion   = "237"
	mangaplusOSVersion    = "35"
	mangaplusSecuritySalt = "4Kin9vGg" // Static salt used by the Manga Plus Android client for device registration.
)

var mangaplusID = regexp.MustCompile(`^[1-9][0-9][0-9][0-9][0-9][0-9]$`)

type mangaplus struct {
	MangaID string
	Client  *http.Client
	baseURL string
	secret  string
}

func NewMangaPlus(mangaID string) domain.Source {
	client := &http.Client{
		Timeout:   60 * time.Second,
		Transport: sharedhttp.Transport,
	}

	return &mangaplus{
		MangaID: mangaID,
		Client:  client,
		baseURL: mangaplusURL,
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
	if err := m.ensureRegistered(ctx); err != nil {
		return domain.Manga{}, fmt.Errorf("registering Manga Plus device: %w", err)
	}

	params := url.Values{
		"title_id": []string{m.MangaID},
		"lang":     []string{"eng"},
		"clang":    []string{"eng"},
	}

	path, err := url.JoinPath(m.baseURL, "title_detailV3")
	if err != nil {
		return domain.Manga{}, fmt.Errorf("building URL: %w", err)
	}

	u, err := url.Parse(path)
	if err != nil {
		return domain.Manga{}, fmt.Errorf("parsing URL %s: %w", path, err)
	}

	u.RawQuery = params.Encode()

	protoResp, err := m.getProtoResponse(ctx, http.MethodGet, u.String())
	if err != nil {
		return domain.Manga{}, fmt.Errorf("getting protobuf response from %s: %w", u.String(), err)
	}

	c := make(map[domain.ChapterNumber]domain.Chapter)
	titleDetail := protoResp.GetSuccess().GetTitleDetailView()

	title := titleDetail.GetTitle().GetName()
	if len(title) == 0 {
		return domain.Manga{}, fmt.Errorf("getting manga for ID %s", m.MangaID)
	}

	if err := m.addTitleDetailChapters(c, titleDetail); err != nil {
		return domain.Manga{}, fmt.Errorf("adding chapters for manga %s (%s): %w", title, m.MangaID, err)
	}

	if len(c) == 0 {
		return domain.Manga{}, fmt.Errorf("getting chapters for manga %s (%s)", title, m.MangaID)
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
	if err := m.ensureRegistered(ctx); err != nil {
		return fmt.Errorf("registering Manga Plus device: %w", err)
	}

	params := url.Values{
		"chapter_id":  []string{chapter.ID},
		"split":       []string{"yes"},
		"img_quality": []string{"super_high"},
		"viewer_mode": []string{"vertical"},
		"clang":       []string{"eng"},
	}

	path, err := url.JoinPath(m.baseURL, "manga_viewer")
	if err != nil {
		return fmt.Errorf("building URL: %w", err)
	}

	u, err := url.Parse(path)
	if err != nil {
		return fmt.Errorf("parsing URL %s: %w", path, err)
	}

	u.RawQuery = params.Encode()

	protoResp, err := m.getProtoResponse(ctx, http.MethodGet, u.String())
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

func (m *mangaplus) ensureRegistered(ctx context.Context) error {
	if m.secret != "" {
		return nil
	}

	deviceToken := md5Hex(mangaplusDeviceID())
	securityKey := md5Hex(deviceToken + mangaplusSecuritySalt)

	params := url.Values{
		"device_token": []string{deviceToken},
		"security_key": []string{securityKey},
	}

	path, err := url.JoinPath(m.baseURL, "register")
	if err != nil {
		return fmt.Errorf("building URL: %w", err)
	}

	u, err := url.Parse(path)
	if err != nil {
		return fmt.Errorf("parsing URL %s: %w", path, err)
	}

	u.RawQuery = params.Encode()

	protoResp, err := m.getProtoResponse(ctx, http.MethodPut, u.String())
	if err != nil {
		return err
	}

	secret, err := m.getRegistrationSecret(protoResp.GetSuccess())
	if err != nil {
		return err
	}

	m.secret = secret
	return nil
}

func (m *mangaplus) getProtoResponse(ctx context.Context, method string, path string) (*protobuf.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, path, nil)
	if err != nil {
		return &protobuf.Response{}, fmt.Errorf("creating request: %w", err)
	}

	query := req.URL.Query()
	query.Set("os", "android")
	query.Set("os_ver", mangaplusOSVersion)
	query.Set("app_ver", mangaplusAppVersion)
	if m.secret != "" {
		query.Set("secret", m.secret)
	}
	req.URL.RawQuery = query.Encode()

	req.Header.Set("User-Agent", "okhttp/4.12.0")

	var protoResp protobuf.Response

	retryErr := retry.Do(func() error {
		resp, err := sharedhttp.ExecRequest(*m.Client, req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(bufio.NewReader(resp.Body))
		if err != nil {
			return fmt.Errorf("reading response body: %w", err)
		}

		if err := proto.Unmarshal(body, &protoResp); err != nil {
			return retry.Unrecoverable(fmt.Errorf("unmarshalling response body: %w", err))
		}

		if err := m.responseError(&protoResp); err != nil {
			return retry.Unrecoverable(err)
		}

		return nil
	},
		sharedhttp.RetryOptions(ctx)...,
	)
	if retryErr != nil {
		return &protobuf.Response{}, fmt.Errorf("executing request %s: %w", req.URL, retryErr)
	}

	return &protoResp, nil
}

func (m *mangaplus) responseError(response *protobuf.Response) error {
	if response.GetError() == nil {
		return nil
	}

	subject := response.GetError().GetEnglishPopup().GetSubject()
	body := response.GetError().GetEnglishPopup().GetBody()
	if subject == "" {
		return fmt.Errorf("Manga Plus API error: %s", body)
	}

	if body == "" {
		return fmt.Errorf("Manga Plus API error: %s", subject)
	}

	return fmt.Errorf("Manga Plus API error: %s: %s", subject, body)
}

func (m *mangaplus) getRegistrationSecret(success *protobuf.SuccessResult) (string, error) {
	if success == nil {
		return "", fmt.Errorf("registration response missing success result")
	}

	secret := success.GetRegisterationData().GetDeviceSecret()
	if secret == "" {
		return "", fmt.Errorf("registration response missing device secret")
	}

	return secret, nil
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func mangaplusDeviceID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown-host"
	}

	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}
	if username == "" {
		username = "unknown-user"
	}

	return "mangarr:" + hostname + ":" + username
}

func (m *mangaplus) addTitleDetailChapters(chapters map[domain.ChapterNumber]domain.Chapter, detail *protobuf.TitleDetailView) error {
	if detail == nil {
		return fmt.Errorf("title detail view is missing")
	}

	if err := m.addChapters(chapters, detail.GetFirstChapterList(), detail.GetLastChapterList()); err != nil {
		return err
	}

	for _, chapterGroup := range detail.GetChapterListGroup() {
		if err := m.addChapters(chapters, chapterGroup.GetFirstChapterList(), chapterGroup.GetLastChapterList()); err != nil {
			return err
		}
	}

	return m.addChapters(chapters, detail.GetChapterListV2())
}

func (m *mangaplus) addChapters(chapters map[domain.ChapterNumber]domain.Chapter, chapterLists ...[]*protobuf.Chapter) error {
	for _, chapterList := range chapterLists {
		for _, chapter := range chapterList {
			chapterName := chapter.GetName()

			// logic used to skip extra chapters named "ex"
			if !strings.ContainsAny(chapterName, "0123456789") {
				continue
			}

			name := strings.Trim(chapterName, "#")

			number, err := domain.ParseChapterNumber(name)
			if err != nil {
				return fmt.Errorf("parsing chapter number from %s: %w", name, err)
			}

			chapters[number] = domain.Chapter{
				ID:     fmt.Sprintf("%d", chapter.GetChapterId()),
				Number: number,
				Title:  chapter.GetSubTitle(),
			}
		}
	}

	return nil
}
