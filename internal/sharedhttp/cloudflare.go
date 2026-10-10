package sharedhttp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ErrCloudflareChallenge reports that Cloudflare served a challenge page instead of the upstream response.
var ErrCloudflareChallenge = errors.New("blocked by Cloudflare challenge")

// ErrCloudflareBlock reports that a Cloudflare WAF rule blocked the request.
var ErrCloudflareBlock = errors.New("blocked by Cloudflare")

// cloudflareBodyPeekLimit bounds how much of a 403 body is read to look for the block page marker.
const cloudflareBodyPeekLimit = 64 << 10

// checkCloudflare returns an error when resp is a Cloudflare challenge or block page.
// Plain HTTP cannot pass either, so callers fail fast with the real cause instead of a generic 403.
// For other 403 responses, it restores resp.Body so callers still read the full upstream body.
func checkCloudflare(resp *http.Response) error {
	if resp.Header.Get("Cf-Mitigated") == "challenge" {
		return fmt.Errorf("%w: status code %d", ErrCloudflareChallenge, resp.StatusCode)
	}

	if resp.StatusCode != http.StatusForbidden || !strings.EqualFold(resp.Header.Get("Server"), "cloudflare") {
		return nil
	}

	prefix, err := io.ReadAll(io.LimitReader(resp.Body, cloudflareBodyPeekLimit))
	resp.Body = readCloser{Reader: io.MultiReader(bytes.NewReader(prefix), resp.Body), Closer: resp.Body}
	// Cloudflare-generated error pages, such as WAF blocks, wrap their details in this element.
	// The match needs a decoded body: a caller that sets Accept-Encoding itself turns off transparent gzip decoding.
	// On a read error, fall back to normal status handling instead of guessing.
	if err == nil && bytes.Contains(prefix, []byte("cf-error-details")) {
		return fmt.Errorf("%w: status code %d", ErrCloudflareBlock, resp.StatusCode)
	}

	return nil
}

type readCloser struct {
	io.Reader
	io.Closer
}

// CloudflareTransport wraps base so Cloudflare challenge and block pages become request errors that wrap
// ErrCloudflareChallenge or ErrCloudflareBlock. Use it for clients that do not go through ExecRequest, such as Colly.
func CloudflareTransport(base http.RoundTripper) http.RoundTripper {
	return cloudflareTransport{base: base}
}

type cloudflareTransport struct {
	base http.RoundTripper
}

func (t cloudflareTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if err := checkCloudflare(resp); err != nil {
		_ = resp.Body.Close()
		return nil, err
	}

	return resp, nil
}
