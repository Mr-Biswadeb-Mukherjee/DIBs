package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestComputePriorityScore(t *testing.T) {
	now := time.Now()
	s1 := computePriorityScore(High, now)
	s2 := computePriorityScore(Low, now)

	require.Less(t, s1, s2, "High priority should produce larger negative base → more priority")
}

func TestEncodeDecodeResult(t *testing.T) {
	orig := WorkerResult{
		Result:   "x",
		Info:     []string{"i1"},
		Warnings: []string{"w1"},
		Errors:   []error{nil},
	}

	s, err := encodeResult(orig)
	require.NoError(t, err)

	decoded, err := decodeResult(s)
	require.NoError(t, err)
	require.Equal(t, orig.Result, decoded.Result)
	require.Equal(t, orig.Info, decoded.Info)
	require.Equal(t, orig.Warnings, decoded.Warnings)
}

func TestClampLoad(t *testing.T) {
	require.Equal(t, 0, clampLoad(-5))
	require.Equal(t, 3, clampLoad(3))
}

func TestLogFormat(t *testing.T) {
	msg := logFormat("PREFIX", "hello")
	require.Equal(t, "PREFIX: hello", msg)
}

func TestComputeDedupeKey(t *testing.T) {
	keyFn := func(f TaskFunc) string { return "abc" }
	opts := &RunOptions{TaskKey: keyFn}

	require.Equal(t, "abc", computeDedupeKey(opts, nil))

	// nil opts => empty
	require.Equal(t, "", computeDedupeKey(nil, nil))
}

func TestReadWithTimeout_Success(t *testing.T) {
	fn := func(ctx context.Context) (string, error) {
		return "ok", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	val, err := readWithTimeout(ctx, fn)
	require.NoError(t, err)
	require.Equal(t, "ok", val)
}

func TestReadWithTimeout_Timeout(t *testing.T) {
	fn := func(ctx context.Context) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := readWithTimeout(ctx, fn)
	require.Error(t, err)
}

func TestExecuteWithRetries_SuccessAfterRetry(t *testing.T) {
	attempts := 0
	tfn := func(ctx context.Context) (interface{}, []string, []string, []error) {
		attempts++
		if attempts < 3 {
			return nil, nil, nil, []error{errors.New("err")}
		}
		return "ok", nil, nil, nil
	}

	cfg := retryConfig{
		MaxRetries: 5,
		Timeout:    50 * time.Millisecond,
	}

	task := &Task{Func: tfn}
	ctx := context.Background()

	res := executeWithRetries(ctx, task, cfg)
	require.Equal(t, "ok", res.Result)
	require.GreaterOrEqual(t, attempts, 3)
}

func TestExecuteWithRetries_AllFail(t *testing.T) {
	tfn := func(ctx context.Context) (interface{}, []string, []string, []error) {
		return nil, nil, nil, []error{errors.New("fail")}
	}

	cfg := retryConfig{
		MaxRetries: 2,
		Timeout:    10 * time.Millisecond,
	}

	task := &Task{Func: tfn}
	ctx := context.Background()

	res := executeWithRetries(ctx, task, cfg)
	require.Len(t, res.Errors, 1)
	require.Error(t, res.Errors[0])
}
