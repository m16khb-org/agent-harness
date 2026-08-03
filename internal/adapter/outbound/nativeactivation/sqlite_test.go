package nativeactivation

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
	activationport "agent-harness/internal/port/nativeactivation"
)

func testBackend(executable string) Backend {
	backend := NewBackend(func(root string) (port.TransactionalRecordStore, error) { return sqlstore.Open(root) })
	backend.executable = func() (string, error) { return executable, nil }
	return backend
}

func TestBackendPersistsAndIdempotentlySealsCurrentActivation(t *testing.T) {
	root := t.TempDir()
	harnessRoot := filepath.Join(root, "harness")
	if err := os.MkdirAll(filepath.Join(harnessRoot, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(harnessRoot, "bin", "agent-harness")
	if err := os.WriteFile(target, []byte("current"), 0o700); err != nil {
		t.Fatal(err)
	}
	backend := testBackend(target)
	begin, err := backend.Begin(context.Background(), activationport.BeginRequest{StateRoot: root, HarnessRoot: harnessRoot, TargetBinary: target})
	if err != nil || !begin.Pending || begin.Sealed || begin.BinarySHA256 == "" {
		t.Fatalf("begin=%+v err=%v", begin, err)
	}
	request := activationport.SealRequest{
		StateRoot: root, HarnessRoot: harnessRoot, TargetBinary: target, TransitionID: begin.TransitionID, CatalogSHA256: "catalog",
		Evidence: []activationport.Evidence{{Host: "codex", Surface: "mcp", Path: "/codex/mcp"}},
	}
	first, err := backend.Seal(context.Background(), request)
	if err != nil || first.Pending || !first.Sealed || first.BinarySHA256 != begin.BinarySHA256 {
		t.Fatalf("first seal=%+v err=%v", first, err)
	}
	second, err := backend.Seal(context.Background(), request)
	if err != nil || second != first {
		t.Fatalf("idempotent seal=%+v want=%+v err=%v", second, first, err)
	}
}

func TestBackendSealsStagedCandidateAfterAtomicRename(t *testing.T) {
	root := t.TempDir()
	harnessRoot := filepath.Join(root, "harness")
	binDir := filepath.Join(harnessRoot, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(binDir, ".agent-harness.activate-test")
	target := filepath.Join(binDir, "agent-harness")
	if err := os.WriteFile(stage, []byte("candidate"), 0o700); err != nil {
		t.Fatal(err)
	}
	backend := testBackend(stage)
	begin, err := backend.Begin(context.Background(), activationport.BeginRequest{
		StateRoot: root, HarnessRoot: harnessRoot, TargetBinary: target,
	})
	if err != nil || !begin.Pending {
		t.Fatalf("begin=%+v err=%v", begin, err)
	}
	if err := os.Rename(stage, target); err != nil {
		t.Fatal(err)
	}
	backend.executable = func() (string, error) { return target, nil }
	sealed, err := backend.Seal(context.Background(), activationport.SealRequest{
		StateRoot: root, HarnessRoot: harnessRoot, TargetBinary: target, TransitionID: begin.TransitionID,
	})
	if err != nil || !sealed.Sealed || sealed.BinarySHA256 != begin.BinarySHA256 {
		t.Fatalf("sealed=%+v err=%v", sealed, err)
	}
}

func TestBackendRejectsBinaryDriftBetweenBeginAndSeal(t *testing.T) {
	root := t.TempDir()
	harnessRoot := filepath.Join(root, "harness")
	if err := os.MkdirAll(filepath.Join(harnessRoot, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(harnessRoot, "bin", "agent-harness")
	if err := os.WriteFile(target, []byte("before"), 0o700); err != nil {
		t.Fatal(err)
	}
	backend := testBackend(target)
	begin, err := backend.Begin(context.Background(), activationport.BeginRequest{StateRoot: root, HarnessRoot: harnessRoot, TargetBinary: target})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("after"), 0o700); err != nil {
		t.Fatal(err)
	}
	if result, err := backend.Seal(context.Background(), activationport.SealRequest{StateRoot: root, HarnessRoot: harnessRoot, TargetBinary: target, TransitionID: begin.TransitionID}); err == nil || result.Sealed {
		t.Fatalf("binary drift was accepted: result=%+v err=%v", result, err)
	}
}

func TestBackendAbortDeletesOnlyExactPendingTransition(t *testing.T) {
	root := t.TempDir()
	harnessRoot := filepath.Join(root, "harness")
	if err := os.MkdirAll(filepath.Join(harnessRoot, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(harnessRoot, "bin", "agent-harness")
	if err := os.WriteFile(target, []byte("candidate"), 0o700); err != nil {
		t.Fatal(err)
	}
	backend := testBackend(target)
	begin, err := backend.Begin(context.Background(), activationport.BeginRequest{StateRoot: root, HarnessRoot: harnessRoot, TargetBinary: target})
	if err != nil {
		t.Fatal(err)
	}
	stale := begin.TransitionID
	if stale[0] == '0' {
		stale = "1" + stale[1:]
	} else {
		stale = "0" + stale[1:]
	}
	request := activationport.AbortRequest{StateRoot: root, HarnessRoot: harnessRoot, TargetBinary: target, TransitionID: stale}
	if result, err := backend.Abort(context.Background(), request); err == nil || result.Aborted {
		t.Fatalf("stale abort was accepted: result=%+v err=%v", result, err)
	}
	request.TransitionID = begin.TransitionID
	aborted, err := backend.Abort(context.Background(), request)
	if err != nil || !aborted.Aborted || aborted.Pending || aborted.TransitionID != begin.TransitionID {
		t.Fatalf("abort=%+v err=%v", aborted, err)
	}
	if result, err := backend.Seal(context.Background(), activationport.SealRequest{StateRoot: root, HarnessRoot: harnessRoot, TargetBinary: target, TransitionID: begin.TransitionID}); err == nil || result.Sealed {
		t.Fatalf("aborted transition sealed: result=%+v err=%v", result, err)
	}
}

func TestBackendBeginInvalidatesPriorSealedReceipt(t *testing.T) {
	root := t.TempDir()
	harnessRoot := filepath.Join(root, "harness")
	if err := os.MkdirAll(filepath.Join(harnessRoot, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(harnessRoot, "bin", "agent-harness")
	if err := os.WriteFile(target, []byte("candidate"), 0o700); err != nil {
		t.Fatal(err)
	}
	backend := testBackend(target)
	first, err := backend.Begin(context.Background(), activationport.BeginRequest{StateRoot: root, HarnessRoot: harnessRoot, TargetBinary: target})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backend.Seal(context.Background(), activationport.SealRequest{StateRoot: root, HarnessRoot: harnessRoot, TargetBinary: target, TransitionID: first.TransitionID}); err != nil {
		t.Fatal(err)
	}
	second, err := backend.Begin(context.Background(), activationport.BeginRequest{StateRoot: root, HarnessRoot: harnessRoot, TargetBinary: target})
	if err != nil || second.TransitionID == first.TransitionID {
		t.Fatalf("second begin=%+v err=%v", second, err)
	}
	if _, err := backend.Abort(context.Background(), activationport.AbortRequest{StateRoot: root, HarnessRoot: harnessRoot, TargetBinary: target, TransitionID: second.TransitionID}); err != nil {
		t.Fatal(err)
	}
	if result, err := backend.Seal(context.Background(), activationport.SealRequest{StateRoot: root, HarnessRoot: harnessRoot, TargetBinary: target, TransitionID: first.TransitionID}); err == nil || result.Sealed {
		t.Fatalf("prior receipt remained authoritative: result=%+v err=%v", result, err)
	}
}

func TestBackendRejectsNonCanonicalTarget(t *testing.T) {
	root := t.TempDir()
	harnessRoot := filepath.Join(root, "harness")
	if err := os.MkdirAll(harnessRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(harnessRoot, "agent-harness")
	if err := os.WriteFile(target, []byte("current"), 0o700); err != nil {
		t.Fatal(err)
	}
	result, err := testBackend(target).Begin(context.Background(), activationport.BeginRequest{
		StateRoot: root, HarnessRoot: harnessRoot, TargetBinary: target,
	})
	if err == nil || result.Pending {
		t.Fatalf("non-canonical target was accepted: result=%+v err=%v", result, err)
	}
}

func TestBinaryIdentityRejectsSymbolicLink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("current"), 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if identity, err := binaryIdentityFromPath(link); err == nil || identity != (binaryIdentity{}) {
		t.Fatalf("symbolic link was accepted: identity=%+v err=%v", identity, err)
	}
}
