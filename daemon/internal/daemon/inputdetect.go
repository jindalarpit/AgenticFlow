package daemon

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultPromptPatterns are the built-in patterns that indicate input is expected.
var DefaultPromptPatterns = []string{
	"? ",
	": ",
	"> ",
	"$ ",
	"?",
}

// InputDetectorConfig holds configuration for the input detector.
type InputDetectorConfig struct {
	InactivityTimeout  time.Duration
	AdditionalPatterns []string
}

// InputDetector monitors CLI output and signals when input is likely expected.
type InputDetector struct {
	config     InputDetectorConfig
	patterns   []string
	lastOutput time.Time
	waiting    atomic.Bool
	timer      *time.Timer
	mu         sync.Mutex
	onWaiting  func()
	onCleared  func()
}

func clampTimeout(timeout time.Duration) time.Duration {
	if timeout == 0 {
		return 10 * time.Second
	}
	if timeout < 3*time.Second {
		return 3 * time.Second
	}
	if timeout > 60*time.Second {
		return 60 * time.Second
	}
	return timeout
}

// NewInputDetector creates a detector with the given config and callbacks.
func NewInputDetector(cfg InputDetectorConfig, onWaiting, onCleared func()) *InputDetector {
	timeout := clampTimeout(cfg.InactivityTimeout)
	patterns := make([]string, 0, len(DefaultPromptPatterns)+len(cfg.AdditionalPatterns))
	patterns = append(patterns, DefaultPromptPatterns...)
	patterns = append(patterns, cfg.AdditionalPatterns...)

	return &InputDetector{
		config: InputDetectorConfig{
			InactivityTimeout:  timeout,
			AdditionalPatterns: cfg.AdditionalPatterns,
		},
		patterns:  patterns,
		onWaiting: onWaiting,
		onCleared: onCleared,
	}
}

// OnOutput is called each time the CLI produces output.
func (d *InputDetector) OnOutput(content string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.lastOutput = time.Now()

	if d.waiting.Load() {
		d.waiting.Store(false)
		if d.onCleared != nil {
			go d.onCleared()
		}
	}

	d.resetTimer()

	if d.matchesPromptPattern(content) {
		d.signalWaiting()
	}
}

func (d *InputDetector) matchesPromptPattern(content string) bool {
	trimmed := strings.TrimRight(content, "\r\n")
	if trimmed == "" {
		return false
	}
	for _, pattern := range d.patterns {
		if strings.HasSuffix(trimmed, pattern) {
			return true
		}
	}
	return false
}

func (d *InputDetector) signalWaiting() {
	if !d.waiting.Load() {
		d.waiting.Store(true)
		if d.onWaiting != nil {
			go d.onWaiting()
		}
	}
}

// Stop cleans up the detector's timer and clears the waiting state.
func (d *InputDetector) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
	if d.waiting.Load() {
		d.waiting.Store(false)
		if d.onCleared != nil {
			go d.onCleared()
		}
	}
}

func (d *InputDetector) resetTimer() {
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.config.InactivityTimeout, func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		d.signalWaiting()
	})
}

// IsWaiting returns whether the detector currently believes the CLI is waiting for input.
func (d *InputDetector) IsWaiting() bool {
	return d.waiting.Load()
}
