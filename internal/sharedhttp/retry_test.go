package sharedhttp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/avast/retry-go"
	"github.com/stretchr/testify/require"
)

func TestRetryAfter(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		status    int
		headers   func(time.Time) []string
		wantWaits []time.Duration
		wantError string
	}{
		{"delta seconds", 429, func(time.Time) []string { return []string{"30"} }, []time.Duration{30 * time.Second}, ""},
		{"HTTP date", 429, func(now time.Time) []string { return []string{now.Add(time.Minute).UTC().Format(http.TimeFormat)} }, []time.Duration{time.Minute}, ""},
		{"server unavailable", 503, func(time.Time) []string { return []string{"10"} }, []time.Duration{10 * time.Second}, ""},
		{"updated guidance", 429, func(time.Time) []string { return []string{"10", "20"} }, []time.Duration{10 * time.Second, 20 * time.Second}, ""},
		{"maximum wait", 429, func(time.Time) []string { return []string{"300"} }, []time.Duration{RetryMaxDelay}, ""},
		{"too long", 429, func(time.Time) []string { return []string{"301"} }, nil, "exceeds maximum wait"},
		{"too distant date", 503, func(now time.Time) []string {
			return []string{now.Add(RetryMaxDelay + time.Second).UTC().Format(http.TimeFormat)}
		}, nil, "exceeds maximum wait"},
		{"overflow", 429, func(time.Time) []string { return []string{"99999999999999999999999999999999999999"} }, nil, "exceeds maximum wait"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				headers := tc.headers(start)
				var calls []time.Time
				var bodies []*trackedBody
				client := http.Client{Transport: retryRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					for _, body := range bodies {
						require.True(t, body.closed, "close each response before waiting/retrying")
					}
					calls = append(calls, time.Now())
					body := &trackedBody{Reader: strings.NewReader("fixture")}
					bodies = append(bodies, body)
					response := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: body, Request: req}
					if len(calls) <= len(headers) {
						response.StatusCode = tc.status
						response.Header.Set("Retry-After", headers[len(calls)-1])
					}
					return response, nil
				})}
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://fixture.invalid/page", nil)
				require.NoError(t, err)
				err = retryRequest(client, req)
				if tc.wantError != "" {
					require.ErrorContains(t, err, tc.wantError)
					require.ErrorContains(t, err, fmt.Sprint(tc.status))
				} else {
					require.NoError(t, err)
				}
				require.Len(t, calls, len(tc.wantWaits)+1)
				for i, wait := range tc.wantWaits {
					require.Equal(t, wait, calls[i+1].Sub(calls[i]))
				}
				for _, body := range bodies {
					require.True(t, body.closed)
				}
			})
		})
	}
}

func TestRetryFallbackIsBounded(t *testing.T) {
	t.Parallel()
	for _, header := range []string{"", "invalid", "-1", "0", "1", "Thu, 01 Jan 1970 00:00:00 GMT"} {
		t.Run(header, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var calls []time.Time
				client := http.Client{Transport: retryRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					calls = append(calls, time.Now())
					return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {header}}, Body: io.NopCloser(strings.NewReader("limited")), Request: req}, nil
				})}
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://fixture.invalid/page", nil)
				require.NoError(t, err)
				err = retryRequest(client, req)
				require.ErrorContains(t, err, "too many requests: status code 429")
				require.Len(t, calls, int(RetryAttempts))
				for i := 1; i < len(calls); i++ {
					delay := calls[i].Sub(calls[i-1])
					minimum := RetryDelay << (i - 1)
					require.GreaterOrEqual(t, delay, minimum)
					require.Less(t, delay, minimum+RetryMaxJitter)
				}
				require.Equal(t, calls[len(calls)-1], time.Now(), "do not wait after final failure")
			})
		})
	}
}

func TestRetryDoesNotRetryPermanentStatus(t *testing.T) {
	t.Parallel()
	for _, status := range []int{401, 403, 404, 405, 418} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			client := http.Client{Transport: retryRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: status, Header: http.Header{"Retry-After": {"30"}}, Body: io.NopCloser(strings.NewReader("permanent")), Request: req}, nil
			})}
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://fixture.invalid/page", nil)
			require.NoError(t, err)
			require.ErrorContains(t, retryRequest(client, req), fmt.Sprint(status))
			require.Equal(t, 1, calls)
		})
	}
}

func TestRetryAfterCancellation(t *testing.T) {
	t.Parallel()
	for _, cancelAfter := range []time.Duration{0, 2 * time.Second} {
		t.Run(cancelAfter.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				start := time.Now()
				if cancelAfter == 0 {
					cancel()
				} else {
					go func() {
						time.Sleep(cancelAfter)
						cancel()
					}()
				}
				calls := 0
				client := http.Client{Transport: retryRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					calls++
					return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {"30"}}, Body: io.NopCloser(strings.NewReader("limited")), Request: req}, nil
				})}
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://fixture.invalid/page", nil)
				require.NoError(t, err)
				require.ErrorIs(t, retryRequest(client, req), context.Canceled)
				require.Equal(t, cancelAfter, time.Since(start))
				if cancelAfter == 0 {
					require.Zero(t, calls)
				} else {
					require.Equal(t, 1, calls)
				}
			})
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		header string
		delay  time.Duration
		valid  bool
	}{
		{" 15 ", 15 * time.Second, true},
		{"0", 0, true},
		{"", 0, false},
		{"+15", 0, false},
		{"-15", 0, false},
		{"1.5", 0, false},
		{"tomorrow", 0, false},
		{now.Add(time.Minute).Format(http.TimeFormat), time.Minute, true},
		{now.Add(-time.Minute).Format(http.TimeFormat), 0, true},
	} {
		t.Run(tc.header, func(t *testing.T) {
			delay, valid := parseRetryAfter(tc.header, now)
			require.Equal(t, tc.valid, valid)
			require.Equal(t, tc.delay, delay)
		})
	}
}

func retryRequest(client http.Client, req *http.Request) error {
	return retry.Do(func() error {
		response, err := ExecRequest(client, req)
		if err != nil {
			return err
		}
		return response.Body.Close()
	}, RetryOptions(req.Context())...)
}

type retryRoundTripFunc func(*http.Request) (*http.Response, error)

func (f retryRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error {
	b.closed = true
	return nil
}
