package issueops

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestExecuteExecutionCompleteRequiresHandler(t *testing.T) {
	result, err := ExecuteExecution(context.Background(), t.TempDir(), ExecutionActionRequest{Action: ExecutionActionComplete, ID: "io-complete"}, ExecutionActionDependencies{})
	if !errors.Is(err, ErrCompleteHandlerUnavailable) {
		t.Fatalf("error = %v", err)
	}
	if got, ok := result.(ExecutionResult); !ok || got.OK || got.ID != "io-complete" {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecuteExecutionCompleteDelegatesExactRequest(t *testing.T) {
	request := ExecutionActionRequest{Action: ExecutionActionComplete, ID: "io-complete", Generation: 7, CWD: "/canonical", FinalHead: "head", TuringReportPath: "/canonical/report", Verification: []string{"test"}, RemoteArtifactURL: "https://github.com/acme/repo/pull/7", Confirm: true}
	var gotRoot string
	var got ExecutionCompleteRequest
	result, err := ExecuteExecution(context.Background(), t.TempDir(), request, ExecutionActionDependencies{Complete: func(_ context.Context, stateRoot string, req ExecutionCompleteRequest) (ExecutionResult, error) {
		gotRoot, got = stateRoot, req
		return ExecutionResult{OK: true, ID: req.ID}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if gotRoot == "" {
		t.Fatal("state root was not delegated")
	}
	want := ExecutionCompleteRequest{ID: request.ID, Generation: request.Generation, Actor: request.Actor, CWD: request.CWD, FinalHead: request.FinalHead, TuringReportPath: request.TuringReportPath, Verification: request.Verification, RemoteArtifactURL: request.RemoteArtifactURL, Confirm: request.Confirm}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("request = %+v, want %+v", got, want)
	}
	if output := result.(ExecutionResult); !output.OK || output.ID != request.ID {
		t.Fatalf("result = %+v", output)
	}
}
