package credential

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	EnvActiveKey   = "DATA_ENCRYPTION_KEY"
	EnvPreviousKey = "DATA_ENCRYPTION_KEY_PREVIOUS"
	storagePrefix  = "ENC:v1:"
)

var (
	ErrKeyNotConfigured  = errors.New("trading credential encryption key is not configured")
	ErrInvalidCiphertext = errors.New("invalid trading credential ciphertext")
)

type keyMaterial struct {
	id  string
	key []byte
}

// Service encrypts credentials with AES-256-GCM and supports one previous key during rotation.
type Service struct {
	active     *keyMaterial
	keyByID    map[string][]byte
	legacyKeys [][]byte
}

func NewServiceFromEnv() *Service {
	active, _ := decodeKey(os.Getenv(EnvActiveKey))
	previous := decodeLegacyKey(os.Getenv(EnvPreviousKey))
	return NewService(active, previous)
}

func NewService(active, previous []byte) *Service {
	service := &Service{keyByID: make(map[string][]byte)}
	if len(active) == 32 {
		material := &keyMaterial{id: keyID(active), key: append([]byte(nil), active...)}
		service.active = material
		service.keyByID[material.id] = material.key
		service.legacyKeys = append(service.legacyKeys, material.key)
	}
	if len(previous) == 16 || len(previous) == 24 || len(previous) == 32 {
		id := keyID(previous)
		service.keyByID[id] = append([]byte(nil), previous...)
		service.legacyKeys = append(service.legacyKeys, service.keyByID[id])
	}
	return service
}

func decodeLegacyKey(value string) []byte {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	decoders := []func(string) ([]byte, error){
		base64.StdEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		hex.DecodeString,
	}
	for _, decoder := range decoders {
		if decoded, err := decoder(value); err == nil && len(decoded) > 0 {
			if len(decoded) == 16 || len(decoded) == 24 || len(decoded) == 32 {
				return decoded
			}
			sum := sha256.Sum256(decoded)
			return sum[:]
		}
	}
	sum := sha256.Sum256([]byte(value))
	return sum[:]
}

func (s *Service) Ready() bool {
	return s != nil && s.active != nil
}

func (s *Service) EncryptField(plaintext, ownerUserID, entityID, field string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if strings.HasPrefix(plaintext, storagePrefix) {
		return plaintext, nil
	}
	if !s.Ready() {
		return "", ErrKeyNotConfigured
	}
	block, err := aes.NewCipher(s.active.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), fieldAAD(ownerUserID, entityID, field))
	return storagePrefix + s.active.id + ":" +
		base64.RawStdEncoding.EncodeToString(nonce) + ":" +
		base64.RawStdEncoding.EncodeToString(ciphertext), nil
}

func (s *Service) DecryptField(value, ownerUserID, entityID, field string) (string, error) {
	if value == "" {
		return "", nil
	}
	if s == nil || len(s.keyByID) == 0 {
		return "", ErrKeyNotConfigured
	}
	if !strings.HasPrefix(value, storagePrefix) {
		return "", ErrInvalidCiphertext
	}
	parts := strings.Split(strings.TrimPrefix(value, storagePrefix), ":")
	switch len(parts) {
	case 3:
		key := s.keyByID[parts[0]]
		if len(key) == 0 {
			return "", fmt.Errorf("%w: unknown key id", ErrInvalidCiphertext)
		}
		return decrypt(key, parts[1], parts[2], fieldAAD(ownerUserID, entityID, field))
	case 2:
		// Compatibility with NOFX ENC:v1:<nonce>:<ciphertext> values.
		for _, key := range s.legacyKeys {
			plaintext, err := decrypt(key, parts[0], parts[1], fieldAAD(ownerUserID, entityID, field))
			if err == nil {
				return plaintext, nil
			}
			// Older NOFX rows may not have field AAD.
			plaintext, err = decrypt(key, parts[0], parts[1], nil)
			if err == nil {
				return plaintext, nil
			}
		}
		return "", fmt.Errorf("%w: authentication failed", ErrInvalidCiphertext)
	default:
		return "", ErrInvalidCiphertext
	}
}

func decrypt(key []byte, nonceValue, ciphertextValue string, aad []byte) (string, error) {
	nonce, err := decodeBase64(nonceValue)
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	ciphertext, err := decodeBase64(ciphertextValue)
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(nonce) != gcm.NonceSize() {
		return "", ErrInvalidCiphertext
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func decodeBase64(value string) ([]byte, error) {
	decoded, err := base64.RawStdEncoding.DecodeString(value)
	if err == nil {
		return decoded, nil
	}
	return base64.StdEncoding.DecodeString(value)
}

func decodeKey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, ErrKeyNotConfigured
	}
	key, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		key, err = base64.RawStdEncoding.DecodeString(value)
	}
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("%s must be a base64 encoded 32-byte key", EnvActiveKey)
	}
	return key, nil
}

func keyID(key []byte) string {
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:6])
}

func fieldAAD(ownerUserID, entityID, field string) []byte {
	return []byte(strings.Join([]string{"nofx", ownerUserID, entityID, field}, "\x00"))
}
