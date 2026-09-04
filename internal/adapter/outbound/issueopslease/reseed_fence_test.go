package issueopslease

import (
	"context"
	"sync"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/port"
)

func TestSQLiteReseedFenceSeparatesLifecycles(t *testing.T) {
	fence, err := NewSQLiteReseedFence(t.TempDir(), func(root string) (port.TransactionalRecordStore, error) { return sqlstore.Open(root) })
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan string, 2)
	release := make(chan struct{})
	var group sync.WaitGroup
	for _, id := range []string{"io-one", "io-two"} {
		group.Add(1)
		go func(id string) {
			defer group.Done()
			if err := fence.Within(context.Background(), id, func(context.Context) error { started <- id; <-release; return nil }); err != nil {
				t.Errorf("fence %s: %v", id, err)
			}
		}(id)
	}
	<-started
	<-started
	close(release)
	group.Wait()
}
