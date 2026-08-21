package cachex

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRedisCache(t *testing.T) {
	conn, err := net.DialTimeout("tcp", "localhost:6379", 200*time.Millisecond)
	if err != nil {
		t.Skip("Redis is not available at localhost:6379")
	}
	_ = conn.Close()

	assert := assert.New(t)

	cache := NewRedisCache(RedisConfig{
		Addr: "localhost:6379",
		DB:   1,
	})

	ctx := context.Background()
	err = cache.Set(ctx, "tt", "foo", "bar")
	assert.Nil(err)

	val, exists, err := cache.Get(ctx, "tt", "foo")
	assert.Nil(err)
	assert.True(exists)
	assert.Equal("bar", val)

	err = cache.Delete(ctx, "tt", "foo")
	assert.Nil(err)

	val, exists, err = cache.Get(ctx, "tt", "foo")
	assert.Nil(err)
	assert.False(exists)
	assert.Equal("", val)

	tmap := make(map[string]bool)
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("foo%d", i)
		err = cache.Set(ctx, "tt", key, "bar")
		assert.Nil(err)
		tmap[key] = true

		err = cache.Set(ctx, "ff", key, "bar")
		assert.Nil(err)
	}

	err = cache.Iterator(ctx, "tt", func(ctx context.Context, key, value string) bool {
		assert.True(tmap[key])
		assert.Equal("bar", value)
		return true
	})
	assert.Nil(err)

	err = cache.Close(ctx)
	assert.Nil(err)
}
