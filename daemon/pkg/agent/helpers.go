package agent

import (
	"log/slog"
	"os"
	"strings"
)

// buildAgentEnv builds the process environment from the current OS env
// plus extra key-value pairs from the agent config.
func buildAgentEnv(extra map[string]string) []string {
	env := os.Environ()
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}

// logWriter is an io.Writer that logs each line to slog with a prefix.
type logWriter struct {
	logger *slog.Logger
	prefix string
	buf    strings.Builder
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.buf.Write(p)
	for {
		s := w.buf.String()
		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			break
		}
		line := s[:idx]
		w.logger.Debug(w.prefix + line)
		w.buf.Reset()
		w.buf.WriteString(s[idx+1:])
	}
	return len(p), nil
}
