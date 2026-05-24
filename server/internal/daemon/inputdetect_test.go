package daemon

import (
	"sync"
	"testing"
	"time"
)

func TestNewInputDetector_DefaultTimeout(t *testing.T) {
	d := NewInputDetector(InputDetectorConfig{}, nil, nil)
	if d.config.InactivityTimeout != 10*time.Second {
		t.Fatalf("expected default timeout 10s, got %v", d.config.InactivityTimeout)
	}
}

func TestNewInputDetector_ClampMinTimeout(t *testing.T) {
	d := NewInputDetector(InputDetectorConfig{InactivityTimeout: 1 * time.Second}, nil, nil)
	if d.config.InactivityTimeout != 3*time.Second {
		t.Fatalf("expected clamped timeout 3s, got %v", d.config.InactivityTimeout)
	}
}

func TestNewInputDetector_ClampMaxTimeout(t *testing.T) {
	d := NewInputDetector(InputDetectorConfig{InactivityTimeout: 120 * time.Second}, nil, nil)
	if d.config.InactivityTimeout != 60*time.Second {
		t.Fatalf("expected clamped timeout 60s, got %v", d.config.InactivityTimeout)
	}
}

func TestNewInputDetector_ValidTimeout(t *testing.T) {
	d := NewInputDetector(InputDetectorConfig{InactivityTimeout: 15 * time.Second}, nil, nil)
	if d.config.InactivityTimeout != 15*time.Second {
		t.Fatalf("expected timeout 15s, got %v", d.config.InactivityTimeout)
	}
}

func TestNewInputDetector_MergesPatterns(t *testing.T) {
	additional := []string{">>> ", "# "}
	d := NewInputDetector(InputDetectorConfig{AdditionalPatterns: additional}, nil, nil)

	expectedLen := len(DefaultPromptPatterns) + len(additional)
	if len(d.patterns) != expectedLen {
		t.Fatalf("expected %d patterns, got %d", expectedLen, len(d.patterns))
	}
}

func TestMatchesPromptPattern_DefaultPatterns(t *testing.T) {
	d := NewInputDetector(InputDetectorConfig{}, nil, nil)

	tests := []struct {
		content string
		want    bool
	}{
		{"Do you want to continue? ", true},
		{"Enter password: ", true},
		{"shell> ", true},
		{"user@host$ ", true},
		{"Are you sure?", true},
		{"Are you sure?\n", true},   // trailing newline trimmed
		{"Are you sure?\r\n", true}, // trailing CRLF trimmed
		{"just some output", false},
		{"no match here!", false},
		{"", false},
		{"\n", false},
	}

	for _, tt := range tests {
		got := d.matchesPromptPattern(tt.content)
		if got != tt.want {
			t.Errorf("matchesPromptPattern(%q) = %v, want %v", tt.content, got, tt.want)
		}
	}
}

func TestMatchesPromptPattern_AdditionalPatterns(t *testing.T) {
	d := NewInputDetector(InputDetectorConfig{
		AdditionalPatterns: []string{">>> ", "(y/n) "},
	}, nil, nil)

	tests := []struct {
		content string
		want    bool
	}{
		{"python>>> ", true},
		{"Continue? (y/n) ", true},
		{"no match", false},
	}

	for _, tt := range tests {
		got := d.matchesPromptPattern(tt.content)
		if got != tt.want {
			t.Errorf("matchesPromptPattern(%q) = %v, want %v", tt.content, got, tt.want)
		}
	}
}

func TestOnOutput_SignalsWaitingOnPromptPattern(t *testing.T) {
	var mu sync.Mutex
	waitingCalled := false

	d := NewInputDetector(InputDetectorConfig{InactivityTimeout: 10 * time.Second}, func() {
		mu.Lock()
		waitingCalled = true
		mu.Unlock()
	}, nil)
	defer d.Stop()

	d.OnOutput("Enter your name: ")

	// Give the goroutine time to execute.
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if !waitingCalled {
		t.Fatal("expected onWaiting to be called for prompt pattern")
	}
	mu.Unlock()

	if !d.IsWaiting() {
		t.Fatal("expected IsWaiting() to be true")
	}
}

