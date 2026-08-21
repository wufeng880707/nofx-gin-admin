package middleware

import (
	"strings"
	"testing"
)

func TestSanitizeLogBodyRedactsSensitiveValues(t *testing.T) {
	body := []byte(`{
		"username":"admin",
		"password":"password-value",
		"data":{
			"access_token":"token-value",
			"api_key":"api-key-value",
			"secret_key":"secret-key-value",
			"private_key_ciphertext":"ciphertext-value",
			"name":"visible-value"
		},
		"items":[{"passphrase":"passphrase-value","provider":"okx"}]
	}`)

	result := sanitizeLogBody(body)
	for _, secret := range []string{
		"password-value",
		"token-value",
		"api-key-value",
		"secret-key-value",
		"ciphertext-value",
		"passphrase-value",
	} {
		if strings.Contains(result, secret) {
			t.Fatalf("sanitized log contains sensitive value %q: %s", secret, result)
		}
	}
	for _, visible := range []string{"admin", "visible-value", "okx"} {
		if !strings.Contains(result, visible) {
			t.Fatalf("sanitized log removed non-sensitive value %q: %s", visible, result)
		}
	}
}

func TestSanitizeLogBodyKeepsNonJSONBody(t *testing.T) {
	const body = "plain text"
	if got := sanitizeLogBody([]byte(body)); got != body {
		t.Fatalf("sanitizeLogBody() = %q, want %q", got, body)
	}
}

func TestSanitizeLogBodyKeepsCredentialPresenceFlags(t *testing.T) {
	result := sanitizeLogBody([]byte(`{"has_private_key":false,"has_api_key_private_key":true,"private_key":"secret"}`))
	for _, visible := range []string{`"has_private_key":false`, `"has_api_key_private_key":true`} {
		if !strings.Contains(result, visible) {
			t.Fatalf("sanitized log removed credential presence flag %q: %s", visible, result)
		}
	}
	if strings.Contains(result, `"private_key":"secret"`) {
		t.Fatalf("sanitized log contains private key: %s", result)
	}
}
