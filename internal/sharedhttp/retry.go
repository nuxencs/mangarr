package sharedhttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go"
)

const (
	RetryAttempts  uint          = 3
	RetryDelay     time.Duration = time.Second
	RetryMaxJitter time.Duration = 250 * time.Millisecond
	RetryMaxDelay  time.Duration = 5 * time.Minute
)

func RetryOptions(ctx context.Context, options ...retry.Option) []retry.Option {
	base := []retry.Option{
		retry.Delay(RetryDelay),
		retry.Attempts(RetryAttempts),
		retry.MaxJitter(RetryMaxJitter),
		retry.DelayType(responseRetryDelay),
		retry.Context(ctx),
	}

	return append(base, options...)
}

type retryAfterError struct {
	err   error
	delay time.Duration
}

func (e *retryAfterError) Error() string { return e.err.Error() }
func (e *retryAfterError) Unwrap() error { return e.err }

func responseRetryDelay(n uint, err error, config *retry.Config) time.Duration {
	delay := retry.CombineDelay(retry.BackOffDelay, retry.RandomDelay)(n, err, config)
	var guided *retryAfterError
	if errors.As(err, &guided) {
		// Server guidance is a minimum wait, never a reason to retry sooner.
		delay = max(delay, guided.delay)
	}
	return delay
}

func parseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}

	if strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) == -1 {
		seconds, err := strconv.ParseUint(value, 10, 64)
		if err != nil || seconds > uint64(RetryMaxDelay/time.Second) {
			// Oversized guidance must fail rather than overflow into an early retry.
			return RetryMaxDelay + time.Second, true
		}
		return time.Duration(seconds) * time.Second, true
	}

	deadline, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	return max(0, deadline.Sub(now)), true
}
