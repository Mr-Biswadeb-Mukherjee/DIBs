package worker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTask(id int64, p TaskPriority, created time.Time) *Task {
	return &Task{
		ID:       id,
		Priority: p,
		Created:  created,
	}
}

func TestPriorityQueue_OrderByPriority(t *testing.T) {
	now := time.Now()

	t1 := newTask(1, Low, now)
	t2 := newTask(2, Medium, now)
	t3 := newTask(3, High, now)

	pq := PriorityQueue{}
	pq.Push(t1)
	pq.Push(t2)
	pq.Push(t3)

	require.Equal(t, 3, pq.Len())
	require.True(t, pq.Less(2, 0)) // High > Low
	require.True(t, pq.Less(2, 1)) // High > Medium
}

func TestPriorityQueue_OrderByCreatedTimeWhenEqualPriority(t *testing.T) {
	t0 := time.Now()
	t1 := newTask(1, Medium, t0)
	t2 := newTask(2, Medium, t0.Add(1*time.Second))

	pq := PriorityQueue{}
	pq.Push(t1)
	pq.Push(t2)

	require.True(t, pq.Less(0, 1), "earlier timestamp wins when priority equal")
}

func TestPriorityQueue_PushPop(t *testing.T) {
	now := time.Now()
	t1 := newTask(1, Medium, now)
	t2 := newTask(2, High, now)

	pq := PriorityQueue{}
	pq.Push(t1)
	pq.Push(t2)

	require.Equal(t, 2, pq.Len())

	// Pop removes highest priority first
	x := pq.Pop().(*Task)
	require.Equal(t, int64(2), x.ID)
	require.Equal(t, 1, pq.Len())

	y := pq.Pop().(*Task)
	require.Equal(t, int64(1), y.ID)
	require.Equal(t, 0, pq.Len())
}

func TestPriorityQueue_SwapUpdatesIndex(t *testing.T) {
	now := time.Now()
	t1 := newTask(10, Low, now)
	t2 := newTask(20, High, now)

	pq := PriorityQueue{t1, t2}
	t1.index = 0
	t2.index = 1

	pq.Swap(0, 1)

	require.Equal(t, pq[0].ID, int64(20))
	require.Equal(t, pq[1].ID, int64(10))
	require.Equal(t, pq[0].index, 0)
	require.Equal(t, pq[1].index, 1)
}
