package issueopsrecord

import (
	"context"
	"fmt"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	issueopscontract "issueops/internal/contract/issueops"
)

func BenchmarkStoreInventory(b *testing.B) {
	for _, count := range []int{1, 100, 1000} {
		b.Run(fmt.Sprintf("records_%d", count), func(b *testing.B) {
			stateRoot := benchmarkInventoryStateRoot(b, count)
			store := Store{}
			b.Run("legacy_list_plus_reads", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					ids, err := store.ListIDs(context.Background(), stateRoot)
					if err != nil {
						b.Fatal(err)
					}
					for _, id := range ids {
						if _, err := store.Read(context.Background(), stateRoot, id); err != nil {
							b.Fatal(err)
						}
					}
				}
			})
			b.Run("bulk_scan", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					records, diagnostics, err := store.Scan(context.Background(), stateRoot)
					if err != nil {
						b.Fatal(err)
					}
					if len(records) != count || len(diagnostics) != 0 {
						b.Fatalf("records=%d diagnostics=%d", len(records), len(diagnostics))
					}
				}
			})
		})
	}
}

func benchmarkInventoryStateRoot(b *testing.B, count int) string {
	b.Helper()
	stateRoot := b.TempDir()
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		b.Fatal(err)
	}
	for index := range count {
		id := fmt.Sprintf("io-benchmark%04d", index)
		encoded, err := Encode(issueopscontract.IssueOpsRecord{
			SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
			ID:            id,
			Repo:          "/repo",
			Phase:         issueopscontract.IssueOpsPhaseProblem,
		})
		if err != nil {
			b.Fatal(err)
		}
		if err := database.Put(Bucket(), id, encoded); err != nil {
			b.Fatal(err)
		}
	}
	return stateRoot
}
