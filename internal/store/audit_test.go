package store

import (
	"context"
	"testing"
)

func TestAuditRecordAndList(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	if err := st.RecordAudit(ctx, "create", "source", "src-1", "req-1"); err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := st.RecordAudit(ctx, "delete", "output", "out-1", "req-2"); err != nil {
		t.Fatalf("record: %v", err)
	}

	rows, err := st.ListAudit(ctx, 10, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) < 2 {
		t.Fatalf("rows = %d, want >= 2", len(rows))
	}

	last := rows[0]
	if last.Action != "delete" || last.Resource != "output" ||
		last.ResourceID != "out-1" || last.RequestID != "req-2" {
		t.Fatalf("unexpected latest row: %+v", last)
	}

	paged, err := st.ListAudit(ctx, 1, 0)
	if err != nil {
		t.Fatalf("list paged: %v", err)
	}
	if len(paged) != 1 {
		t.Fatalf("paged rows = %d, want 1", len(paged))
	}
}
