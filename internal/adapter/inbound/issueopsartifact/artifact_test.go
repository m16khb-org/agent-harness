package issueopsartifact

import (
	"context"
	"strings"
	"testing"

	issueopsartifactapplication "issueops/internal/application/issueopsartifact"
	issueopsartifactcontract "issueops/internal/contract/issueopsartifact"
)

type fakeRepository struct {
	updateCalls int
	readCalls   int
	record      issueopsartifactcontract.Record
	staged      issueopsartifactcontract.Staged
}

func (repo *fakeRepository) Update(
	_ context.Context,
	_ string,
	_ string,
	mutate func(issueopsartifactcontract.Record, issueopsartifactcontract.Staged) (issueopsartifactcontract.Staged, error),
) (issueopsartifactcontract.Record, error) {
	repo.updateCalls++
	next, err := mutate(repo.record, repo.staged)
	if err != nil {
		return repo.record, err
	}
	repo.staged = next
	return repo.record, nil
}

func (repo *fakeRepository) ReadStatedForTest() issueopsartifactcontract.Staged {
	return repo.staged
}

func (repo *fakeRepository) ReadStaged(context.Context, string, string) (issueopsartifactcontract.Staged, error) {
	repo.readCalls++
	return repo.staged, nil
}

func newTestHandlers() (Handlers, *fakeRepository) {
	repo := &fakeRepository{staged: issueopsartifactcontract.Staged{}}
	return NewHandlers(issueopsartifactapplication.NewService(repo)), repo
}

func TestHandlersStageNormalizesAndDelegatesToRepository(t *testing.T) {
	handlers, repo := newTestHandlers()

	record, err := handlers.Stage("/state", "io-1", "  plan  ", []byte("# plan body"))
	if err != nil {
		t.Fatalf("stage failed: %v", err)
	}
	if repo.updateCalls != 1 {
		t.Fatalf("update calls = %d, want 1", repo.updateCalls)
	}
	if repo.staged["plan"] != "# plan body" {
		t.Fatalf("staged content = %q", repo.staged["plan"])
	}
	if record.ID != "" && record.ID != repo.record.ID {
		t.Fatalf("record id drift: %q", record.ID)
	}
}

func TestHandlersStageRejectsInvalidNameWithoutRepositoryWrite(t *testing.T) {
	handlers, repo := newTestHandlers()

	if _, err := handlers.Stage("/state", "io-1", "../escape", []byte("# x")); err == nil {
		t.Fatal("invalid artifact name must fail")
	} else if !strings.Contains(err.Error(), "plan|spec|verified-execution-loop") {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("repository must not be written on validation failure, got %d calls", repo.updateCalls)
	}
}

func TestHandlersStageRejectsEmptyContent(t *testing.T) {
	handlers, repo := newTestHandlers()

	if _, err := handlers.Stage("/state", "io-1", "plan", nil); err == nil {
		t.Fatal("empty content must fail")
	}
	if repo.updateCalls != 0 {
		t.Fatalf("repository must not be written on empty content, got %d calls", repo.updateCalls)
	}
}

func TestHandlersNamesListsStagedArtifacts(t *testing.T) {
	handlers, repo := newTestHandlers()
	repo.staged["spec"] = "spec body"
	repo.staged["plan"] = "plan body"

	names, err := handlers.Names("/state", "io-1")
	if err != nil {
		t.Fatalf("names failed: %v", err)
	}
	got := map[string]bool{}
	for _, name := range names {
		got[name] = true
	}
	if !got["plan"] || !got["spec"] || len(names) != 2 {
		t.Fatalf("names = %v, want exactly plan and spec", names)
	}
}

func TestHandlersUnstageRemovesNamedArtifact(t *testing.T) {
	handlers, repo := newTestHandlers()
	repo.staged["plan"] = "plan body"

	if _, err := handlers.Unstage("/state", "io-1", "plan"); err != nil {
		t.Fatalf("unstage failed: %v", err)
	}
	if _, ok := repo.staged["plan"]; ok {
		t.Fatalf("plan still staged after unstage: %v", repo.staged)
	}
}

func TestNewServiceWithoutRepositoryFailsClosed(t *testing.T) {
	handlers := NewHandlers(issueopsartifactapplication.NewService(nil))

	if _, err := handlers.Stage("/state", "io-1", "plan", []byte("# x")); err == nil ||
		!strings.Contains(err.Error(), "repository is required") {
		t.Fatalf("nil repository must fail closed, got %v", err)
	}
}
