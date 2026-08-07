package source

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mangarr/internal/domain"

	"github.com/stretchr/testify/require"
)

func TestAtsumaruValidateInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:  "valid URL",
			input: "https://atsu.moe/manga/Q5Mqy",
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: "atsumaru manga URL is required",
		},
		{
			name:    "raw ID rejected",
			input:   "Q5Mqy",
			wantErr: "the URL for Atsumaru must start with https://atsu.moe",
		},
		{
			name:    "wrong host",
			input:   "https://example.com/manga/Q5Mqy",
			wantErr: "the URL for Atsumaru must start with https://atsu.moe",
		},
		{
			name:    "wrong path",
			input:   "https://atsu.moe/title/Q5Mqy",
			wantErr: "invalid Atsumaru manga URL path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			src := NewAtsumaru(tt.input, "scan-1")
			err := src.ValidateInput()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestAtsumaruValidateInputRequiresScanID(t *testing.T) {
	t.Parallel()

	src := NewAtsumaru("https://atsu.moe/manga/Q5Mqy", "")

	err := src.ValidateInput()
	require.EqualError(t, err, "atsumaru scan ID is required")
}

func TestAtsumaruDiscover(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/manga/info", r.URL.Path)
		require.Equal(t, "Q5Mqy", r.URL.Query().Get("mangaId"))

		fmt.Fprint(w, `{
			"id": "Q5Mqy",
			"title": "Kagurabachi",
			"forceStrip": true,
			"chapters": [
				{"id": "chapter-0", "title": "Chapter 0", "number": 0, "scanId": "scan-1"},
				{"id": "chapter-7-1", "title": "Chapter 7.1", "number": 7.1, "scanId": "scan-1"},
				{"id": "chapter-7-1-other", "title": "Chapter 7.1", "number": 7.1, "scanId": "scan-2"}
			]
		}`)
	}))
	defer server.Close()

	src := newTestAtsumaru(server.URL + "/manga/Q5Mqy")

	manga, err := src.Discover(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Q5Mqy", manga.ID)
	require.Equal(t, server.URL+"/manga/Q5Mqy", manga.URL)
	require.Equal(t, "Kagurabachi", manga.Title)
	require.True(t, manga.IsManhwa)
	require.Len(t, manga.Chapters, 2)

	ch0, ok := manga.Chapters[mustChapterNumber("0")]
	require.True(t, ok)
	require.Equal(t, "chapter-0", ch0.ID)
	require.Equal(t, "Chapter 0", ch0.Title)

	ch71, ok := manga.Chapters[mustChapterNumber("7.1")]
	require.True(t, ok)
	require.Equal(t, "chapter-7-1", ch71.ID)
	require.Equal(t, "Chapter 7.1", ch71.Title)
}

func TestAtsumaruDiscoverErrorsWhenScanIDHasNoChapters(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{
			"id": "Q5Mqy",
			"title": "Kagurabachi",
			"chapters": [
				{"id": "chapter-1", "title": "Chapter 1", "number": 1, "scanId": "scan-2"}
			]
		}`)
	}))
	defer server.Close()

	src := newTestAtsumaru(server.URL + "/manga/Q5Mqy")

	_, err := src.Discover(t.Context())
	require.EqualError(t, err, "getting chapters for manga Kagurabachi")
}

func TestAtsumaruDiscoverErrorsWhenNoChapters(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"id":"Q5Mqy","title":"Kagurabachi","chapters":[]}`)
	}))
	defer server.Close()

	src := newTestAtsumaru(server.URL + "/manga/Q5Mqy")

	_, err := src.Discover(t.Context())
	require.EqualError(t, err, "getting chapters for manga Kagurabachi")
}

func TestAtsumaruPages(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/read/chapter", r.URL.Path)
		require.Equal(t, "Q5Mqy", r.URL.Query().Get("mangaId"))
		require.Equal(t, "yzmwX4", r.URL.Query().Get("chapterId"))

		fmt.Fprint(w, `{
			"readChapter": {
				"id": "yzmwX4",
				"title": "Chapter 121",
				"pages": [
					{"image": "/static/pages/yzmwX4/0.webp"},
					{"image": "https://cdn.atsu.test/static/pages/yzmwX4/1.webp"}
				]
			}
		}`)
	}))
	defer server.Close()

	src := newTestAtsumaru(server.URL + "/manga/Q5Mqy")
	chapter := domain.Chapter{
		ID:     "yzmwX4",
		Number: mustChapterNumber("121"),
	}

	pages, err := src.Pages(t.Context(), chapter)
	require.NoError(t, err)
	require.Len(t, pages, 2)
	require.Equal(t, server.URL+"/static/pages/yzmwX4/0.webp", pages[0].ImageURL)
	require.Equal(t, "https://cdn.atsu.test/static/pages/yzmwX4/1.webp", pages[1].ImageURL)
}

func TestAtsumaruPagesErrorsWhenNoPages(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"readChapter":{"id":"yzmwX4","title":"Chapter 121","pages":[]}}`)
	}))
	defer server.Close()

	src := newTestAtsumaru(server.URL + "/manga/Q5Mqy")
	chapter := domain.Chapter{
		ID:     "yzmwX4",
		Number: mustChapterNumber("121"),
	}

	_, err := src.Pages(t.Context(), chapter)
	require.EqualError(t, err, "getting image URLs for chapter ID yzmwX4")
}

func newTestAtsumaru(mangaURL string) *atsumaru {
	return &atsumaru{
		MangaURL: mangaURL,
		ScanID:   "scan-1",
		BaseURL:  strings.TrimSuffix(mangaURL, "/manga/Q5Mqy"),
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}
