package nativeactivation_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	activationapp "agent-harness/internal/application/nativeactivation"
	activationcontract "agent-harness/internal/contract/nativeactivation"
	activationport "agent-harness/internal/port/nativeactivation"
)

const validTimestamp = "2000-01-01T00:00:00.123456789Z"

type backendSpy struct {
	calls       []string
	beginErr    error
	beginResult *activationport.Result
	sealResult  *activationport.Result
}

func (spy *backendSpy) Begin(_ context.Context, request activationport.BeginRequest) (activationport.Result, error) {
	spy.calls = append(spy.calls, "begin")
	if spy.beginErr != nil {
		return activationport.Result{}, spy.beginErr
	}
	if spy.beginResult != nil {
		return *spy.beginResult, nil
	}
	return activationport.Result{StateRoot: request.StateRoot, HarnessRoot: request.HarnessRoot, TargetBinary: request.TargetBinary, BinarySHA256: hash64('c'), Pending: true, UpdatedAt: validTimestamp}, nil
}

func TestServiceNeverReportsOKWhenBeginPersistenceFails(t *testing.T) {
	backend := &backendSpy{beginErr: errors.New("persist failed")}
	service := activationapp.NewService(backend, readbackSpy{calls: &backend.calls})
	result, err := service.Begin(context.Background(), activationcontract.Request{StateRoot: "/state", HarnessRoot: "/harness", TargetBinary: "/harness/bin/agent-harness"})
	if err == nil || result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func (spy *backendSpy) Seal(_ context.Context, request activationport.SealRequest) (activationport.Result, error) {
	spy.calls = append(spy.calls, "seal")
	if spy.sealResult != nil {
		return *spy.sealResult, nil
	}
	return activationport.Result{StateRoot: request.StateRoot, HarnessRoot: request.HarnessRoot, TargetBinary: request.TargetBinary, BinarySHA256: hash64('c'), Sealed: true, UpdatedAt: validTimestamp}, nil
}

type readbackSpy struct {
	calls    *[]string
	evidence *[]activationport.Evidence
	readback *activationport.Readback
}

func (spy readbackSpy) Verify(context.Context, string, string) (activationport.Readback, error) {
	*spy.calls = append(*spy.calls, "readback")
	if spy.readback != nil {
		return *spy.readback, nil
	}
	evidence := validEvidence()
	if spy.evidence != nil {
		*spy.evidence = evidence
	}
	return activationport.Readback{CatalogSHA256: hash64('d'), Evidence: evidence}, nil
}

func TestServiceRejectsUnconfirmedBackendTransitions(t *testing.T) {
	request := activationcontract.Request{StateRoot: "/state", HarnessRoot: "/harness", TargetBinary: "/harness/bin/agent-harness"}
	t.Run("begin not pending", func(t *testing.T) {
		backend := &backendSpy{beginResult: &activationport.Result{StateRoot: request.StateRoot, HarnessRoot: request.HarnessRoot, TargetBinary: request.TargetBinary}}
		service := activationapp.NewService(backend, readbackSpy{calls: &backend.calls})
		if result, err := service.Begin(context.Background(), request); err == nil || result.OK {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})
	t.Run("begin without binary digest", func(t *testing.T) {
		backend := &backendSpy{beginResult: &activationport.Result{StateRoot: request.StateRoot, HarnessRoot: request.HarnessRoot, TargetBinary: request.TargetBinary, Pending: true, UpdatedAt: validTimestamp}}
		service := activationapp.NewService(backend, readbackSpy{calls: &backend.calls})
		if result, err := service.Begin(context.Background(), request); err == nil || result.OK {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})
	t.Run("begin with noncanonical transition timestamp", func(t *testing.T) {
		backend := &backendSpy{beginResult: &activationport.Result{StateRoot: request.StateRoot, HarnessRoot: request.HarnessRoot, TargetBinary: request.TargetBinary, BinarySHA256: hash64('c'), Pending: true, UpdatedAt: "now"}}
		service := activationapp.NewService(backend, readbackSpy{calls: &backend.calls})
		if result, err := service.Begin(context.Background(), request); err == nil || result.OK {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})
	t.Run("begin without transition timestamp", func(t *testing.T) {
		backend := &backendSpy{beginResult: &activationport.Result{StateRoot: request.StateRoot, HarnessRoot: request.HarnessRoot, TargetBinary: request.TargetBinary, BinarySHA256: hash64('c'), Pending: true}}
		service := activationapp.NewService(backend, readbackSpy{calls: &backend.calls})
		if result, err := service.Begin(context.Background(), request); err == nil || result.OK {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})
	t.Run("seal with noncanonical transition timestamp", func(t *testing.T) {
		backend := &backendSpy{sealResult: &activationport.Result{StateRoot: request.StateRoot, HarnessRoot: request.HarnessRoot, TargetBinary: request.TargetBinary, BinarySHA256: hash64('c'), Sealed: true, UpdatedAt: "now"}}
		service := activationapp.NewService(backend, readbackSpy{calls: &backend.calls})
		if result, err := service.Seal(context.Background(), request); err == nil || result.OK {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})
	t.Run("seal not sealed", func(t *testing.T) {
		backend := &backendSpy{sealResult: &activationport.Result{StateRoot: request.StateRoot, HarnessRoot: request.HarnessRoot, TargetBinary: request.TargetBinary}}
		service := activationapp.NewService(backend, readbackSpy{calls: &backend.calls})
		if result, err := service.Seal(context.Background(), request); err == nil || result.OK {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})
	t.Run("seal without transition timestamp", func(t *testing.T) {
		backend := &backendSpy{sealResult: &activationport.Result{StateRoot: request.StateRoot, HarnessRoot: request.HarnessRoot, TargetBinary: request.TargetBinary, BinarySHA256: hash64('c'), Sealed: true}}
		service := activationapp.NewService(backend, readbackSpy{calls: &backend.calls})
		if result, err := service.Seal(context.Background(), request); err == nil || result.OK {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})
	t.Run("identity mismatch", func(t *testing.T) {
		backend := &backendSpy{sealResult: &activationport.Result{StateRoot: "/other", HarnessRoot: request.HarnessRoot, TargetBinary: request.TargetBinary, Sealed: true}}
		service := activationapp.NewService(backend, readbackSpy{calls: &backend.calls})
		if result, err := service.Seal(context.Background(), request); err == nil || result.OK {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})
}

func TestServiceRejectsNoncanonicalReadbackIdentity(t *testing.T) {
	backend := &backendSpy{}
	evidence := validEvidence()
	evidence[0].Host = " codex"
	readback := activationport.Readback{CatalogSHA256: hash64('d'), Evidence: evidence}
	service := activationapp.NewService(backend, readbackSpy{calls: &backend.calls, readback: &readback})
	request := activationcontract.Request{StateRoot: "/state", HarnessRoot: "/harness", TargetBinary: "/harness/bin/agent-harness"}
	if result, err := service.Seal(context.Background(), request); err == nil || result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestServiceBeginOmitsUnsealedReceipt(t *testing.T) {
	backend := &backendSpy{}
	service := activationapp.NewService(backend, readbackSpy{calls: &backend.calls})
	result, err := service.Begin(context.Background(), activationcontract.Request{StateRoot: "/state", HarnessRoot: "/harness", TargetBinary: "/harness/bin/agent-harness"})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"receipt"`) {
		t.Fatalf("begin result must not synthesize a receipt: %s", encoded)
	}
}

func TestServiceSealDoesNotMutateReadbackEvidence(t *testing.T) {
	backend := &backendSpy{}
	var evidence []activationport.Evidence
	service := activationapp.NewService(backend, readbackSpy{calls: &backend.calls, evidence: &evidence})
	request := activationcontract.Request{StateRoot: "/state", HarnessRoot: "/harness", TargetBinary: "/harness/bin/agent-harness"}
	if _, err := service.Seal(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(evidence))
	for _, item := range evidence {
		got = append(got, item.Host+"/"+item.Surface)
	}
	want := []string{"codex/mcp", "claude/hooks", "codex/hooks", "claude/mcp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readback evidence was mutated: got %v want %v", got, want)
	}
}

func TestServiceSealsOnlyAfterFourSurfaceReadback(t *testing.T) {
	backend := &backendSpy{}
	service := activationapp.NewService(backend, readbackSpy{calls: &backend.calls})
	request := activationcontract.Request{StateRoot: "/state", HarnessRoot: "/harness", TargetBinary: "/harness/bin/agent-harness"}
	if _, err := service.Begin(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	result, err := service.Seal(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Sealed || !reflect.DeepEqual(backend.calls, []string{"begin", "readback", "seal"}) || len(result.Receipt.Evidence) != 4 {
		t.Fatalf("write-last activation drift: result=%+v calls=%v", result, backend.calls)
	}
}

func hash64(character byte) string {
	value := make([]byte, 64)
	for index := range value {
		value[index] = character
	}
	return string(value)
}

func validEvidence() []activationport.Evidence {
	evidence := make([]activationport.Evidence, 0, 4)
	for _, item := range [][2]string{{"codex", "mcp"}, {"claude", "hooks"}, {"codex", "hooks"}, {"claude", "mcp"}} {
		evidence = append(evidence, activationport.Evidence{Host: item[0], Surface: item[1], Path: "/" + item[0] + "/" + item[1], SemanticSHA256: hash64('a'), SHA256: hash64('b'), Mode: 0o100600, Size: 1, Device: 1, Inode: 1})
	}
	return evidence
}
