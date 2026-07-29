package issueopslease

import (
	"context"
	"testing"

	leasedomain "agent-harness/internal/domain/issueopslease"
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
