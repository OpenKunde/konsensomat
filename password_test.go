package main

import "testing"

func TestHashPasswordVerifyRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Error("VerifyPassword: expected the original password to verify")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Error("VerifyPassword: expected a wrong password to fail")
	}
}

func TestHashPasswordUsesRandomSalt(t *testing.T) {
	hash1, err := HashPassword("same password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	hash2, err := HashPassword("same password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if hash1 == hash2 {
		t.Error("expected two hashes of the same password to differ (random salt)")
	}
	if !VerifyPassword(hash1, "same password") || !VerifyPassword(hash2, "same password") {
		t.Error("both independently salted hashes should still verify the same password")
	}
}

func TestVerifyPasswordRejectsMalformedHashes(t *testing.T) {
	cases := []string{
		"",
		"not-a-hash-at-all",
		"pbkdf2-sha256$abc$deadbeef$deadbeef",   // non-numeric iterations
		"pbkdf2-sha256$0$deadbeef$deadbeef",     // zero iterations
		"pbkdf2-sha256$-5$deadbeef$deadbeef",    // negative iterations
		"pbkdf2-sha256$1000$zz$deadbeef",        // invalid hex salt
		"pbkdf2-sha256$1000$deadbeef$zz",        // invalid hex hash
		"bcrypt$1000$deadbeef$deadbeef",         // wrong algorithm tag
		"pbkdf2-sha256$1000$deadbeef",           // too few fields
		"pbkdf2-sha256$1000$deadbeef$dead$beef", // too many fields
	}
	for _, c := range cases {
		if VerifyPassword(c, "anything") {
			t.Errorf("VerifyPassword(%q): expected malformed hash to be rejected", c)
		}
	}
}

// TestPBKDF2Deterministic pins down that pbkdf2 is a pure function of its
// inputs - the same password, salt, iteration count and key length must
// always derive the same key, since VerifyPassword's whole security model
// depends on that being true.
func TestPBKDF2Deterministic(t *testing.T) {
	salt := []byte("0123456789abcdef")
	a := pbkdf2([]byte("hunter2"), salt, 1000, 32)
	b := pbkdf2([]byte("hunter2"), salt, 1000, 32)

	if len(a) != 32 {
		t.Fatalf("expected a 32-byte key, got %d bytes", len(a))
	}
	if string(a) != string(b) {
		t.Error("expected identical inputs to derive identical keys")
	}

	c := pbkdf2([]byte("hunter3"), salt, 1000, 32)
	if string(a) == string(c) {
		t.Error("expected different passwords to derive different keys")
	}
}
