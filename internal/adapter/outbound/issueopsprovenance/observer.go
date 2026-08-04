package issueopsprovenance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	provenanceport "agent-harness/internal/port/issueopsprovenance"
)

type executableObserver struct {
	executable func() (string, error)
}

func NewExecutableObserver() provenanceport.Observer {
	return &executableObserver{executable: os.Executable}
}

func (o *executableObserver) Observe(ctx context.Context) (provenanceport.Receipt, error) {
	if err := ctx.Err(); err != nil {
		return provenanceport.Receipt{}, err
	}
	if o == nil || o.executable == nil {
		return provenanceport.Receipt{}, fmt.Errorf("generated command executable observer is unavailable")
	}
	executable, err := o.executable()
	if err != nil {
		return provenanceport.Receipt{}, fmt.Errorf("observe generated command executable: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return provenanceport.Receipt{}, fmt.Errorf("resolve generated command executable: %w", err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return provenanceport.Receipt{}, fmt.Errorf("canonicalize generated command executable: %w", err)
	}
	file, err := os.Open(canonical)
	if err != nil {
		return provenanceport.Receipt{}, fmt.Errorf("open generated command executable: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return provenanceport.Receipt{}, fmt.Errorf("hash generated command executable: %w", err)
	}
	return provenanceport.Receipt{
		ExecutablePath: canonical, ExecutableSHA256: hex.EncodeToString(hash.Sum(nil)),
	}, nil
}
