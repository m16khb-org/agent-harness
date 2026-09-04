package issueopsartifact

import (
	"context"
	"reflect"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestServiceStagesListsAndUnstagesArtifacts(t *testing.T) {
	repository := &fakeRepository{
		record: issueopscontract.IssueOpsRecord{OK: true, ID: "io-artifact01"},
		staged: map[string]string{},
	}
	service := NewService(repository)

	if _, err := service.Stage(
		context.Background(),
		"/state",
		"io-artifact01",
		"spec",
		[]byte("# Spec\n"),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Stage(
		context.Background(),
		"/state",
		"io-artifact01",
		"plan",
		[]byte("# Plan\n"),
	); err != nil {
		t.Fatal(err)
	}
	names, err := service.Names(context.Background(), "/state", "io-artifact01")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(names, []string{"plan", "spec"}) {
		t.Fatalf("staged names are not sorted: %v", names)
	}

	if _, err := service.Unstage(
		context.Background(),
		"/state",
		"io-artifact01",
		"plan",
	); err != nil {
		t.Fatal(err)
	}
	if _, exists := repository.staged["plan"]; exists {
		t.Fatalf("plan remained staged: %v", repository.staged)
	}
}

type fakeRepository struct {
	record issueopscontract.IssueOpsRecord
	staged map[string]string
}

func (repository *fakeRepository) Update(
	_ context.Context,
	_ string,
	_ string,
	mutate func(
		issueopscontract.IssueOpsRecord,
		map[string]string,
	) (map[string]string, error),
) (issueopscontract.IssueOpsRecord, error) {
	staged, err := mutate(repository.record, cloneMap(repository.staged))
	if err != nil {
		return issueopscontract.IssueOpsRecord{OK: false}, err
	}
	repository.staged = staged
	return repository.record, nil
}

func (repository *fakeRepository) ReadStaged(context.Context, string, string) (map[string]string, error) {
	return cloneMap(repository.staged), nil
}

func cloneMap(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
