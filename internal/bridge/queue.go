package bridge

import (
	"sync"
	"time"

	"github.com/ePaint/roblox-studio-mcp/internal/types"
)

type JobQueue struct {
	mu       sync.Mutex
	pending  []*types.Job
	capacity int
	arrived  chan struct{}
	results  map[string]chan *types.JobResult
	seqCount int64
}

func NewJobQueue(bufferSize int) *JobQueue {
	if bufferSize <= 0 {
		bufferSize = 64
	}
	return &JobQueue{
		capacity: bufferSize,
		arrived:  make(chan struct{}),
		results:  make(map[string]chan *types.JobResult),
	}
}

func (q *JobQueue) Enqueue(job *types.Job) <-chan *types.JobResult {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.seqCount++
	job.Seq = q.seqCount

	resChan := make(chan *types.JobResult, 1)
	if len(q.pending) >= q.capacity {
		resChan <- &types.JobResult{
			JobID: job.ID,
			OK:    false,
			Error: &types.ErrorInfo{
				Code:    types.ErrTooLarge,
				Message: "bridge job queue is full",
			},
		}
		return resChan
	}

	q.results[job.ID] = resChan
	q.pending = append(q.pending, job)
	close(q.arrived)
	q.arrived = make(chan struct{})
	return resChan
}

// Next returns the oldest pending job for role, waiting up to timeout for one to arrive.
func (q *JobQueue) Next(role string, timeout time.Duration) *types.Job {
	deadline := time.After(timeout)
	for {
		q.mu.Lock()
		job := q.takeLocked(role)
		arrived := q.arrived
		q.mu.Unlock()

		if job != nil {
			return job
		}
		if timeout <= 0 {
			return nil
		}

		select {
		case <-arrived:
		case <-deadline:
			return nil
		}
	}
}

func (q *JobQueue) takeLocked(role string) *types.Job {
	for i, job := range q.pending {
		if job.Role != role {
			continue
		}
		q.pending = append(q.pending[:i], q.pending[i+1:]...)
		return job
	}
	return nil
}

func (q *JobQueue) Complete(result *types.JobResult) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	resChan, ok := q.results[result.JobID]
	if !ok {
		return false
	}

	delete(q.results, result.JobID)
	resChan <- result
	return true
}

func (q *JobQueue) Cancel(jobID string, code string, msg string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, job := range q.pending {
		if job.ID == jobID {
			q.pending = append(q.pending[:i], q.pending[i+1:]...)
			break
		}
	}

	resChan, ok := q.results[jobID]
	if !ok {
		return
	}

	delete(q.results, jobID)
	resChan <- &types.JobResult{
		JobID: jobID,
		OK:    false,
		Error: &types.ErrorInfo{
			Code:    code,
			Message: msg,
		},
	}
}
