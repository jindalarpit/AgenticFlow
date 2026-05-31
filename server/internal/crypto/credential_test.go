package crypto

import (
	"encoding/hex"
	"testing"
)

func TestNewCredentialEncryptor_ValidKey(t *testing.T) {
	// 64 hex chars = 32 bytes
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	enc, err := NewCredentialEncryptor(key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enc == nil {
		t.Fatal("expected non-nil encryptor")
	}
	if len(enc.key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(enc.key))
	}
}

func TestNewCredentialEncryptor_InvalidLength(t *testing.T) {
	// Too short
	_, err := NewCredentialEncryptor("0123456789abcdef")
	if err == nil {
		t.Fatal("expected error for short key")
	}

	// Too long
	_, err = NewCredentialEncryptor("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef00")
	if err == nil {
		t.Fatal("expected error for long key")
	}
}

func TestNewCredentialEncryptor_InvalidHex(t *testing.T) {
	// 64 chars but not valid hex
	key := "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"
	_, err := NewCredentialEncryptor(key)
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := hex.EncodeToString(make([]byte, 32))
	// Use a known key for testing
	key = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	enc, err := NewCredentialEncryptor(key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testCases := []struct {
		name      string
		plaintext []byte
	}{
		{"empty", []byte{}},
		{"simple json", []byte(`{"api_key":"sk-test123"}`)},
		{"complex json", []byte(`{"api_key":"sk-proj-abc123","base_url":"https://api.openai.com/v1","organization":"org-xyz"}`)},
		{"single byte", []byte{0x42}},
		{"binary data", []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0xfd}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ciphertext, err := enc.Encrypt(tc.plaintext)
			if err != nil {
				t.Fatalf("encrypt failed: %v", err)
			}

			if ciphertext == "" {
				t.Fatal("expected non-empty ciphertext")
			}

			decrypted, err := enc.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("decrypt failed: %v", err)
			}

			if len(tc.plaintext) == 0 && len(decrypted) == 0 {
				return // both empty, OK
			}

			if string(decrypted) != string(tc.plaintext) {
				t.Fatalf("round-trip failed: got %q, want %q", decrypted, tc.plaintext)
			}
		})
	}
}

func TestEncrypt_UniqueNonce(t *testing.T) {
	key := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	enc, err := NewCredentialEncryptor(key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plaintext := []byte(`{"api_key":"test"}`)

	// Encrypt the same plaintext twice — should produce different ciphertexts
	ct1, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt 1 failed: %v", err)
	}

	ct2, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt 2 failed: %v", err)
	}

	if ct1 == ct2 {
		t.Fatal("expected different ciphertexts due to random nonce")
	}
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	key := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	enc, err := NewCredentialEncryptor(key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = enc.Decrypt("not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecrypt_TooShort(t *testing.T) {
	key := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	enc, err := NewCredentialEncryptor(key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Base64 of a very short byte slice (less than 12 bytes nonce)
	_, err = enc.Decrypt("AQID") // 3 bytes
	if err == nil {
		t.Fatal("expected error for ciphertext too short")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	key := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	enc, err := NewCredentialEncryptor(key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plaintext := []byte(`{"secret":"value"}`)
	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	// Tamper with the ciphertext by changing a character
	tampered := []byte(ciphertext)
	// Change a character in the middle (after base64 encoding)
	if len(tampered) > 20 {
		tampered[20] = 'A'
		if tampered[20] == ciphertext[20] {
			tampered[20] = 'B'
		}
	}

	_, err = enc.Decrypt(string(tampered))
	if err == nil {
		t.Fatal("expected error for tampered ciphertext")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	key2 := "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

	enc1, _ := NewCredentialEncryptor(key1)
	enc2, _ := NewCredentialEncryptor(key2)

	plaintext := []byte(`{"api_key":"sk-secret"}`)
	ciphertext, err := enc1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	_, err = enc2.Decrypt(ciphertext)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestMaskCredentialValue(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"single char", "a", "*"},
		{"two chars", "ab", "**"},
		{"three chars", "abc", "***"},
		{"exactly four chars", "abcd", "abcd"},
		{"five chars", "abcde", "*bcde"},
		{"typical api key", "sk-test-key-12345", "*************2345"},
		{"short key", "sk-1", "sk-1"},
		{"long key", "sk-proj-abcdefghijklmnopqrstuvwxyz123456", "************************************3456"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := MaskCredentialValue(tc.input)
			if result != tc.expected {
				t.Errorf("MaskCredentialValue(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestMaskCredentialValue_NeverLeaksFullValue(t *testing.T) {
	// For any string >= 5 chars, the masked output should never equal the original
	values := []string{
		"sk-test-key-12345",
		"AKIA1234567890ABCDEF",
		"super-secret-password",
		"12345",
	}

	for _, v := range values {
		masked := MaskCredentialValue(v)
		if masked == v {
			t.Errorf("MaskCredentialValue(%q) returned the original value unchanged", v)
		}
	}
}

func TestMaskCredentialValue_LastFourVisible(t *testing.T) {
	// For strings >= 4 chars, the last 4 chars should be visible
	values := []string{
		"sk-test-key-12345",
		"AKIA1234567890ABCDEF",
		"abcd",
		"12345678",
	}

	for _, v := range values {
		masked := MaskCredentialValue(v)
		if len(v) >= 4 {
			last4 := v[len(v)-4:]
			maskedLast4 := masked[len(masked)-4:]
			if maskedLast4 != last4 {
				t.Errorf("MaskCredentialValue(%q): last 4 chars = %q, want %q", v, maskedLast4, last4)
			}
		}
	}
}

func TestMaskCredentialValue_CorrectLength(t *testing.T) {
	// The masked output should always have the same length as the input
	values := []string{"", "a", "ab", "abc", "abcd", "abcde", "sk-test-key-12345"}

	for _, v := range values {
		masked := MaskCredentialValue(v)
		if len(masked) != len(v) {
			t.Errorf("MaskCredentialValue(%q): length = %d, want %d", v, len(masked), len(v))
		}
	}
}
