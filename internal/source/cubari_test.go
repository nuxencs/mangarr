package source

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mangarr/internal/domain"

	"github.com/stretchr/testify/require"
)

func newTestCubari(mangaURL, baseURL string) *cubari {
	return &cubari{
		MangaURL: mangaURL,
		GroupID:  "The Koi Pond",
		BaseURL:  baseURL,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func TestCubariGetMangaProxyChapter(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gist":
			fmt.Fprint(w, `{
				"title": "One Punch Man",
				"chapters": {
					"235": {"title": "Chapter 235: Inline", "groups": {"The Koi Pond": ["https://cdn.example.com/a.png"]}},
					"236": {"title": "Chapter 236: Part-Time Bro 2", "groups": {"The Koi Pond": "/proxy/api/imgchest/chapter/a846bgrj2yx"}}
				}
			}`)
		case "/proxy/api/imgchest/chapter/a846bgrj2yx":
			fmt.Fprint(w, `["https://cdn.imgchest.com/files/1cac361a319f.png", "https://cdn.imgchest.com/files/512b2631f5df.png"]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	src := newTestCubari(server.URL+"/gist", server.URL)

	manga, err := src.GetManga(t.Context())
	require.NoError(t, err)

	chapter, ok := manga.Chapters[domain.ChapterNumber{Whole: 236}]
	require.True(t, ok, "chapter 236 missing")
	require.Equal(t, "Part-Time Bro 2", chapter.Title)
	require.Equal(t, []domain.ImageInfo{
		{ImageURL: "https://cdn.imgchest.com/files/1cac361a319f.png"},
		{ImageURL: "https://cdn.imgchest.com/files/512b2631f5df.png"},
	}, chapter.ImageInfo)

	inline, ok := manga.Chapters[domain.ChapterNumber{Whole: 235}]
	require.True(t, ok, "chapter 235 missing")
	require.Equal(t, []domain.ImageInfo{{ImageURL: "https://cdn.example.com/a.png"}}, inline.ImageInfo)
}
