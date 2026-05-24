package daemon

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultPromptPatterns are the built-in patterns that indicate input is expected.
// These are checked as suffixes of the trimmed output content.
var DefaultPromptPatterns = []string{
	"? ",  // Question mark followed by space (common prompt)
	": ",  // Colon followed by space (password/input prompts)
	"> ",  // Greater-than followed by space (shell-like prompts)
	"$ ",  // Dollar followed by space (shell prompts)
	"?",   // Line ending with question mark
}

// InputDetectorConfig holds configuration for the input detector.
type InputDetectorConfig struct {
	// InactivityTimeout is the duration of silence before signaling a potential
	// input wait. Default 10s, min 3s, max 60s.
	InactivityTimeout time.Duration

	// AdditionalPatterns are extra prompt patterns from daemon configuration
	// that are appended to DefaultPromptPatterns.
	AdditionalPatterns []string
}

// InputDetector monitors CLI output and signals when input is likely expected.
// It uses two detection mechanisms:
//  1. Pattern matching: checks if output ends with a recognized prompt pattern.
//  2. Inactivity timeout: signals after a configurable period of no output.
type InputDetector struct {
	config    InputDetectorConfig
	patterns  []string // merged default + additional patterns
	lastOutput time.Time
	waiting   atomic.Bool
	timer     *time.Timer
	mu        sync.Mutex
	onWaiting func() // callback when input is detected
	onCleared func() // callback when input state is cleared
}

// clampTimeout ensures the timeout is within the allowed range [3s, 60s],
// defaulting to 10s if zero.
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
// The onWaiting callback is invoked (in a goroutine) when the detector determines
// the CLI is waiting for input. The onCleared callback is invoked when the waiting
// state is cleared (output resumes or detector is stopped).
func NewInputDetector(cfg InputDetectorConfig, onWaiting, onCleared func()) *InputDetector {
	timeout := clampTimeout(cfg.InactivityTimeout)

	// Merge default patterns with additional patterns.
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
// It checks for prompt patterns and resets the inactivity timer.
func (d *InputDetector) OnOutput(content string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.lastOutput = time.Now()

	// If we were waiting, clear the state (output resumed).
	if d.waiting.Load() {
		d.waiting.Store(false)
		if d.onCleared != nil {
			go d.onCleared()
		}
	}

	// Reset inactivity timer.
	d.resetTimer()

	// Check if output ends with a prompt pattern.
	if d.matchesPromptPattern(content) {
		d.signalWaiting()
	}
}

// matchesPromptPattern checks if the content ends with a recognized prompt pattern.
// It trims trailing \r\n before checking suffixes.
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

// signalWaiting transitions to the waiting state and invokes the onWaiting callback.
// It is a no-op if already in the waiting state.
func (d *InputDetector) signalWaiting() {
	if !d.waiting.Load() {
		d.waiting.Store(true)
		if d.onWaiting != nil {
			go d.onWaiting()
		}
	}
}

// Stop cleans up the detector's timer and clears the waiting state.
// This should be called when the CLI process exits.
func (d *InputDetector) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}

	// Clear waiting state on process exit.
	if d.waiting.Load() {
		d.waiting.Store(false)
		if d.onCleared != nil {
			go d.onCleared()
		}
	}
}

// resetTimer resets or creates the inactivity timer. When the timer fires,
// it calls signalWaiting to indicate the CLI may be waiting for input.
// Must be called with d.mu held.
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
