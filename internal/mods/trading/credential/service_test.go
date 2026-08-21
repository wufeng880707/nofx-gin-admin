package credential

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestServiceRoundTripAndAAD(t *testing.T) {
	service := NewService([]byte(strings.Repeat("a", 32)), nil)
	ciphertext, err := service.EncryptField("secret", "user-1", "exchange-1", "api_key")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if !strings.HasPrefix(ciphertext, storagePrefix) || strings.Contains(ciphertext, "secret") {
		t.Fatalf("unexpected ciphertext: %q", ciphertext)
	}
	plaintext, err := service.DecryptField(ciphertext, "user-1", "exchange-1", "api_key")
	if err != nil || plaintext != "secret" {
		t.Fatalf("decrypt = %q, %v", plaintext, err)
	}
	if _, err := service.DecryptField(ciphertext, "user-2", "exchange-1", "api_key"); err == nil {
		t.Fatal("expected AAD mismatch to fail")
	}
}

func TestServiceReadsLegacyNOFXPassphraseFromPreviousKey(t *testing.T) {
	activeKey := []byte(strings.Repeat("n", 32))
	t.Setenv(EnvActiveKey, base64.StdEncoding.EncodeToString(activeKey))
	t.Setenv(EnvPreviousKey, "legacy-passphrase")
	legacyKey := decodeLegacyKey("legacy-passphrase")
	legacyValue := legacyCiphertext(t, legacyKey, "legacy-secret")

	plaintext, err := NewServiceFromEnv().DecryptField(legacyValue, "user", "exchange", "api_key")
	if err != nil || plaintext != "legacy-secret" {
		t.Fatalf("legacy passphrase decrypt = %q, %v", plaintext, err)
	}
}

func TestServiceDecryptsLegacyNOFXPaddedBase64WithoutAAD(t *testing.T) {
	key := []byte(strings.Repeat("l", 32))
	legacyValue := legacyCiphertext(t, key, "legacy-secret")

	plaintext, err := NewService(key, nil).DecryptField(legacyValue, "user", "exchange", "api_key")
	if err != nil || plaintext != "legacy-secret" {
		t.Fatalf("legacy decrypt = %q, %v", plaintext, err)
	}
}

func legacyCiphertext(t *testing.T, key []byte, plaintext string) string {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return storagePrefix + base64.StdEncoding.EncodeToString(nonce) + ":" + base64.StdEncoding.EncodeToString(ciphertext)
}

func TestServiceRotation(t *testing.T) {
	oldKey := []byte(strings.Repeat("o", 32))
	oldService := NewService(oldKey, nil)
	value, err := oldService.EncryptField("secret", "user", "model", "api_key")
	if err != nil {
		t.Fatal(err)
	}
	rotated := NewService([]byte(strings.Repeat("n", 32)), oldKey)
	plaintext, err := rotated.DecryptField(value, "user", "model", "api_key")
	if err != nil || plaintext != "secret" {
		t.Fatalf("rotated decrypt = %q, %v", plaintext, err)
	}
}

func TestServiceRequiresConfiguredKey(t *testing.T) {
	service := NewService(nil, nil)
	_, err := service.EncryptField("secret", "user", "exchange", "api_key")
	if !errors.Is(err, ErrKeyNotConfigured) {
		t.Fatalf("expected ErrKeyNotConfigured, got %v", err)
	}
}
