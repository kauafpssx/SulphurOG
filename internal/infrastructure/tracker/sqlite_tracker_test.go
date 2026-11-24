package tracker

import (
	"path/filepath"
	"testing"

	"github.com/sulphurog/sulphurog/internal/domain"
)

func TestFinishedRecordAndPendingLookup(t *testing.T) {
	tracker, err := NewSQLiteTracker(filepath.Join(t.TempDir(), "tracker.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer tracker.db.Close()

	source := "https://t.me/c/123/45"
	if err := tracker.MarkDownloaded(domain.FileRecord{ContentHash: source, Source: source, MessageID: 45, Filename: "same.rar"}); err != nil {
		t.Fatal(err)
	}
	if err := tracker.MarkFinished(source, "output/ulp.txt", 1); err != nil {
		t.Fatal(err)
	}
	record, err := tracker.GetFileRecord(source)
	if err != nil || record.Status != domain.StatusFinished || record.FailedAt != nil {
		t.Fatalf("finished record: %+v, %v", record, err)
	}

	pendingSource := "https://t.me/c/123/46"
	if err := tracker.AddPending([]domain.PendingFile{{Source: pendingSource, MessageID: 46}}); err != nil {
		t.Fatal(err)
	}
	queued, err := tracker.IsPending(pendingSource)
	if err != nil || !queued {
		t.Fatalf("pending lookup: %v, %v", queued, err)
	}
	queued, err = tracker.IsPending(source)
	if err != nil || queued {
		t.Fatalf("finished lookup: %v, %v", queued, err)
	}
}
