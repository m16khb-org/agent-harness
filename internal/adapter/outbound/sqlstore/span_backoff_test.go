package sqlstore

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestSpanRetryGapUsesBoundedExponentialBackoff(t *testing.T) {
	gap := spanLockInitialRetryGap
	want := []time.Duration{
		time.Millisecond,
		2 * time.Millisecond,
		4 * time.Millisecond,
		8 * time.Millisecond,
		10 * time.Millisecond,
		10 * time.Millisecond,
	}
	for index, expected := range want {
		if gap != expected {
			t.Fatalf("retry gap %d = %s want %s", index, gap, expected)
		}
		gap = nextSpanRetryGap(gap)
	}
}

func BenchmarkSpanLockShortContentionHandoff(b *testing.B) {
	b.Run("adaptive", func(b *testing.B) {
		benchmarkSpanLockHandoff(b, func(database *DB) (*sql.Tx, bool, error) {
			return database.beginSpanTx(context.Background())
		})
	})
	b.Run("fixed_10ms_baseline", func(b *testing.B) {
		benchmarkSpanLockHandoff(b, func(database *DB) (*sql.Tx, bool, error) {
			transaction, err := beginSpanTxFixedGap(
				context.Background(),
				database,
				10*time.Millisecond,
			)
			return transaction, true, err
		})
	})
}

func benchmarkSpanLockHandoff(
	b *testing.B,
	acquire func(*DB) (*sql.Tx, bool, error),
) {
	b.Helper()
	root := b.TempDir()
	holder, err := newDB(root)
	if err != nil {
		b.Fatal(err)
	}
	defer holder.data.Close()
	defer holder.span.Close()
	contender, err := newDB(root)
	if err != nil {
		b.Fatal(err)
	}
	defer contender.data.Close()
	defer contender.span.Close()

	var total time.Duration
	for b.Loop() {
		transaction, err := holder.span.BeginTx(context.Background(), nil)
		if err != nil {
			b.Fatal(err)
		}
		release := time.AfterFunc(2*time.Millisecond, func() {
			_ = transaction.Rollback()
		})
		started := time.Now()
		acquired, contended, err := acquire(contender)
		total += time.Since(started)
		release.Stop()
		if err != nil {
			b.Fatal(err)
		}
		if !contended {
			b.Fatal("benchmark did not exercise SQLite lock contention")
		}
		_ = acquired.Rollback()
	}
	b.ReportMetric(float64(total.Nanoseconds())/float64(b.N), "handoff_ns/op")
}

func beginSpanTxFixedGap(
	ctx context.Context,
	database *DB,
	gap time.Duration,
) (*sql.Tx, error) {
	for {
		transaction, err := database.span.BeginTx(ctx, nil)
		if err == nil {
			return transaction, nil
		}
		if !isSQLiteLockContention(err) {
			return nil, err
		}
		timer := time.NewTimer(gap)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
