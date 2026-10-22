package bridge

import (
	"strings"
	"sync"
	"time"

	"github.com/ePaint/roblox-studio-mcp/internal/types"
)

type LogBuffer struct {
	mu      sync.Mutex
	cond    *sync.Cond
	entries []types.LogEntry
	maxSize int
	seq     int64
}

func NewLogBuffer(maxSize int) *LogBuffer {
	if maxSize <= 0 {
		maxSize = 2000
	}
	lb := &LogBuffer{
		entries: make([]types.LogEntry, 0, maxSize),
		maxSize: maxSize,
	}
	lb.cond = sync.NewCond(&lb.mu)
	return lb
}

func (b *LogBuffer) Append(entries []types.LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now().UnixMilli()
	for _, e := range entries {
		b.seq++
		e.Seq = b.seq
		if e.TS == 0 {
			e.TS = now
		}
		if len(b.entries) >= b.maxSize {
			b.entries = b.entries[1:]
		}
		b.entries = append(b.entries, e)
	}

	b.cond.Broadcast()
}

func (b *LogBuffer) Query(sinceSeq int64, level, source, pattern string, waitTimeout time.Duration) ([]types.LogEntry, int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	filter := func() ([]types.LogEntry, int64) {
		var matches []types.LogEntry
		lastSeq := b.seq
		for _, e := range b.entries {
			if e.Seq <= sinceSeq {
				continue
			}
			if level != "" && !strings.EqualFold(e.Level, level) {
				continue
			}
			if source != "" && !strings.EqualFold(e.Source, source) {
				continue
			}
			if pattern != "" && !strings.Contains(e.Message, pattern) {
				continue
			}
			matches = append(matches, e)
		}
		return matches, lastSeq
	}

	res, lastSeq := filter()
	if len(res) > 0 || waitTimeout <= 0 {
		return res, lastSeq
	}

	// Wait for new logs with timeout
	done := make(chan struct{})
	go func() {
		time.Sleep(waitTimeout)
		b.mu.Lock()
		b.cond.Broadcast()
		b.mu.Unlock()
		close(done)
	}()

	for len(res) == 0 {
		b.cond.Wait()
		select {
		case <-done:
			return res, b.seq
		default:
		}
		res, lastSeq = filter()
	}

	return res, lastSeq
}