func TestOnOutput_ClearsWaitingOnNewOutput(t *testing.T) {
	var mu sync.Mutex
	clearedCalled := false

	d := NewInputDetector(InputDetectorConfig{InactivityTimeout: 10 * time.Second}, func() {}, func() {
		mu.Lock()
		clearedCalled = true
		mu.Unlock()
	})
	defer d.Stop()

	// First, trigger waiting state.
	d.OnOutput("Continue? ")
	time.Sleep(50 * time.Millisecond)

	if !d.IsWaiting() {
		t.Fatal("expected IsWaiting() to be true after prompt")
	}

	// Now send new output — should clear waiting state.
	d.OnOutput("Processing...\n")
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if !clearedCalled {
		t.Fatal("expected onCleared to be called when output resumes")
	}
	mu.Unlock()

	if d.IsWaiting() {
		t.Fatal("expected IsWaiting() to be false after new output")
	}
}

func TestOnOutput_DoesNotSignalForNonMatchingOutput(t *testing.T) {
	waitingCalled := false

	d := NewInputDetector(InputDetectorConfig{InactivityTimeout: 10 * time.Second}, func() {
		waitingCalled = true
	}, nil)
	defer d.Stop()

	d.OnOutput("Building project...\n")
	time.Sleep(50 * time.Millisecond)

	if waitingCalled {
		t.Fatal("onWaiting should not be called for non-matching output")
	}

	if d.IsWaiting() {
		t.Fatal("expected IsWaiting() to be false for non-matching output")
	}
}

func TestInactivityTimeout_SignalsWaiting(t *testing.T) {
	var mu sync.Mutex
	waitingCalled := false

	d := NewInputDetector(InputDetectorConfig{InactivityTimeout: 3 * time.Second}, func() {
		mu.Lock()
		waitingCalled = true
		mu.Unlock()
	}, nil)
	defer d.Stop()

	// Send non-matching output to start the timer.
	d.OnOutput("Starting process...\n")

	// Wait for the inactivity timeout to fire (3s + buffer).
	time.Sleep(3500 * time.Millisecond)

	mu.Lock()
	if !waitingCalled {
		t.Fatal("expected onWaiting to be called after inactivity timeout")
	}
	mu.Unlock()

	if !d.IsWaiting() {
		t.Fatal("expected IsWaiting() to be true after inactivity timeout")
	}
}

func TestStop_ClearsWaitingState(t *testing.T) {
	var mu sync.Mutex
	clearedCalled := false

	d := NewInputDetector(InputDetectorConfig{InactivityTimeout: 10 * time.Second}, func() {}, func() {
		mu.Lock()
		clearedCalled = true
		mu.Unlock()
	})

	// Trigger waiting state.
	d.OnOutput("Enter value: ")
	time.Sleep(50 * time.Millisecond)

	if !d.IsWaiting() {
		t.Fatal("expected IsWaiting() to be true before Stop")
	}

	// Stop should clear the waiting state.
	d.Stop()
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if !clearedCalled {
		t.Fatal("expected onCleared to be called on Stop")
	}
	mu.Unlock()

	if d.IsWaiting() {
		t.Fatal("expected IsWaiting() to be false after Stop")
	}
}

func TestStop_NoCallbackIfNotWaiting(t *testing.T) {
	clearedCalled := false

	d := NewInputDetector(InputDetectorConfig{InactivityTimeout: 10 * time.Second}, nil, func() {
		clearedCalled = true
	})

	// Stop without ever entering waiting state.
	d.Stop()
	time.Sleep(50 * time.Millisecond)

	if clearedCalled {
		t.Fatal("onCleared should not be called if not in waiting state")
	}
}

func TestSignalWaiting_OnlySignalsOnce(t *testing.T) {
	var mu sync.Mutex
	callCount := 0

	d := NewInputDetector(InputDetectorConfig{InactivityTimeout: 10 * time.Second}, func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	}, nil)
	defer d.Stop()

	// Send multiple prompt-matching outputs without clearing.
	d.OnOutput("First? ")
	time.Sleep(50 * time.Millisecond)

	// Manually call signalWaiting again (simulating timer fire while already waiting).
	d.mu.Lock()
	d.signalWaiting()
	d.mu.Unlock()
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if callCount != 1 {
		t.Fatalf("expected onWaiting called once, got %d", callCount)
	}
	mu.Unlock()
}
