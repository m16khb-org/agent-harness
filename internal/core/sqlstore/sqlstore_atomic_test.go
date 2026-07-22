package sqlstore

import (
	"context"
	"sync"
	"testing"
)

func TestApplyMutationsCommitsLifecycleAndLeaseIndexTogether(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Apply(context.Background(), []Mutation{
		{Bucket: "issueops_v1", ID: "io-1", Data: []byte(`{"schema_version":1}`)},
		{Bucket: "lease_holder_v1", ID: "codex-session-1", Data: []byte(`{"lifecycle_id":"io-1","generation":1}`)},
	}); err != nil {
		t.Fatal(err)
	}
	for bucket, id := range map[string]string{
		"issueops_v1":     "io-1",
		"lease_holder_v1": "codex-session-1",
	} {
		if _, ok, err := db.Get(bucket, id); err != nil || !ok {
			t.Fatalf("missing atomic row %s/%s: ok=%v err=%v", bucket, id, ok, err)
		}
	}
	if err := db.Apply(context.Background(), []Mutation{
		{Bucket: "issueops_v1", ID: "io-1", Data: []byte(`{"schema_version":1,"released":true}`)},
		{Bucket: "lease_holder_v1", ID: "codex-session-1", Delete: true},
	}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := db.Get("lease_holder_v1", "codex-session-1"); err != nil || ok {
		t.Fatalf("lease index delete was not committed with the lifecycle row: ok=%v err=%v", ok, err)
	}
}

func TestApplyRequireAbsentIsAtomicUnderConcurrentWriters(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for _, lifecycleID := range []string{"io-1", "io-2"} {
		lifecycleID := lifecycleID
		go func() {
			ready.Done()
			<-start
			errs <- db.Apply(context.Background(), []Mutation{{
				Bucket: "lease_holder_v1", ID: "codex-session-1",
				Data: []byte(`{"lifecycle_id":"` + lifecycleID + `"}`), RequireAbsent: true,
			}})
		}()
	}
	ready.Wait()
	close(start)
	successes := 0
	for range 2 {
		if err := <-errs; err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("require-absent transaction successes=%d want=1", successes)
	}
}
