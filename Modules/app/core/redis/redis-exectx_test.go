package redis

import (
	"context"
	"errors"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

//
// ===================================================================
//        Pipeline spy: wraps a real redis.Pipeliner
// ===================================================================
//

type spyPipeline struct {
	redis.Pipeliner
	execCalls    int
	discardCalls int
	execErr      error
}

// method signature MUST MATCH Redis Pipeliner EXACTLY
func (s *spyPipeline) Discard() {
	s.discardCalls++
}

func (s *spyPipeline) Exec(ctx context.Context) ([]redis.Cmder, error) {
	s.execCalls++
	if s.execErr != nil {
		return nil, s.execErr
	}
	return []redis.Cmder{}, nil
}

//
// ===================================================================
//        Mock client overriding TxPipeline to return spy
// ===================================================================
//

type mockExecTxClient struct {
	*redis.Client
	pipe *spyPipeline
}

func newMockExecTxClient(pipe *spyPipeline) *mockExecTxClient {
	base := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	return &mockExecTxClient{Client: base, pipe: pipe}
}

// override ONLY TxPipeline
func (m *mockExecTxClient) TxPipeline() redis.Pipeliner {
	return m.pipe
}

//
// ===================================================================
//                           TESTS
// ===================================================================
//

func TestExecTx_FnReturnsError(t *testing.T) {
	pipe := &spyPipeline{}
	client := &RedisClient{
		client: newMockExecTxClient(pipe),
		cfg:    &RedisConfig{},
		logger: silentLogger{},
	}

	err := client.ExecTx(context.Background(), func(p redis.Pipeliner) error {
		return errors.New("boom")
	})

	require.Error(t, err)
	require.Equal(t, 0, pipe.execCalls)
	require.Equal(t, 1, pipe.discardCalls)
}

func TestExecTx_ExecReturnsError(t *testing.T) {
	pipe := &spyPipeline{execErr: errors.New("execfail")}
	client := &RedisClient{
		client: newMockExecTxClient(pipe),
		cfg:    &RedisConfig{},
		logger: silentLogger{},
	}

	err := client.ExecTx(context.Background(), func(p redis.Pipeliner) error {
		return nil
	})

	require.Error(t, err)
	require.Equal(t, 1, pipe.execCalls)
	require.Equal(t, 0, pipe.discardCalls)
}

func TestExecTx_Success(t *testing.T) {
	pipe := &spyPipeline{}
	client := &RedisClient{
		client: newMockExecTxClient(pipe),
		cfg:    &RedisConfig{},
		logger: silentLogger{},
	}

	err := client.ExecTx(context.Background(), func(p redis.Pipeliner) error {
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, 1, pipe.execCalls)
	require.Equal(t, 0, pipe.discardCalls)
}
