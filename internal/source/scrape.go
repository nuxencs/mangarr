package source

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"mangarr/internal/domain"
	"mangarr/internal/sharedhttp"

	"github.com/avast/retry-go"
	"github.com/gocolly/colly/v2"
)

// browserUserAgent is sent on page and image requests of HTML-scraping adapters, because Cloudflare WAF rules on those sites block non-browser user agents.
const browserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"

// newScrapeCollector returns the base collector for HTML-scraping adapters.
// Set request behavior here as collector config or transport, which Clone keeps. Clone drops
// callbacks, so extensions.RandomUserAgent made clones send colly's default user agent.
func newScrapeCollector(timeout time.Duration) *colly.Collector {
	collector := colly.NewCollector(
		colly.AllowURLRevisit(),
		colly.UserAgent(browserUserAgent),
	)
	collector.SetRequestTimeout(timeout)
	// Without this, Colly reports Cloudflare blocks as a bare "Forbidden".
	collector.WithTransport(sharedhttp.CloudflareTransport(sharedhttp.Transport))

	return collector
}

// browserImage sends the browser user agent with an image download, because scraping sources' image hosts are behind Cloudflare too.
func browserImage(imageURL string) domain.ImageInfo {
	return domain.ImageInfo{
		ImageURL:       imageURL,
		RequestHeaders: map[string]string{"User-Agent": browserUserAgent},
	}
}

// fetchPage gets rawURL with the browser user agent and shared retries, for adapters that parse a page without Colly.
func fetchPage(ctx context.Context, client http.Client, rawURL, accept string) ([]byte, error) {
	var body []byte

	err := retry.Do(func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return fmt.Errorf("creating request for %s: %w", rawURL, err)
		}

		req.Header.Set("User-Agent", browserUserAgent)
		req.Header.Set("Accept", accept)

		resp, err := sharedhttp.ExecRequest(client, req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("reading response body from %s: %w", rawURL, err)
		}

		return nil
	}, sharedhttp.RetryOptions(ctx)...)
	if err != nil {
		return nil, err
	}

	return body, nil
}
