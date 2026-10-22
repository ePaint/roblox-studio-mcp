package bridge

import (
	"sync"
	"time"

	"github.com/ePaint/roblox-studio-mcp/internal/types"
)

type JobQueue struct {
	mu       sync.Mutex
	pending  chan *types.Job
	results  map[string]chan *types.JobResult
	seqCount int64
}

func NewJobQueue(bufferSize int) *JobQueue {
	if bufferSize <= 0 {
		bufferSize = 64
	}
	return &JobQueue{
		pending: make(chan *types.Job, bufferSize),
		results: make(map[string]chan *types.JobResult),
	}
}

func (q *JobQueue) Enqueue(job *types.Job) <-chan *types.JobResult {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.seqCount++
	job.Seq = q.seqCount

	resChan := make(chan *types.JobResult, 1)
	q.results[job.ID] = resChan

	// Push to pending queue
	select {
	case q.pending <- job:
	default:
		// Queue full, drop immediately
		resChan <- &types.JobResult{
			JobID: job.ID,
			OK:    false,
			Error: &types.ErrorInfo{
				Code:    types.ErrTooLarge,
				Message: "bridge job queue is full",
			},
		}
	}

	return resChan
}

func (q *JobQueue) Next(timeout time.Duration) *types.Job {
	if timeout <= 0 {
		select {
		case job := <-q.pending:
			return job
		default:
			return nil
		}
	}

	select {
	case job := <-q.pending:
		return job
	case <-time.After(timeout):
		return nil
	}
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
