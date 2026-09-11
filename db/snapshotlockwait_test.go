package db

import (
	"testing"
	"time"
)

// TestSnapshotLockWaitRecorded checks that a commit blocked behind the DB-wide exclusive lock
// (as a Truncate/Restore holds it) has its wait time attributed to SnapshotLockWait, while an
// uncontended commit records none.
func TestSnapshotLockWaitRecorded(t *testing.T) {
	db, err := Open("")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Uncontended commit: no wait recorded (the TryRLock fast path).
	tx := db.Begin()
	tx.NewClassAd("1.0", mustAd(t, "N = 1"))
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if w := db.OpStats().SnapshotLockWait.Count; w != 0 {
		t.Fatalf("uncontended commit recorded %d waits, want 0", w)
	}

	// Hold the exclusive lock (as Truncate/Restore does), then commit concurrently: the commit
	// must block acquiring the shared lock and record the wait once the exclusive hold releases.
	release := db.lockSnapExclusive()
	done := make(chan error, 1)
	go func() {
		tx := db.Begin()
		tx.NewClassAd("2.0", mustAd(t, "N = 2"))
		done <- tx.Commit()
	}()
	time.Sleep(100 * time.Millisecond) // let the committer reach the blocked RLock
	release()
	if err := <-done; err != nil {
		t.Fatalf("blocked commit failed: %v", err)
	}

	ws := db.OpStats().SnapshotLockWait
	if ws.Count == 0 {
		t.Fatal("a commit blocked behind the exclusive lock recorded no wait")
	}
	if ws.Nanos == 0 {
		t.Error("recorded a wait event but zero wait time")
	}
}
