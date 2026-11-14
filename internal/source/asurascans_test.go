package source

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// Since 2026-11-10, Cloudflare challenges every asurascans.com page, but api.asurascans.com stays public.
func TestAsurascansReadsAPIWhenSitePagesAreChallenged(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/series/solo-max-level-newbie-7f873ca6":
			fmt.Fprint(w, `{"series":{"id":1896,"slug":"solo-max-level-newbie","title":"Solo Max-Level Newbie","public_url":"/comics/solo-max-level-newbie-f212c582"},"recommended_series":[]}`)
		case "/api/series/solo-max-level-newbie-7f873ca6/chapters":
			fmt.Fprint(w, `{"data":[
				{"id":262769,"series_id":1896,"number":284,"title":"Passage of Trials","slug":"chapter-284","page_count":2,"is_premium":false,"series_slug":"solo-max-level-newbie"},
				{"id":143802,"series_id":1896,"number":0,"title":"","slug":"chapter-0","page_count":2,"is_premium":false,"series_slug":"solo-max-level-newbie"}
			]}`)
		case "/api/series/solo-max-level-newbie/chapters/chapter-284":
			fmt.Fprint(w, `{"data":{"access_gate":"","chapter":{"number":284,"slug":"chapter-284","pages":[
				{"url":"https://cdn.asurascans.com/asura-images/chapters/solo-max-level-newbie/284/be6022.webp?v=1794162325","width":1532,"height":1024},
				{"url":"https://cdn.asurascans.com/asura-images/chapters/solo-max-level-newbie/284/0c11aa.webp?v=1794162325","width":1532,"height":1024}
			]}}}`)
		default:
			// The challenge page that real asurascans.com pages return.
			w.Header().Set("Cf-Mitigated", "challenge")
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	defer server.Close()

	src := newTestAsurascans(server.URL+"/comics/solo-max-level-newbie-7f873ca6", server.URL)

	manga, err := src.Discover(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Solo Max-Level Newbie", manga.Title)
	require.Len(t, manga.Chapters, 2)

	chapter284, ok := manga.Chapters[mustChapterNumber("284")]
	require.True(t, ok)
	require.Equal(t, "Passage of Trials", chapter284.Title)
	require.Equal(t, server.URL+"/api/series/solo-max-level-newbie/chapters/chapter-284", chapter284.URL)

	pages, err := src.Pages(t.Context(), chapter284)
	require.NoError(t, err)
	require.Len(t, pages, 2)
	require.Equal(t, "https://cdn.asurascans.com/asura-images/chapters/solo-max-level-newbie/284/be6022.webp?v=1794162325", pages[0].ImageURL)
	require.Equal(t, "https://cdn.asurascans.com/asura-images/chapters/solo-max-level-newbie/284/0c11aa.webp?v=1794162325", pages[1].ImageURL)
}

func TestAsurascansDiscoverSkipsPremiumChapters(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/series/pick-me-up-infinite-gacha-f6174291":
			fmt.Fprint(w, `{"series":{"slug":"pick-me-up-infinite-gacha","title":"Pick Me Up, Infinite Gacha"},"recommended_series":[]}`)
		case "/api/series/pick-me-up-infinite-gacha-f6174291/chapters":
			fmt.Fprint(w, `{"data":[
				{"number":215,"title":"Premium Chapter","slug":"chapter-215","is_premium":true,"early_access_until":"2026-08-19T00:00:00Z","series_slug":"pick-me-up-infinite-gacha"},
				{"number":214,"title":"Public Chapter","slug":"chapter-214","is_premium":false,"early_access_until":null,"series_slug":"pick-me-up-infinite-gacha"}
			]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	src := newTestAsurascans(server.URL+"/comics/pick-me-up-infinite-gacha-f6174291", server.URL)

	manga, err := src.Discover(t.Context())
	require.NoError(t, err)
	require.Len(t, manga.Chapters, 1)

	_, hasPremium := manga.Chapters[mustChapterNumber("215")]
	require.False(t, hasPremium, "premium chapter 215 should be filtered out")

	chapter214, ok := manga.Chapters[mustChapterNumber("214")]
	require.True(t, ok)
	require.Equal(t, "Public Chapter", chapter214.Title)
}

// Since 2026-11-10, some API origins wrap the series response in "data", and Cloudflare caches either shape for up to 5 minutes.
func TestAsurascansDiscoverReadsBothSeriesResponseShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		series string
	}{
		{
			name:   "unwrapped",
			series: `{"series":{"slug":"star-embracing-swordmaster","title":"Star-Embracing Swordmaster"},"recommended_series":[]}`,
		},
		{
			name:   "data wrapper",
			series: `{"data":{"linked_novel":{"id":5,"title":"Star-Embracing Swordmaster"},"recommended_series":[],"series":{"slug":"star-embracing-swordmaster","title":"Star-Embracing Swordmaster"}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/series/star-embracing-swordmaster-f212c582":
					fmt.Fprint(w, tt.series)
				case "/api/series/star-embracing-swordmaster-f212c582/chapters":
					fmt.Fprint(w, `{"data":[{"number":146,"title":"","slug":"chapter-146","is_premium":false,"series_slug":"star-embracing-swordmaster"}]}`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			src := newTestAsurascans(server.URL+"/comics/star-embracing-swordmaster-f212c582", server.URL)

			manga, err := src.Discover(t.Context())
			require.NoError(t, err)
			require.Equal(t, "Star-Embracing Swordmaster", manga.Title)
			require.Len(t, manga.Chapters, 1)
		})
	}
}

func newTestAsurascans(mangaURL, apiURL string) *asurascans {
	return &asurascans{
		MangaURL: mangaURL,
		APIURL:   apiURL,
		Client:   *http.DefaultClient,
	}
}
