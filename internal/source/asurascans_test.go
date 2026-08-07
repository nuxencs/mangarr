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

func TestAsurascansDiscoverParsesCurrentSeriesURL(t *testing.T) {
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
						<astro-island component-url="/_astro/ChapterListReact.xxx.js" props="{&quot;chapters&quot;:[1,[[0,{&quot;number&quot;:[0,249],&quot;is_locked&quot;:[0,false]}],[0,{&quot;number&quot;:[0,248],&quot;is_locked&quot;:[0,false]}]]]}" ssr>
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
						</astro-island>
					</body>
				</html>
			`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	src := newTestAsurascans(server.URL+currentSeries, server.URL)

	manga, err := src.Discover(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Solo Max-Level Newbie", manga.Title)
	require.Equal(t, server.URL+currentSeries, manga.URL)
	require.Len(t, manga.Chapters, 2)

	ch249, ok := manga.Chapters[mustChapterNumber("249")]
	require.True(t, ok)
	require.Equal(t, "Tangled Threads (2)", ch249.Title)
	require.Equal(t, server.URL+currentChapter1, ch249.URL)

	ch248, ok := manga.Chapters[mustChapterNumber("248")]
	require.True(t, ok)
	require.Equal(t, "Tangled Threads (1)", ch248.Title)
	require.Equal(t, server.URL+currentChapter2, ch248.URL)
}

func TestAsurascansDiscoverSkipsLockedChapters(t *testing.T) {
	t.Parallel()

	const (
		currentSeries  = "/comics/pick-me-up-infinite-gacha-f6174291"
		lockedChapter  = "/comics/pick-me-up-infinite-gacha-f6174291/chapter/194"
		regularChapter = "/comics/pick-me-up-infinite-gacha-f6174291/chapter/193"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case currentSeries:
			fmt.Fprint(w, `
				<html>
					<body>
						<h1>Pick Me Up, Infinite Gacha</h1>
						<astro-island component-url="/_astro/ChapterListReact.xxx.js" props="{&quot;chapters&quot;:[1,[[0,{&quot;number&quot;:[0,194],&quot;is_locked&quot;:[0,true]}],[0,{&quot;number&quot;:[0,193],&quot;is_locked&quot;:[0,false]}]]]}" ssr>
						<a href="`+lockedChapter+`">
							<span>Chapter 194</span>
							<span>Early Access Title</span>
							<span>1 hour ago</span>
						</a>
						<a href="`+regularChapter+`">
							<span>Chapter 193</span>
							<span>Regular Title</span>
							<span>last week</span>
						</a>
						</astro-island>
					</body>
				</html>
			`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	src := newTestAsurascans(server.URL+currentSeries, server.URL)

	manga, err := src.Discover(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Pick Me Up, Infinite Gacha", manga.Title)
	require.Len(t, manga.Chapters, 1)

	_, hasLocked := manga.Chapters[mustChapterNumber("194")]
	require.False(t, hasLocked, "locked chapter 194 should be filtered out")

	ch193, ok := manga.Chapters[mustChapterNumber("193")]
	require.True(t, ok)
	require.Equal(t, "Regular Title", ch193.Title)
}

func TestAsurascansPagesExtractsChapterAssets(t *testing.T) {
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
		Number: mustChapterNumber("249"),
	}

	pages, err := src.Pages(t.Context(), chapter)
	require.NoError(t, err)
	require.Len(t, pages, 2)
	require.Equal(t, "https://cdn.asurascans.com/asura-images/chapters/solo-max-level-newbie/249/001.webp", pages[0].ImageURL)
	require.Equal(t, "https://cdn.asurascans.com/asura-images/chapters/solo-max-level-newbie/249/002.webp", pages[1].ImageURL)
}

func newTestAsurascans(mangaURL, baseURL string) *asurascans {
	collector := colly.NewCollector(
		colly.AllowURLRevisit(),
	)
	extensions.RandomUserAgent(collector)
	collector.SetRequestTimeout(10 * time.Second)

	return &asurascans{
		MangaURL:  mangaURL,
		Collector: collector,
		BaseURL:   baseURL,
		Client:    *http.DefaultClient,
	}
}
