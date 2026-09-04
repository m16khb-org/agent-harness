package issueopslease

import (
	"context"
	"testing"

	leasedomain "issueops/internal/domain/issueopslease"
)

func TestReleaseKeepsLegacyNativeActorValidationErrorsPublic(t *testing.T) {
	service := NewReleaseService(nil, nil, nil, nil)
	for _, tc := range []struct {
		name  string
		actor leasedomain.Actor
		want  string
	}{
		{name: "missing session", actor: leasedomain.Actor{Host: "codex"}, want: "native actor session_id is required"},
		{name: "missing receipt", actor: leasedomain.Actor{Host: "codex", SessionID: "missing-receipt"}, want: "native actor requires a PID reuse-safe session_process receipt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.Release(context.Background(), ReleaseRequest{ID: "io-actor-receipt", Generation: 1, Actor: tc.actor})
			if err == nil || err.Error() != tc.want {
				t.Fatalf("actor validation error=%v want=%q", err, tc.want)
			}
		})
	}
}

func TestResolveActorAcceptsOmoNativeProcess(t *testing.T) {
	process := leasedomain.ProcessReceipt{
		PID: 42, StartedAt: "2026-08-12T00:00:00Z", Executable: "/Users/test/Library/pnpm/bin/omo",
	}
	actor, err := resolveActor(
		context.Background(),
		leasedomain.Actor{Host: "omo", SessionID: "019ff5b8-7d62-707a-a693-5e7a5e8a3187", Process: &process},
		[]leasedomain.ProcessReceipt{process},
		func(context.Context, leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
			return "live", process, nil
		},
	)
	if err != nil {
		t.Fatalf("Omo native actor must resolve: %v", err)
	}
	if actor.Host != "omo" || actor.SessionID == "" || actor.Process == nil || *actor.Process != process {
		t.Fatalf("resolved Omo actor=%+v", actor)
	}
}
