package core

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func acquireDraftWikiQueueLock(projectStateDir string) (func(), bool, error) {
	if err := os.MkdirAll(projectStateDir, 0o700); err != nil {
		return nil, false, err
	}
	path := filepath.Join(projectStateDir, draftWikiQueueLockFile)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if os.IsExist(err) {
		return func() {}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	_, writeErr := fmt.Fprintf(f, "%d %s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		if writeErr != nil {
			return nil, false, writeErr
		}
		return nil, false, closeErr
	}
	return func() { _ = os.Remove(path) }, true, nil
}
