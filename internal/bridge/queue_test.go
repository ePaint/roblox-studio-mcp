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
		Tool: "execute_luau",
		Args: json.RawMessage(`{"code":"print('hello')"}`),
	}

	resChan := q.Enqueue(job)

	// Verify next retrieves the job
	got := q.Next(100 * time.Millisecond)
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
