package secure

import (
	"strings"
	"testing"
)

func TestEncryptDecryptStringRoundTrip(t *testing.T) {
	t.Setenv("PASSWORD", "test-password")
	t.Setenv("GOBAN_SECRET_KEY", "test-secret-key")

	plain := "SESSDATA=abc; bili_jct=def"
	encrypted, err := EncryptString(plain)
	if err != nil {
		t.Fatalf("EncryptString failed: %v", err)
	}
	if encrypted == plain {
		t.Fatal("encrypted value should not equal plaintext")
	}
	if !strings.HasPrefix(encrypted, encryptedPrefix) {
		t.Fatalf("encrypted value should use prefix %q, got %q", encryptedPrefix, encrypted)
	}

	decrypted, err := DecryptString(encrypted)
	if err != nil {
		t.Fatalf("DecryptString failed: %v", err)
	}
	if decrypted != plain {
		t.Fatalf("expected %q, got %q", plain, decrypted)
	}
}

func TestDecryptStringKeepsPlainLegacyValues(t *testing.T) {
	t.Setenv("PASSWORD", "test-password")
	t.Setenv("GOBAN_SECRET_KEY", "test-secret-key")

	value := "legacy-cookie"
	decrypted, err := DecryptString(value)
	if err != nil {
		t.Fatalf("DecryptString failed: %v", err)
	}
	if decrypted != value {
		t.Fatalf("expected legacy value to pass through, got %q", decrypted)
	}
}
