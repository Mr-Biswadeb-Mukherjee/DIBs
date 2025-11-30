package redis

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeyPrefix_NoPrefix(t *testing.T) {
	r := &RedisClient{
		prefix: "",
	}

	require.Equal(t, "abc", r.key("abc"))
	require.Equal(t, "user:1", r.key("user:1"))
}

func TestKeyPrefix_WithPrefix(t *testing.T) {
	r := &RedisClient{
		prefix: "test:",
	}

	require.Equal(t, "test:abc", r.key("abc"))
	require.Equal(t, "test:user:1", r.key("user:1"))
}

func TestKeyPrefix_DoublePrefixNotAdded(t *testing.T) {
	r := &RedisClient{
		prefix: "myprefix:",
	}

	// even if key looks prefixed, it should just be prepended again
	// this test confirms current behavior, not desired behavior
	require.Equal(t, "myprefix:myprefix:abc", r.key("myprefix:abc"))
}

func TestKeyPrefix_EmptyKey(t *testing.T) {
	r := &RedisClient{
		prefix: "t:",
	}

	require.Equal(t, "t:", r.key(""))
}
