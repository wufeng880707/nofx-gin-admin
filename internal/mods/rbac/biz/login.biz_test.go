package biz

import (
	"crypto/md5" //nolint:gosec // legacy migration test fixture
	"fmt"
	"testing"

	"github.com/LyricTian/gin-admin/v10/pkg/crypto/hash"
)

func TestCompareStoredPasswordSupportsCurrentAndLegacyHashes(t *testing.T) {
	password := "strong-password-123"
	currentHash, err := hash.GeneratePassword(password)
	if err != nil {
		t.Fatal(err)
	}
	legacyDigest := fmt.Sprintf("%x", md5.Sum([]byte(password))) //nolint:gosec
	legacyHash, err := hash.GeneratePassword(legacyDigest)
	if err != nil {
		t.Fatal(err)
	}

	legacy, err := compareStoredPassword(currentHash, password)
	if err != nil || legacy {
		t.Fatalf("current hash mismatch: legacy=%v err=%v", legacy, err)
	}
	legacy, err = compareStoredPassword(legacyHash, password)
	if err != nil || !legacy {
		t.Fatalf("legacy hash mismatch: legacy=%v err=%v", legacy, err)
	}
	if _, err := compareStoredPassword(currentHash, "wrong-password"); err == nil {
		t.Fatal("expected invalid password error")
	}
}
