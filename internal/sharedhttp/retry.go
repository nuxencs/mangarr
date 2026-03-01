package sharedhttp

import (
	"context"
	"time"

	"github.com/avast/retry-go"
)

const (
	RetryAttempts  uint          = 3
	RetryDelay     time.Duration = time.Second
	RetryMaxJitter time.Duration = 250 * time.Millisecond
)

func RetryOptions(ctx context.Context, options ...retry.Option) []retry.Option {
	base := []retry.Option{
		retry.Delay(RetryDelay),
		retry.Attempts(RetryAttempts),
		retry.MaxJitter(RetryMaxJitter),
		retry.Context(ctx),
	}

	return append(base, options...)
}
