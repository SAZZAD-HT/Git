package store

import (
	"context"
	"errors"
	"sync"
	"testing"

	"anvil/internal/config"
	"anvil/internal/errs"
)

func openTest(t *testing.T) *DB {
	t.Helper()
	db, err := Open(context.Background(), config.DatabaseConfig{Driver: "sqlite"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// TC-ADM-07: migrations run once, are recorded, and a second run is a no-op.
func TestMigrate(t *testing.T) {
	ctx := context.Background()
	db := openTest(t)
	dry, err := db.Migrate(ctx, true)
	if err != nil || len(dry) == 0 {
		t.Fatalf("dry run: %v %v", dry, err)
	}
	if pending, _ := db.PendingMigrations(ctx); len(pending) != len(dry) {
		t.Fatalf("dry run applied migrations: pending %v", pending)
	}
	applied, err := db.Migrate(ctx, false)
	if err != nil || len(applied) != len(dry) {
		t.Fatalf("applied %v, %v", applied, err)
	}
	again, err := db.Migrate(ctx, false)
	if err != nil || len(again) != 0 {
		t.Fatalf("second run applied %v, %v", again, err)
	}
	if err := db.IntegrityCheck(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownMigrationRefused(t *testing.T) {
	ctx := context.Background()
	db := openTest(t)
	if _, err := db.Migrate(ctx, false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.write.ExecContext(ctx, "INSERT INTO schema_migrations VALUES (9999, 0)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Migrate(ctx, false); err == nil {
		t.Fatal("expected an error for a migration from a newer release")
	}
}

func TestTxRollsBack(t *testing.T) {
	ctx := context.Background()
	db := openTest(t)
	db.Migrate(ctx, false)
	boom := errors.New("boom")
	err := db.Tx(ctx, func(tx *Tx) error {
		if err := tx.SetSetting(ctx, "k", `"v"`); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if _, err := db.Setting(ctx, "k"); !errors.Is(err, errs.NotFound) {
		t.Fatalf("setting survived rollback: %v", err)
	}
}

func TestClaimTaskIsExclusive(t *testing.T) {
	ctx := context.Background()
	db := openTest(t)
	db.Migrate(ctx, false)
	for i := 0; i < 20; i++ {
		db.Tx(ctx, func(tx *Tx) error { _, err := tx.InsertTask(ctx, "k", "{}", 0, 0); return err })
	}
	var mu sync.Mutex
	claimed := map[int64]int{}
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				var task *TaskRow
				err := db.Tx(ctx, func(tx *Tx) error {
					var err error
					task, err = tx.ClaimTask(ctx, []string{"k"}, 100, 200)
					return err
				})
				if errors.Is(err, errs.NotFound) {
					return
				}
				if err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				claimed[task.ID]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(claimed) != 20 {
		t.Fatalf("claimed %d distinct tasks, want 20", len(claimed))
	}
	for id, n := range claimed {
		if n != 1 {
			t.Errorf("task %d claimed %d times", id, n)
		}
	}
}
