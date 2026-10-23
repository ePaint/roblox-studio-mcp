package bridge

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ePaint/roblox-studio-mcp/internal/types"
)

func TestJobQueue_EnqueueAndComplete(t *testing.T) {
	q := NewJobQueue(10)

	job := &types.Job{
		ID:   "job-1",
		Role: "edit",
		Tool: "execute_luau",
		Args: json.RawMessage(`{"code":"print('hello')"}`),
	}

	resChan := q.Enqueue(job)

	// Verify next retrieves the job
	got := q.Next("edit", 100*time.Millisecond)
	if got == nil || got.ID != "job-1" {
		t.Fatalf("expected job-1, got %v", got)
	}

	// Complete the job
	expectedRes := &types.JobResult{
		JobID: "job-1",
		OK:    true,
		Value: json.RawMessage(`"hello"`),
	}
	if !q.Complete(expectedRes) {
		t.Fatal("failed to complete job")
	}

	select {
	case res := <-resChan:
		if !res.OK || string(res.Value) != `"hello"` {
			t.Fatalf("unexpected result: %+v", res)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for result")
	}
}

func TestJobQueue_Cancel(t *testing.T) {
	q := NewJobQueue(10)

	job := &types.Job{
		ID:   "job-2",
		Tool: "delete_instance",
	}

	resChan := q.Enqueue(job)
	q.Cancel("job-2", types.ErrTimeout, "timed out")

	select {
	case res := <-resChan:
		if res.OK || res.Error == nil || res.Error.Code != types.ErrTimeout {
			t.Fatalf("expected timeout error, got %+v", res)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for canceled result")
	}
}

func TestJobQueue_NextOnlyReturnsJobsForRequestedRole(t *testing.T) {
	q := NewJobQueue(10)
	q.Enqueue(&types.Job{ID: "edit-job", Role: "edit"})
	q.Enqueue(&types.Job{ID: "server-job", Role: "server"})

	if got := q.Next("server", 0); got == nil || got.ID != "server-job" {
		t.Fatalf("expected server-job, got %v", got)
	}
	if got := q.Next("server", 0); got != nil {
		t.Fatalf("expected no more server jobs, got %v", got.ID)
	}
	if got := q.Next("edit", 0); got == nil || got.ID != "edit-job" {
		t.Fatalf("expected edit-job, got %v", got)
	}
}

func TestJobQueue_NextWaitsForMatchingRole(t *testing.T) {
	q := NewJobQueue(10)
	got := make(chan *types.Job, 1)
	go func() { got <- q.Next("server", time.Second) }()

	time.Sleep(20 * time.Millisecond)
	q.Enqueue(&types.Job{ID: "edit-job", Role: "edit"})
	time.Sleep(20 * time.Millisecond)
	q.Enqueue(&types.Job{ID: "server-job", Role: "server"})

	select {
	case job := <-got:
		if job == nil || job.ID != "server-job" {
			t.Fatalf("expected server-job, got %v", job)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Next did not return")
	}
}

func TestJobQueue_CancelRemovesPendingJob(t *testing.T) {
	q := NewJobQueue(10)
	q.Enqueue(&types.Job{ID: "job-3", Role: "edit"})

	q.Cancel("job-3", types.ErrTimeout, "timed out")

	if got := q.Next("edit", 0); got != nil {
		t.Fatalf("expected canceled job to be gone, got %v", got.ID)
	}
}

func TestJobQueue_RejectsWhenFull(t *testing.T) {
	q := NewJobQueue(1)
	q.Enqueue(&types.Job{ID: "first", Role: "edit"})

	res := <-q.Enqueue(&types.Job{ID: "second", Role: "edit"})

	if res.OK || res.Error == nil || res.Error.Code != types.ErrTooLarge {
		t.Fatalf("expected TOO_LARGE, got %+v", res)
	}
}
