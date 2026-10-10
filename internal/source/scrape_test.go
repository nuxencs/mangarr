package source

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"mangarr/internal/domain"
	"mangarr/internal/sharedhttp"

	"github.com/stretchr/testify/require"
)

// scrapingSources builds each HTML-scraping adapter against a test server.
var scrapingSources = []struct {
	name      string
	newSource func(serverURL string) domain.Source
}{
	{
		name: "weebcentral",
		newSource: func(serverURL string) domain.Source {
			src := NewWeebCentral(serverURL + "/series/fixture").(*weebcentral)
			src.BaseURL = serverURL
			return src
		},
	},
	{
		name: "asurascans",
		newSource: func(serverURL string) domain.Source {
			src := NewAsurascans(serverURL + "/comics/fixture").(*asurascans)
			src.BaseURL = serverURL
			return src
		},
	},
	{
		name: "flamecomics",
		newSource: func(serverURL string) domain.Source {
			src := NewFlamecomics(serverURL + "/series/fixture").(*flamecomics)
			src.BaseURL = serverURL
			return src
		},
	},
	{
		name: "tcbscans",
		newSource: func(serverURL string) domain.Source {
			src := NewTCBScans("Fixture").(*tcbscans)
			src.BaseURL = serverURL
			return src
		},
	},
}

// Cloudflare blocks non-browser user agents on these sites, so every request path must send browserUserAgent.
func TestScrapingSourcesSendBrowserUserAgent(t *testing.T) {
	t.Parallel()

	for _, tt := range scrapingSources {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var mu sync.Mutex
			var userAgents []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				userAgents = append(userAgents, r.UserAgent())
				mu.Unlock()
				http.NotFound(w, r)
			}))
			defer server.Close()

			src := tt.newSource(server.URL)
			// Errors are expected: the server only records the request headers.
			_, _ = src.Discover(t.Context())
			_, _ = src.Pages(t.Context(), domain.Chapter{URL: server.URL + "/chapters/fixture", Number: mustChapterNumber("1")})

			mu.Lock()
			defer mu.Unlock()
			require.GreaterOrEqual(t, len(userAgents), 2)
			for _, userAgent := range userAgents {
				require.Equal(t, browserUserAgent, userAgent)
			}
		})
	}
}

func TestScrapingSourcesReportCloudflareBlock(t *testing.T) {
	t.Parallel()

	for _, tt := range scrapingSources {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Server", "cloudflare")
				w.WriteHeader(http.StatusForbidden)
				_, _ = io.WriteString(w, `<div id="cf-wrapper"><div id="cf-error-details">Sorry, you have been blocked</div></div>`)
			}))
			defer server.Close()

			src := tt.newSource(server.URL)
			_, err := src.Discover(t.Context())
			require.ErrorIs(t, err, sharedhttp.ErrCloudflareBlock)

			// Some adapters fetch pages through retry-go, whose aggregate error does not unwrap.
			_, err = src.Pages(t.Context(), domain.Chapter{URL: server.URL + "/chapters/fixture", Number: mustChapterNumber("1")})
			require.ErrorContains(t, err, sharedhttp.ErrCloudflareBlock.Error()+": status code 403")
		})
	}
}
