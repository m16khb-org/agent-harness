package sqlstore

import (
	"context"
	"database/sql"
	"sync"
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
		benchmarkSpanLockHandoff(b, func(database *DB, afterContention func()) (*sql.Tx, bool, error) {
			return database.beginSpanTxAfterContention(context.Background(), afterContention)
		})
	})
	b.Run("fixed_10ms_baseline", func(b *testing.B) {
		benchmarkSpanLockHandoff(b, func(database *DB, afterContention func()) (*sql.Tx, bool, error) {
			return beginSpanTxFixedGap(
				context.Background(),
				database,
				10*time.Millisecond,
				afterContention,
			)
		})
	})
}

func benchmarkSpanLockHandoff(
	b *testing.B,
	acquire func(*DB, func()) (*sql.Tx, bool, error),
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
		release := make(chan struct{})
		released := make(chan struct{})
		go func() {
			<-release
			_ = transaction.Rollback()
			close(released)
		}()
		var releaseOnce sync.Once
		releaseAfterContention := func() {
			releaseOnce.Do(func() {
				close(release)
			})
		}
		started := time.Now()
		acquired, contended, err := acquire(contender, releaseAfterContention)
		total += time.Since(started)
		releaseAfterContention()
		<-released
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
	afterFirstContention func(),
) (*sql.Tx, bool, error) {
	contended := false
	for {
		transaction, err := database.span.BeginTx(ctx, nil)
		if err == nil {
			return transaction, contended, nil
		}
		if !isSQLiteLockContention(err) {
			return nil, contended, err
		}
		if !contended {
			contended = true
			afterFirstContention()
		}
		timer := time.NewTimer(gap)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, contended, ctx.Err()
		case <-timer.C:
		}
	}
}
