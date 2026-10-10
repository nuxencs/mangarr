package sharedhttp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/avast/retry-go"
	"github.com/stretchr/testify/require"
)

const cloudflareBlockPage = `<html><head><title>Attention Required! | Cloudflare</title></head>` +
	`<body><div id="cf-wrapper"><div id="cf-error-details">Sorry, you have been blocked</div></div></body></html>`

func cloudflareBlockHandler(attempts *atomic.Int32) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.Header().Set("Server", "cloudflare")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, cloudflareBlockPage)
	}
}

func TestExecRequestReportsCloudflareBlock(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	server := httptest.NewServer(cloudflareBlockHandler(&attempts))
	defer server.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	client := server.Client()
	err = retry.Do(func() error {
		resp, reqErr := ExecRequest(*client, req)
		if reqErr != nil {
			return reqErr
		}
		defer resp.Body.Close()

		return nil
	}, RetryOptions(t.Context())...)
	// retry-go aggregates attempt errors without Unwrap, so match the message.
	require.ErrorContains(t, err, ErrCloudflareBlock.Error()+": status code 403")
	require.NotContains(t, err.Error(), "authentication error")
	require.Equal(t, int32(1), attempts.Load())
}

func TestExecRequestKeepsOrigin403AsAuthenticationError(t *testing.T) {
	t.Parallel()

	tests := map[string]http.HandlerFunc{
		"no Cloudflare server header": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, cloudflareBlockPage)
		},
		"Cloudflare-proxied origin 403": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Server", "cloudflare")
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, "origin says no")
		},
	}

	for name, handler := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(handler)
			defer server.Close()

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
			require.NoError(t, err)

			_, err = ExecRequest(*server.Client(), req)
			require.ErrorContains(t, err, "authentication error: status code 403")
			require.NotErrorIs(t, err, ErrCloudflareBlock)
		})
	}
}

func TestCloudflareTransportReportsBlock(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	server := httptest.NewServer(cloudflareBlockHandler(&attempts))
	defer server.Close()

	client := http.Client{Transport: CloudflareTransport(server.Client().Transport)}
	resp, err := client.Get(server.URL)
	if resp != nil {
		_ = resp.Body.Close()
	}
	require.ErrorIs(t, err, ErrCloudflareBlock)
}

func TestCloudflareTransportPreservesOther403Body(t *testing.T) {
	t.Parallel()

	// Longer than the peek limit, so the restored body must join the peeked prefix and the unread rest.
	body := strings.Repeat("origin says no. ", cloudflareBodyPeekLimit/8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Server", "cloudflare")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, body)
	}))
	defer server.Close()

	client := http.Client{Transport: CloudflareTransport(server.Client().Transport)}
	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.Equal(t, body, string(got))
}
