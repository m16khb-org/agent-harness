package issueopsprovenance

import (
	"context"
	"errors"
	"strings"
	"testing"

	commandparsecontract "agent-harness/internal/contract/commandparse"
	provenanceport "agent-harness/internal/port/issueopsprovenance"
)

type stubObserver struct {
	receipt provenanceport.Receipt
	err     error
}

func (s stubObserver) Observe(context.Context) (provenanceport.Receipt, error) {
	return s.receipt, s.err
}

// Bind/BindMany는 생성된 명령에 실행 정체性 증명을 심는다. 빈 명령 리스트는
// 관측 없이 통과하고, 관측 실패는 typed 에러로, 성공은 모든 비어있지 않은
// 명령에 증거를 바인딩해야 한다.
func TestBindManyObservesAndBindsAllNonEmptyCommands(t *testing.T) {
	observer := stubObserver{receipt: provenanceport.Receipt{
		ExecutablePath: "/repo/bin/agent-harness", ExecutableSHA256: strings.Repeat("a", 64),
	}}
	bound, err := BindMany(context.Background(), []string{"agent-harness issueops status --id io-1", "", "agent-harness issueops list"}, 3, observer)
	if err != nil {
		t.Fatal(err)
	}
	if bound[1] != "" {
		t.Fatalf("empty command must stay empty: %q", bound[1])
	}
	for _, command := range []string{bound[0], bound[2]} {
		if !strings.Contains(command, "issueops") || !strings.Contains(command, "--generated-for-generation 3") {
			t.Fatalf("command not bound with provenance: %q", command)
		}
		if !strings.Contains(command, "/repo/bin/agent-harness") {
			t.Fatalf("bound command must carry the observed executable path: %q", command)
		}
	}
}

func TestBindManyEmptyInputSkipsObservation(t *testing.T) {
	bound, err := BindMany(context.Background(), []string{"", ""}, 1, nil)
	if err != nil || len(bound) != 2 || bound[0] != "" {
		t.Fatalf("empty commands must pass without an observer: %#v err=%v", bound, err)
	}
	if _, err := Bind(context.Background(), "", 1, nil); err != nil || bound == nil {
		t.Fatalf("single empty bind must pass: %v", err)
	}
}

func TestBindManyFailsClosedOnObserverProblems(t *testing.T) {
	if _, err := BindMany(context.Background(), []string{"agent-harness issueops status"}, 1, nil); err == nil {
		t.Fatal("nil observer must fail closed")
	} else {
		var obsErr *commandparsecontract.GeneratedCommandProvenanceError
		if !errors.As(err, &obsErr) || obsErr.Code != "generated_command_provenance_observation_failed" {
			t.Fatalf("observer failure must be typed observation error, got %T: %v", err, err)
		}
	}
	failing := stubObserver{err: errors.New("hash failed")}
	if _, err := BindMany(context.Background(), []string{"agent-harness issueops status"}, 1, failing); err == nil {
		t.Fatal("observer error must propagate")
	}
}
