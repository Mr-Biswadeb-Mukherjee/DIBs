package redis

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

//
// Fake Eval client
//

type mockEvalClient struct {
	*redis.Client
	script string
	keys   []string
	args   []interface{}
	out    interface{}
	err    error
}

func (m *mockEvalClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	m.script = script
	m.keys = keys
	m.args = args

	cmd := redis.NewCmd(ctx)
	if m.err != nil {
		cmd.SetErr(m.err)
		return cmd
	}

	cmd.SetVal(m.out)
	return cmd
}

func TestEval_AppliesPrefix(t *testing.T) {
	m := &mockEvalClient{out: "ok"}

	rc := &RedisClient{
		client: m,
		cfg:    &RedisConfig{},
		logger: silentLogger{},
		prefix: "pre:",
	}

	res, err := rc.Eval(context.Background(), "return 1", []string{"a", "b"}, 11, 22)
	require.NoError(t, err)
	require.Equal(t, "ok", res)

	// keys should be rewritten
	require.Equal(t, []string{"pre:a", "pre:b"}, m.keys)
	require.Equal(t, "return 1", m.script)
	require.Equal(t, []interface{}{11, 22}, m.args)
}
