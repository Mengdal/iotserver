package things

import "testing"

func TestEncryptDecryptPassword(t *testing.T) {
	t.Setenv(encryptionKeyEnv, "unit-test-key")
	encrypted, err := EncryptPassword("s3cret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if encrypted == "" || encrypted == "s3cret" {
		t.Fatalf("password was not encrypted: %q", encrypted)
	}
	raw, err := DecryptPassword(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if raw != "s3cret" {
		t.Fatalf("unexpected plaintext: %q", raw)
	}
	if IsMasked(encrypted) {
		t.Fatalf("encrypted value must not be treated as mask")
	}
	if !IsMasked("******") {
		t.Fatalf("mask should be recognized")
	}
}

func TestEncryptEmptyPassword(t *testing.T) {
	t.Setenv(encryptionKeyEnv, "unit-test-key")
	got, err := EncryptPassword("")
	if err != nil || got != "" {
		t.Fatalf("empty password should stay empty: %q %v", got, err)
	}
	got, err = DecryptPassword("")
	if err != nil || got != "" {
		t.Fatalf("empty ciphertext should stay empty: %q %v", got, err)
	}
}
