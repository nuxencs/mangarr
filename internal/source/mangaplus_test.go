package source

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"mangarr/internal/domain"
	"mangarr/internal/protobuf"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestMangaPlusGetMangaRegistersAndUsesSecret(t *testing.T) {
	t.Parallel()

	var registered atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/register":
			require.Equal(t, http.MethodPut, r.Method)
			require.NotEmpty(t, r.URL.Query().Get("device_token"))
			require.NotEmpty(t, r.URL.Query().Get("security_key"))
			writeProtoResponse(t, w, &protobuf.Response{
				Success: &protobuf.SuccessResult{
					RegisterationData: &protobuf.RegistrationData{DeviceSecret: "registered-secret"},
				},
			})
			registered.Store(true)
		case "/api/title_detailV3":
			require.True(t, registered.Load())
			require.Equal(t, "registered-secret", r.URL.Query().Get("secret"))
			writeProtoResponse(t, w, &protobuf.Response{
				Success: &protobuf.SuccessResult{
					TitleDetailView: &protobuf.TitleDetailView{
						Title: &protobuf.Title{Name: "Registered Title"},
						ChapterListV2: []*protobuf.Chapter{
							{ChapterId: 1001, Name: "#001", SubTitle: "First"},
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	src := &mangaplus{
		MangaID: "100352",
		Client:  server.Client(),
		baseURL: server.URL + "/api",
	}

	manga, err := src.GetManga(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Registered Title", manga.Title)
	require.Equal(t, "registered-secret", src.secret)
}

func TestMangaPlusGetMangaParsesChapterListV2(t *testing.T) {
	t.Parallel()

	titleDetail := &protobuf.TitleDetailView{
		Title: &protobuf.Title{Name: "Hero Organization"},
		ChapterListV2: []*protobuf.Chapter{
			{ChapterId: 1022325, Name: "#001", SubTitle: "Chapter 1: THE ORDINARY AND THE GENIUS"},
			{ChapterId: 1022326, Name: "#002", SubTitle: "Chapter 2: HEROES"},
			{ChapterId: 1022327, Name: "ex", SubTitle: "Extra"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/title_detailV3", r.URL.Path)
		require.Equal(t, "100352", r.URL.Query().Get("title_id"))
		writeProtoResponse(t, w, &protobuf.Response{
			Success: &protobuf.SuccessResult{TitleDetailView: titleDetail},
		})
	}))
	defer server.Close()

	src := newTestMangaPlus("100352", server)

	manga, err := src.GetManga(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Hero Organization", manga.Title)
	require.Len(t, manga.Chapters, 2)

	chapter1, ok := manga.Chapters[domain.MustParseChapterNumber("1")]
	require.True(t, ok)
	require.Equal(t, "1022325", chapter1.ID)
	require.Equal(t, "Chapter 1: THE ORDINARY AND THE GENIUS", chapter1.Title)

	chapter2, ok := manga.Chapters[domain.MustParseChapterNumber("2")]
	require.True(t, ok)
	require.Equal(t, "1022326", chapter2.ID)
	require.Equal(t, "Chapter 2: HEROES", chapter2.Title)
}

func TestMangaPlusGetMangaKeepsLegacyChapterGroups(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeProtoResponse(t, w, &protobuf.Response{
			Success: &protobuf.SuccessResult{
				TitleDetailView: &protobuf.TitleDetailView{
					Title: &protobuf.Title{Name: "Legacy Title"},
					ChapterListGroup: []*protobuf.ChapterGroup{
						{
							FirstChapterList: []*protobuf.Chapter{
								{ChapterId: 1001, Name: "#001", SubTitle: "First"},
							},
							LastChapterList: []*protobuf.Chapter{
								{ChapterId: 1002, Name: "#010", SubTitle: "Latest"},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	src := newTestMangaPlus("100352", server)

	manga, err := src.GetManga(t.Context())
	require.NoError(t, err)
	require.Len(t, manga.Chapters, 2)
	require.Equal(t, "1001", manga.Chapters[domain.MustParseChapterNumber("1")].ID)
	require.Equal(t, "1002", manga.Chapters[domain.MustParseChapterNumber("10")].ID)
}

func TestMangaPlusGetImageURLsParsesViewerPages(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/manga_viewer", r.URL.Path)
		require.Equal(t, "1022325", r.URL.Query().Get("chapter_id"))
		require.Equal(t, "yes", r.URL.Query().Get("split"))
		require.Equal(t, "super_high", r.URL.Query().Get("img_quality"))
		writeProtoResponse(t, w, &protobuf.Response{
			Success: &protobuf.SuccessResult{
				MangaViewer: &protobuf.MangaViewer{
					Pages: []*protobuf.Page{
						{MangaPage: &protobuf.MangaPage{ImageUrl: "https://cdn.example/page-001.webp", EncryptionKey: "abc"}},
						{MangaPage: &protobuf.MangaPage{ImageUrl: "https://cdn.example/page-002.webp"}},
						{Advertisement: &protobuf.AdNetworkList{}},
					},
				},
			},
		})
	}))
	defer server.Close()

	src := newTestMangaPlus("100352", server)
	chapter := domain.Chapter{ID: "1022325"}

	err := src.GetImageURLs(t.Context(), &chapter)
	require.NoError(t, err)
	require.Equal(t, []domain.ImageInfo{
		{ImageURL: "https://cdn.example/page-001.webp", EncryptionKey: "abc"},
		{ImageURL: "https://cdn.example/page-002.webp"},
	}, chapter.ImageInfo)
}

func newTestMangaPlus(mangaID string, server *httptest.Server) *mangaplus {
	return &mangaplus{
		MangaID: mangaID,
		Client:  server.Client(),
		baseURL: server.URL + "/api",
		secret:  "test-secret",
	}
}

func writeProtoResponse(t *testing.T, w http.ResponseWriter, response *protobuf.Response) {
	t.Helper()

	responseBytes, err := proto.Marshal(response)
	require.NoError(t, err)

	w.WriteHeader(http.StatusOK)
	_, err = w.Write(responseBytes)
	require.NoError(t, err)
}
