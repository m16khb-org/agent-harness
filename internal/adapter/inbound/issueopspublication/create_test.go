package issueopspublication

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"agent-harness/internal/adapter/issueops"
	publicationcontract "agent-harness/internal/contract/issueopspublication"
	"agent-harness/internal/port"
)

func TestCreateHandlerMapsAllPublicFields(t *testing.T) {
	service := &fakeCreateService{
		t: t, expected: true,
		result: publicationcontract.ProviderCreateResult{
			OK: true, URL: "https://github.com/acme/repo/pull/1", Number: "1", Preview: "created preview",
		},
	}
	request := fullCoreCreateRequest()
	got, err := NewCreateHandler(service)(context.Background(), "/state", request)
	if err != nil {
		t.Fatal(err)
	}
	wantCommand := publicationcontract.CreateCommand{
		ID: request.ID, Provider: request.Provider, Title: request.Title, Body: request.Body,
		Head: request.Head, Base: request.Base, Labels: []string{"enhancement", "issueops"},
		Assignees: []string{"maintainer"}, ExpectedGeneration: request.ExpectedGeneration,
		Actor: publicationcontract.Actor{
			Host: request.Actor.Host, SessionID: request.Actor.SessionID, AgentID: request.Actor.AgentID,
			SessionProcess: &publicationcontract.ProcessReceipt{
				PID: 123, StartedAt: "2026-08-01T00:00:00.123456789Z", Executable: "/usr/local/bin/codex",
			},
			ProcessAncestry: []publicationcontract.ProcessReceipt{
				{PID: 122, StartedAt: "2026-08-01T00:00:00.023456789Z", Executable: "/usr/bin/zsh"},
				{PID: 123, StartedAt: "2026-08-01T00:00:00.123456789Z", Executable: "/usr/local/bin/codex"},
			},
		},
		CWD: request.CWD, Confirm: true,
	}
	if !reflect.DeepEqual(service.command, wantCommand) {
		t.Fatalf("command=%#v want=%#v", service.command, wantCommand)
	}
	wantResult := port.IssueProviderCreatePullRequestResult{OK: true, URL: "https://github.com/acme/repo/pull/1", Number: "1", Preview: "created preview"}
	if !reflect.DeepEqual(got, wantResult) {
		t.Fatalf("result=%#v want=%#v", got, wantResult)
	}

	request.Labels[0] = "changed"
	request.Assignees[0] = "changed"
	request.Actor.SessionProcess.PID = 999
	request.Actor.ProcessAncestry[0].PID = 998
	if !reflect.DeepEqual(service.command, wantCommand) {
		t.Fatalf("mapped command aliases public request: %#v", service.command)
	}
}

func TestCreateHandlerPreservesServiceErrorAndResult(t *testing.T) {
	cause := errors.New("remote create outcome requires execution reconcile")
	service := &fakeCreateService{
		t: t, expected: true, err: cause,
		result: publicationcontract.ProviderCreateResult{URL: "https://github.com/acme/repo/pull/1", Number: "1"},
	}
	got, err := NewCreateHandler(service)(context.Background(), "/state", fullCoreCreateRequest())
	if err != cause || got.URL != "https://github.com/acme/repo/pull/1" || got.Number != "1" {
		t.Fatalf("result=%#v err=%v", got, err)
	}
}

func TestCreateHandlerPreservesNilAndEmptySliceShape(t *testing.T) {
	for _, test := range []struct {
		name   string
		labels []string
	}{
		{name: "nil", labels: nil},
		{name: "empty", labels: []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeCreateService{t: t, expected: true}
			request := fullCoreCreateRequest()
			request.Labels = test.labels
			request.Assignees = test.labels
			request.Actor.ProcessAncestry = nil
			if _, err := NewCreateHandler(service)(context.Background(), "/state", request); err != nil {
				t.Fatal(err)
			}
			if (service.command.Labels == nil) != (test.labels == nil) ||
				(service.command.Assignees == nil) != (test.labels == nil) ||
				service.command.Actor.ProcessAncestry != nil {
				t.Fatalf("command slice shape=%#v", service.command)
			}
		})
	}
}

func TestCreateHandlerFailsClosedWithoutService(t *testing.T) {
	got, err := NewCreateHandler(nil)(context.Background(), "/state", fullCoreCreateRequest())
	if !errors.Is(err, issueops.ErrRemotePullRequestCreateHandlerUnavailable) || got != (port.IssueProviderCreatePullRequestResult{}) {
		t.Fatalf("result=%#v err=%v", got, err)
	}
}
