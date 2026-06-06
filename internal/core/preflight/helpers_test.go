package preflight

import "testing"

func TestRedactRemoteRedactsCredentials(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "http token user info",
			in:   "https://token@github.com/acme/repo.git",
			want: "https://<redacted>@github.com/acme/repo.git",
		},
		{
			name: "scheme username password",
			in:   "ssh://user:pass@example.com/acme/repo.git",
			want: "ssh://<redacted>:<redacted>@example.com/acme/repo.git",
		},
		{
			name: "scp style remote",
			in:   "git@github.com:acme/repo.git",
			want: "git@github.com:acme/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redactRemote(tt.in); got != tt.want {
				t.Fatalf("redactRemote() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAtoiParsesLeadingIntegerOrZero(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{in: "42", want: 42},
		{in: "7 files", want: 7},
		{in: "-3", want: -3},
		{in: "not a number", want: 0},
		{in: "", want: 0},
	}

	for _, tt := range tests {
		if got := atoi(tt.in); got != tt.want {
			t.Fatalf("atoi(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
