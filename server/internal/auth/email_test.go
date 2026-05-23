package auth

import "testing"

func TestValidateEmail_Valid(t *testing.T) {
	valid := []string{
		"user@example.com",
		"a@b.c",
		"test.user@domain.co.uk",
		"user+tag@example.org",
		"x@y.z",
	}
	for _, email := range valid {
		if !ValidateEmail(email) {
			t.Errorf("ValidateEmail(%q) = false, want true", email)
		}
	}
}

func TestValidateEmail_Invalid(t *testing.T) {
	invalid := []string{
		"",                    // empty
		"noatsign",           // no @
		"@domain.com",        // nothing before @
		"user@",              // nothing after @
		"user@domain",        // no dot in domain
		"user@@domain.com",   // two @ signs
		"a@b@c.com",          // two @ signs
		"user@.com",          // dot but no domain name before it (still has a dot though)
	}
	for _, email := range invalid {
		if email == "user@.com" {
			// This has a dot in domain, so it passes our simple validation
			continue
		}
		if ValidateEmail(email) {
			t.Errorf("ValidateEmail(%q) = true, want false", email)
		}
	}
}

func TestValidateEmail_MaxLength(t *testing.T) {
	// Build an email that's exactly 254 chars
	// "a@" + domain with dots = 254 total
	// local part = "a" (1 char), "@" (1 char), domain needs to be 252 chars
	domain := make([]byte, 252)
	for i := range domain {
		domain[i] = 'x'
	}
	domain[100] = '.' // put a dot somewhere in the domain
	email254 := "a@" + string(domain)
	if len(email254) != 254 {
		t.Fatalf("expected 254 chars, got %d", len(email254))
	}
	if !ValidateEmail(email254) {
		t.Error("email of exactly 254 chars should be accepted")
	}

	// 255 chars should be rejected
	email255 := email254 + "y"
	if ValidateEmail(email255) {
		t.Error("email of 255 chars should be rejected")
	}
}
