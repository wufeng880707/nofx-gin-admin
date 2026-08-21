package jwtx

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestAuth(t *testing.T) {
	cache := NewMemoryCache(MemoryConfig{CleanupInterval: time.Second})

	store := NewStoreWithCache(cache)
	ctx := context.Background()
	jwtAuth := New(store)

	userID := "test"
	token, err := jwtAuth.GenerateToken(ctx, userID)
	assert.Nil(t, err)
	assert.NotNil(t, token)

	id, err := jwtAuth.ParseSubject(ctx, token.GetAccessToken())
	assert.Nil(t, err)
	assert.Equal(t, userID, id)

	err = jwtAuth.DestroyToken(ctx, token.GetAccessToken())
	assert.Nil(t, err)

	id, err = jwtAuth.ParseSubject(ctx, token.GetAccessToken())
	assert.NotNil(t, err)
	assert.EqualError(t, err, ErrInvalidToken.Error())
	assert.Empty(t, id)

	err = jwtAuth.Release(ctx)
	assert.Nil(t, err)
}

func TestSigningKeyRotationAndRevocation(t *testing.T) {
	ctx := context.Background()
	store := NewStoreWithCache(NewMemoryCache(MemoryConfig{CleanupInterval: time.Second}))
	oldKey := "old-signing-key-with-at-least-32-bytes"
	newKey := "new-signing-key-with-at-least-32-bytes"

	oldAuth := New(store, SetSigningKey(oldKey, ""))
	oldToken, err := oldAuth.GenerateToken(ctx, "old-user")
	assert.NoError(t, err)

	rotatedAuth := New(store, SetSigningKey(newKey, oldKey))
	subject, err := rotatedAuth.ParseSubject(ctx, oldToken.GetAccessToken())
	assert.NoError(t, err)
	assert.Equal(t, "old-user", subject)

	newToken, err := rotatedAuth.GenerateToken(ctx, "new-user")
	assert.NoError(t, err)
	_, err = oldAuth.ParseSubject(ctx, newToken.GetAccessToken())
	assert.ErrorIs(t, err, ErrInvalidToken)

	assert.NoError(t, rotatedAuth.DestroyToken(ctx, newToken.GetAccessToken()))
	_, err = rotatedAuth.ParseSubject(ctx, newToken.GetAccessToken())
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestRejectsUnexpectedHMACAlgorithm(t *testing.T) {
	key := "configured-signing-key-with-at-least-32-bytes"
	claims := jwt.RegisteredClaims{
		Subject:   "test-user",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(key))
	assert.NoError(t, err)

	auth := New(nil, SetSigningMethod(jwt.SigningMethodHS512), SetSigningKey(key, ""))
	_, err = auth.ParseSubject(context.Background(), token)
	assert.ErrorIs(t, err, ErrInvalidToken)
}
