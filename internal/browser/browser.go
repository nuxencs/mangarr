package browser

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

const (
	Timeout = 30 * time.Second
)

// Manager handles the shared browser instance
type Manager struct {
	browser *rod.Browser
	mu      sync.Mutex
}

// NewManager creates a new browser manager
func NewManager() *Manager {
	return &Manager{}
}

// Get returns a configured browser instance or creates one if none exists
func (bm *Manager) Get() *rod.Browser {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.browser != nil {
		return bm.browser
	}

	path, _ := launcher.LookPath()
	u := launcher.New().Bin(path).MustLaunch()

	bm.browser = rod.New().
		ControlURL(u).
		MustConnect()

	return bm.browser
}

// Close cleanly shuts down the browser
func (bm *Manager) Close() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.browser != nil {
		bm.browser.MustClose()
		bm.browser = nil
	}
}

func HandleError(err error) error {
	var evalErr *rod.EvalError
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("context deadline exceeded")
	case errors.As(err, &evalErr):
		return fmt.Errorf("evaluation error in line %d: %w", evalErr.LineNumber, evalErr)
	default:
		return err
	}
}
