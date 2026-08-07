package source

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mangarr/internal/domain"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
	"github.com/stretchr/testify/require"
)

func TestWeebCentralGetImageURLsExtractsChapterAssets(t *testing.T) {
	t.Parallel()

	const chapterPath = "/chapters/01TESTCHAPTER"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case chapterPath + "/images":
			require.Equal(t, "False", r.URL.Query().Get("is_prev"))
			require.Equal(t, "1", r.URL.Query().Get("current_page"))
			require.Equal(t, "long_strip", r.URL.Query().Get("reading_style"))

			fmt.Fprint(w, `
				<section>
					<img src="https://cdn.weebcentral.test/manga/chapter-001.png" />
					<img src="/media/chapter-002.png" />
					<img src="https://cdn.weebcentral.test/manga/chapter-001.png" />
				</section>
			`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	src := newTestWeebCentral(server.URL+"/series/series-id", server.URL)

	chapter := domain.Chapter{
		URL:    server.URL + chapterPath + "?foo=bar#reader",
		Number: domain.MustParseChapterNumber("340.2"),
	}

	err := src.GetImageURLs(t.Context(), &chapter)
	require.NoError(t, err)
	require.Len(t, chapter.ImageInfo, 2)
	require.Equal(t, "https://cdn.weebcentral.test/manga/chapter-001.png", chapter.ImageInfo[0].ImageURL)
	require.Equal(t, server.URL+"/media/chapter-002.png", chapter.ImageInfo[1].ImageURL)
}

func TestWeebCentralResolvesRelativeChapterURLs(t *testing.T) {
	t.Parallel()

	const chapterPath = "/chapters/01RELATIVECHAPTER"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/series/series-id/full-chapter-list":
			fmt.Fprintf(w, `<a class="flex" href="%s"><span class="grow">Chapter 356</span></a>`, chapterPath)
		case chapterPath + "/images":
			fmt.Fprint(w, `<img src="/media/chapter-356.png" />`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	src := newTestWeebCentral(server.URL+"/series/series-id", server.URL)
	chapterNumber := domain.MustParseChapterNumber("356")
	manga := domain.Manga{
		Title:    "Blue Lock",
		Chapters: make(map[domain.ChapterNumber]domain.Chapter),
	}

	require.NoError(t, src.GetChapters(t.Context(), manga))
	chapter := manga.Chapters[chapterNumber]
	require.NoError(t, src.GetImageURLs(t.Context(), &chapter))
	require.Equal(t, server.URL+"/media/chapter-356.png", chapter.ImageInfo[0].ImageURL)
}

func TestWeebCentralGetImageURLsErrorsWhenFragmentHasNoImages(t *testing.T) {
	t.Parallel()

	const chapterPath = "/chapters/01EMPTYCHAPTER"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case chapterPath + "/images":
			fmt.Fprint(w, `<section><p>No pages.</p></section>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	src := newTestWeebCentral(server.URL+"/series/series-id", server.URL)

	chapter := domain.Chapter{
		URL:    server.URL + chapterPath,
		Number: domain.MustParseChapterNumber("1"),
	}

	err := src.GetImageURLs(t.Context(), &chapter)
	require.EqualError(t, err, "getting image URLs for chapter 1")
}

func TestWeebCentralRejectsPrefixHostSpoofing(t *testing.T) {
	source := NewWeebCentral("https://weebcentral.com.example/series/fixture")
	if err := source.ValidateInput(); err == nil {
		t.Fatal("expected spoofed host to fail validation")
	}
}

func newTestWeebCentral(mangaURL, baseURL string) *weebcentral {
	collector := colly.NewCollector(
		colly.AllowURLRevisit(),
	)
	extensions.RandomUserAgent(collector)
	collector.SetRequestTimeout(10 * time.Second)

	return &weebcentral{
		MangaURL:  mangaURL,
		Collector: collector,
		BaseURL:   baseURL,
		Client: http.Client{
			Timeout: 10 * time.Second,
		},
	}
}
