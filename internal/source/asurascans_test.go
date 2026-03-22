package source

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mangarr/internal/domain"

	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
	"github.com/stretchr/testify/require"
)

func TestAsurascansGetMangaParsesCurrentSeriesURL(t *testing.T) {
	t.Parallel()

	const (
		currentSeries   = "/comics/solo-max-level-newbie-7f873ca6"
		currentChapter1 = "/comics/solo-max-level-newbie-7f873ca6/chapter/249"
		currentChapter2 = "/comics/solo-max-level-newbie-7f873ca6/chapter/248"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case currentSeries:
			fmt.Fprint(w, `
				<html>
					<body>
						<h1>Solo Max-Level Newbie</h1>
						<a href="`+currentChapter1+`">
							<span>Chapter 249</span>
							<span>Tangled Threads (2)</span>
							<span>last week</span>
						</a>
						<a href="`+currentChapter2+`">
							<span>Chapter 248</span>
							<span>Tangled Threads (1)</span>
							<span>two weeks ago</span>
						</a>
					</body>
				</html>
			`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	src := newTestAsurascans(server.URL+currentSeries, server.URL)

	manga, err := src.GetManga(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Solo Max-Level Newbie", manga.Title)
	require.Equal(t, server.URL+currentSeries, manga.URL)
	require.Len(t, manga.Chapters, 2)

	ch249, ok := manga.Chapters[domain.MustParseChapterNumber("249")]
	require.True(t, ok)
	require.Equal(t, "Tangled Threads (2)", ch249.Title)
	require.Equal(t, server.URL+currentChapter1, ch249.URL)

	ch248, ok := manga.Chapters[domain.MustParseChapterNumber("248")]
	require.True(t, ok)
	require.Equal(t, "Tangled Threads (1)", ch248.Title)
	require.Equal(t, server.URL+currentChapter2, ch248.URL)
}

func TestAsurascansGetImageURLsExtractsChapterAssets(t *testing.T) {
	t.Parallel()

	const chapterPath = "/comics/solo-max-level-newbie-7f873ca6/chapter/249"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case chapterPath:
			fmt.Fprint(w, `
				<html>
					<body>
						<script>
							window.__DATA__ = {
								pages: [
									"https://cdn.asurascans.com/asura-images/chapters/solo-max-level-newbie/249/001.webp",
									"https://cdn.asurascans.com/asura-images/chapters/solo-max-level-newbie/249/002.webp",
									"https://cdn.asurascans.com/asura-images/chapters/solo-max-level-newbie/249/001.webp"
								]
							}
						</script>
					</body>
				</html>
			`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	src := newTestAsurascans(server.URL+chapterPath, server.URL)

	chapter := domain.Chapter{
		URL:    server.URL + chapterPath,
		Number: domain.MustParseChapterNumber("249"),
	}

	err := src.GetImageURLs(t.Context(), &chapter)
	require.NoError(t, err)
	require.Len(t, chapter.ImageInfo, 2)
	require.Equal(t, "https://cdn.asurascans.com/asura-images/chapters/solo-max-level-newbie/249/001.webp", chapter.ImageInfo[0].ImageURL)
	require.Equal(t, "https://cdn.asurascans.com/asura-images/chapters/solo-max-level-newbie/249/002.webp", chapter.ImageInfo[1].ImageURL)
}

func newTestAsurascans(mangaURL, baseURL string) *asurascans {
	collector := colly.NewCollector(
		colly.AllowURLRevisit(),
	)
	extensions.RandomUserAgent(collector)
	collector.SetRequestTimeout(10 * time.Second)

	return &asurascans{
		MangaURL:  mangaURL,
		Collector: *collector,
		BaseURL:   baseURL,
		Client:    *http.DefaultClient,
	}
}
