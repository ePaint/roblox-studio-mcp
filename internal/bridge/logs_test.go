package bridge

import (
	"testing"

	"github.com/ePaint/roblox-studio-mcp/internal/types"
)

func TestLogBuffer_AppendAndQuery(t *testing.T) {
	b := NewLogBuffer(5)

	entries := []types.LogEntry{
		{Message: "test message 1", Level: "output", Source: "edit"},
		{Message: "warning alert", Level: "warning", Source: "edit"},
		{Message: "server error occurred", Level: "error", Source: "server"},
	}

	b.Append(entries)

	// Query all
	got, lastSeq := b.Query(0, "", "", "", 0)
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}
	if lastSeq != 3 {
		t.Fatalf("expected lastSeq 3, got %d", lastSeq)
	}

	// Query with filter
	errLogs, _ := b.Query(0, "error", "", "", 0)
	if len(errLogs) != 1 || errLogs[0].Level != "error" {
		t.Fatalf("expected 1 error log, got %v", errLogs)
	}

	// Query with sinceSeq
	afterOne, _ := b.Query(1, "", "", "", 0)
	if len(afterOne) != 2 {
		t.Fatalf("expected 2 logs after seq 1, got %d", len(afterOne))
	}
}

func TestLogBuffer_RingWrap(t *testing.T) {
	b := NewLogBuffer(3)

	for i := 1; i <= 5; i++ {
		b.Append([]types.LogEntry{{Message: "msg"}})
	}

	got, lastSeq := b.Query(0, "", "", "", 0)
	if len(got) != 3 {
		t.Fatalf("expected 3 entries after ring wrap, got %d", len(got))
	}
	if lastSeq != 5 {
		t.Fatalf("expected lastSeq 5, got %d", lastSeq)
	}
}
