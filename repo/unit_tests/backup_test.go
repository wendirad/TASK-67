package unit_tests

import (
	"bytes"
	"crypto/rand"
	"testing"

	"campusrec/internal/services"
)

func TestAESGCMEncryptDecryptRoundtrip(t *testing.T) {
	key := "test-encryption-key-for-backups"
	plaintext := []byte("This is a test backup payload with some SQL data inside it.")

	encrypted, err := services.AESGCMEncrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if bytes.Equal(encrypted, plaintext) {
		t.Fatal("Encrypted data should differ from plaintext")
	}

	decrypted, err := services.AESGCMDecrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypted data does not match original: got %q, want %q", decrypted, plaintext)
	}
}

func TestAESGCMDecryptWrongKey(t *testing.T) {
	plaintext := []byte("sensitive backup data")
	encrypted, err := services.AESGCMEncrypt(plaintext, "correct-key")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = services.AESGCMDecrypt(encrypted, "wrong-key")
	if err == nil {
		t.Fatal("Expected decryption with wrong key to fail")
	}
}

func TestAESGCMEncryptNonDeterministic(t *testing.T) {
	key := "test-key"
	plaintext := []byte("same data")

	enc1, err := services.AESGCMEncrypt(plaintext, key)
	if err != nil {
		t.Fatalf("First encrypt failed: %v", err)
	}

	enc2, err := services.AESGCMEncrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Second encrypt failed: %v", err)
	}

	if bytes.Equal(enc1, enc2) {
		t.Error("Two encryptions of same data should produce different ciphertext (random nonce)")
	}
}

func TestAESGCMDecryptTooShort(t *testing.T) {
	_, err := services.AESGCMDecrypt([]byte("short"), "key")
	if err == nil {
		t.Fatal("Expected error for ciphertext shorter than nonce")
	}
}

func TestAESGCMEncryptLargePayload(t *testing.T) {
	key := "backup-encryption-key"
	// Simulate a 1MB backup
	plaintext := make([]byte, 1024*1024)
	if _, err := rand.Read(plaintext); err != nil {
		t.Fatalf("Failed to generate random data: %v", err)
	}

	encrypted, err := services.AESGCMEncrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := services.AESGCMDecrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Error("Large payload roundtrip failed")
	}
}

func TestAESGCMEmptyPayload(t *testing.T) {
	key := "key"
	plaintext := []byte{}

	encrypted, err := services.AESGCMEncrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt empty failed: %v", err)
	}

	decrypted, err := services.AESGCMDecrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt empty failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Error("Empty payload roundtrip failed")
	}
}
