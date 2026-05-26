package vault

import (
	"bytes"
	"strings"
	"testing"
)

const testKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	data := []byte("secret data that should survive encryption and decryption")

	encrypted, err := Encrypt(testKey, data)
	if err != nil {
		t.Fatalf("expected no encrypt error, got %v", err)
	}

	if bytes.Equal(encrypted, data) {
		t.Fatal("expected encrypted data to differ from plaintext")
	}

	decrypted, err := Decrypt(testKey, encrypted)
	if err != nil {
		t.Fatalf("expected no decrypt error, got %v", err)
	}

	if !bytes.Equal(decrypted, data) {
		t.Fatalf("expected decrypted data %q, got %q", data, decrypted)
	}
}

func TestEncryptUsesRandomNonce(t *testing.T) {
	data := []byte("same plaintext")

	first, err := Encrypt(testKey, data)
	if err != nil {
		t.Fatalf("expected no error encrypting first value, got %v", err)
	}

	second, err := Encrypt(testKey, data)
	if err != nil {
		t.Fatalf("expected no error encrypting second value, got %v", err)
	}

	if bytes.Equal(first, second) {
		t.Fatal("expected encrypted values to differ because nonce should be random")
	}
}

func TestEncryptRejectsInvalidKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{
			name: "invalid hex key",
			key:  "not-hex",
		},
		{
			name: "invalid key length",
			key:  "00010203",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Encrypt(tt.key, []byte("data"))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if len(got) != 0 {
				t.Fatalf("expected empty result, got %v", got)
			}
		})
	}
}

func TestDecryptRejectsInvalidKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{
			name: "invalid hex key",
			key:  "not-hex",
		},
		{
			name: "invalid key length",
			key:  "00010203",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decrypt(tt.key, []byte("ciphertext"))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if len(got) != 0 {
				t.Fatalf("expected empty result, got %v", got)
			}
		})
	}
}

func TestDecryptRejectsShortCiphertext(t *testing.T) {
	got, err := Decrypt(testKey, []byte("short"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "cipher too small") {
		t.Fatalf("expected error to contain %q, got %v", "cipher too small", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %v", got)
	}
}

func TestDecryptRejectsTamperedCiphertext(t *testing.T) {
	encrypted, err := Encrypt(testKey, []byte("secret data"))
	if err != nil {
		t.Fatalf("expected no encrypt error, got %v", err)
	}

	encrypted[len(encrypted)-1] ^= 0xff

	got, err := Decrypt(testKey, encrypted)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %v", got)
	}
}
