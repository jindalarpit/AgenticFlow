package realtime

import (
	"net/http"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// genPATToken generates random valid PAT-like token strings.
// PAT tokens are alphanumeric strings prefixed with "af_" that don't contain
// commas or spaces (which would break the Sec-WebSocket-Protocol header parsing).
func genPATToken() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		// Generate a random alphanumeric suffix (simulating a PAT token body)
		chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"
		length := rapid.IntRange(8, 64).Draw(t, "tokenLength")
		var sb strings.Builder
		sb.WriteString("af_")
		for i := 0; i < length; i++ {
			idx := rapid.IntRange(0, len(chars)-1).Draw(t, "charIdx")
			sb.WriteByte(chars[idx])
		}
		return sb.String()
	})
}

// genNonTokenProtocol generates random sub-protocol strings that do NOT start with "access_token.".
func genNonTokenProtocol() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		// Generate a protocol name that doesn't start with "access_token."
		prefixes := []string{"graphql", "mqtt", "chat", "v1", "binary", "json"}
		idx := rapid.IntRange(0, len(prefixes)-1).Draw(t, "prefixIdx")
		suffix := rapid.StringMatching(`[a-zA-Z0-9._-]{0,20}`).Draw(t, "suffix")
		return prefixes[idx] + suffix
	})
}

// **Validates: Requirements 4.2, 4.3**

// TestProperty_ExtractTokenFromProtocol_SingleToken verifies that for any generated
// token string, when placed in a Sec-WebSocket-Protocol header as "access_token.<token>",
// extractTokenFromProtocol returns the original token.
func TestProperty_ExtractTokenFromProtocol_SingleToken(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		token := genPATToken().Draw(t, "token")

		req, _ := http.NewRequest("GET", "/ws", nil)
		req.Header.Set("Sec-WebSocket-Protocol", "access_token."+token)

		extracted := extractTokenFromProtocol(req)
		if extracted != token {
			t.Fatalf("expected token %q, got %q", token, extracted)
		}
	})
}

// TestProperty_ExtractTokenFromProtocol_NoPrefix verifies that when no sub-protocol
// has the "access_token." prefix, extractTokenFromProtocol returns an empty string.
func TestProperty_ExtractTokenFromProtocol_NoPrefix(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate 1-5 non-token protocols
		count := rapid.IntRange(1, 5).Draw(t, "protocolCount")
		protocols := make([]string, count)
		for i := 0; i < count; i++ {
			protocols[i] = genNonTokenProtocol().Draw(t, "protocol")
		}

		req, _ := http.NewRequest("GET", "/ws", nil)
		req.Header.Set("Sec-WebSocket-Protocol", strings.Join(protocols, ", "))

		extracted := extractTokenFromProtocol(req)
		if extracted != "" {
			t.Fatalf("expected empty string when no access_token prefix, got %q", extracted)
		}
	})
}

// TestProperty_ExtractTokenFromProtocol_MultipleProtocols verifies that when multiple
// sub-protocols are present and one is "access_token.<token>", the function still
// correctly extracts the token.
func TestProperty_ExtractTokenFromProtocol_MultipleProtocols(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		token := genPATToken().Draw(t, "token")

		// Generate 1-4 non-token protocols
		otherCount := rapid.IntRange(1, 4).Draw(t, "otherCount")
		protocols := make([]string, 0, otherCount+1)
		for i := 0; i < otherCount; i++ {
			protocols = append(protocols, genNonTokenProtocol().Draw(t, "otherProtocol"))
		}

		// Insert the access_token protocol at a random position
		insertIdx := rapid.IntRange(0, len(protocols)).Draw(t, "insertIdx")
		tokenProtocol := "access_token." + token
		protocols = append(protocols[:insertIdx], append([]string{tokenProtocol}, protocols[insertIdx:]...)...)

		req, _ := http.NewRequest("GET", "/ws", nil)
		req.Header.Set("Sec-WebSocket-Protocol", strings.Join(protocols, ", "))

		extracted := extractTokenFromProtocol(req)
		if extracted != token {
			t.Fatalf("expected token %q from multiple protocols, got %q (protocols: %v)", token, extracted, protocols)
		}
	})
}

// TestProperty_ExtractTokenFromProtocol_EmptyHeader verifies that when no
// Sec-WebSocket-Protocol header is set, the function returns empty string.
func TestProperty_ExtractTokenFromProtocol_EmptyHeader(t *testing.T) {
	req, _ := http.NewRequest("GET", "/ws", nil)
	extracted := extractTokenFromProtocol(req)
	if extracted != "" {
		t.Fatalf("expected empty string for missing header, got %q", extracted)
	}
}
