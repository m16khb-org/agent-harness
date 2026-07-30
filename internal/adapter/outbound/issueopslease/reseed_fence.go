package issueopslease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"

	"agent-harness/internal/port"
)

type ReseedFenceStore func(string) (port.TransactionalRecordStore, error)

type SQLiteReseedFence struct {
	root string
	open ReseedFenceStore
}

func NewSQLiteReseedFence(stateRoot string, open ReseedFenceStore) (*SQLiteReseedFence, error) {
	root, err := filepath.Abs(stateRoot)
	if err != nil {
		return nil, err
	}
	if open == nil {
		return nil, fmt.Errorf("reseed fence store opener is required")
	}
	return &SQLiteReseedFence{root: root, open: open}, nil
}

func (f *SQLiteReseedFence) Within(ctx context.Context, id string, fn func(context.Context) error) error {
	sum := sha256.Sum256([]byte(id))
	db, err := f.open(filepath.Join(f.root, "issueops-reseed-fence", hex.EncodeToString(sum[:])))
	if err != nil {
		return err
	}
	return db.WithSpan(ctx, fn)
}
