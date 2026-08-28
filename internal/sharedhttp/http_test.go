package sharedhttp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/avast/retry-go"
	"github.com/stretchr/testify/require"
)

func TestExecRequestDoesNotRetryOnNotFound(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
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
	require.Error(t, err)
	require.Contains(t, err.Error(), ErrNotFound.Error())
	require.Equal(t, int32(1), attempts.Load())
}

func TestExecRequestDoesNotRetryOnTooManyRequests(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
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
	require.Error(t, err)
	require.Contains(t, err.Error(), "too many requests")
	require.Equal(t, int32(1), attempts.Load())
}

func TestExecRequestRetriesOnServerError(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
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
	require.NoError(t, err)
	require.Equal(t, int32(2), attempts.Load())
}

func TestExecRequestRetriesOnTransportError(t *testing.T) {
	t.Parallel()

	var servedAttempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		servedAttempts.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	base := server.Client().Transport
	flaky := &flakyRoundTripper{base: base, remainingFailures: 1}
	client := http.Client{
		Timeout:   2 * time.Second,
		Transport: flaky,
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	err = retry.Do(func() error {
		resp, reqErr := ExecRequest(client, req)
		if reqErr != nil {
			return reqErr
		}
		defer resp.Body.Close()

		return nil
	}, RetryOptions(t.Context())...)
	require.NoError(t, err)
	require.Equal(t, int32(2), flaky.calls.Load())
	require.Equal(t, int32(1), servedAttempts.Load())
}

type flakyRoundTripper struct {
	base              http.RoundTripper
	remainingFailures int32
	calls             atomic.Int32
}

func (f *flakyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	current := f.calls.Add(1)
	if current <= atomic.LoadInt32(&f.remainingFailures) {
		return nil, fmt.Errorf("simulated transport failure")
	}

	return f.base.RoundTrip(req)
}
