package sqlstore

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestWithSpanContextRejectsNilArguments(t *testing.T) {
	d := openTestDB(t)
	if err := d.WithSpanContext(nil, func(context.Context) error { return nil }); err == nil {
		t.Fatal("nil context was accepted")
	}
	if err := d.WithSpanContext(context.Background(), nil); err == nil {
		t.Fatal("nil callback was accepted")
	}
}

func TestWithSpanContextRejectsActiveRootReentry(t *testing.T) {
	d := openTestDB(t)
	err := d.WithSpanContext(context.Background(), func(spanCtx context.Context) error {
		return d.WithSpanContext(spanCtx, func(context.Context) error { return nil })
	})
	var nested *NestedSpanError
	if !errors.As(err, &nested) || nested.RequestedDir != d.dir || !reflect.DeepEqual(nested.ActiveDirs, []string{d.dir}) {
		t.Fatalf("nested error=%#v err=%v", nested, err)
	}
}

func TestWithSpanContextAllowsDistinctRootsAndRejectsCycle(t *testing.T) {
	a, b := openTestDB(t), openTestDB(t)
	err := a.WithSpanContext(context.Background(), func(aCtx context.Context) error {
		return b.WithSpanContext(aCtx, func(bCtx context.Context) error {
			err := a.WithSpanContext(bCtx, func(context.Context) error { return nil })
			var nested *NestedSpanError
			if !errors.As(err, &nested) || !reflect.DeepEqual(nested.ActiveDirs, []string{a.dir, b.dir}) {
				return fmt.Errorf("cycle error=%#v err=%v", nested, err)
			}
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWithSpanContextCancelsLocalWaiter(t *testing.T) {
	d := openTestDB(t)
	entered, release, holderDone := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		holderDone <- d.WithSpanContext(context.Background(), func(context.Context) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var called atomic.Bool
	started := time.Now()
	err := d.WithSpanContext(ctx, func(context.Context) error {
		called.Store(true)
		return nil
	})
	if !errors.Is(err, context.Canceled) || called.Load() || time.Since(started) >= 2*time.Second {
		t.Fatalf("cancelled local wait: called=%v elapsed=%v err=%v", called.Load(), time.Since(started), err)
	}
	close(release)
	if err := <-holderDone; err != nil {
		t.Fatal(err)
	}
}

func TestWithSpanContextCancelsSQLiteWaiter(t *testing.T) {
	dir := t.TempDir()
	d1, err := newDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := newDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = d1.data.Close()
		_ = d1.span.Close()
		_ = d2.data.Close()
		_ = d2.span.Close()
	})
	entered, release, holderDone := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		holderDone <- d1.WithSpanContext(context.Background(), func(context.Context) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	var called atomic.Bool
	started := time.Now()
	err = d2.WithSpanContext(ctx, func(context.Context) error {
		called.Store(true)
		return nil
	})
	if !errors.Is(err, context.DeadlineExceeded) || called.Load() || time.Since(started) >= 2*time.Second {
		t.Fatalf("cancelled sqlite wait: called=%v elapsed=%v err=%v", called.Load(), time.Since(started), err)
	}
	close(release)
	if err := <-holderDone; err != nil {
		t.Fatal(err)
	}
}

func TestWithSpanContextPreservesCallbackError(t *testing.T) {
	d, want := openTestDB(t), errors.New("sentinel")
	if err := d.WithSpanContext(context.Background(), func(context.Context) error { return want }); err != want {
		t.Fatalf("callback error identity lost: %v", err)
	}
}

func TestWithSpanContextPanicReleasesGate(t *testing.T) {
	d, want := openTestDB(t), errors.New("panic sentinel")
	func() {
		defer func() {
			if got := recover(); got != want {
				t.Fatalf("panic=%v want=%v", got, want)
			}
		}()
		_ = d.WithSpanContext(context.Background(), func(context.Context) error { panic(want) })
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := d.WithSpanContext(ctx, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
}
