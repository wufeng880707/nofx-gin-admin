package hash

import (
	"crypto/sha1"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// sha1 hash
func SHA1(b []byte) string {
	h := sha1.New()
	_, _ = h.Write(b)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// sha1 hash
func SHA1String(s string) string {
	return SHA1([]byte(s))
}

// Use bcrypt generate password hash
func GeneratePassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Use bcrypt compare hash password and password
func CompareHashAndPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
