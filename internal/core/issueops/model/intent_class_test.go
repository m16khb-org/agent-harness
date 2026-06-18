package model

import "testing"

func TestNormalizeIntentClass(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "standard", false},
		{"  ", "standard", false},
		{"Trivial", "trivial", false},
		{"architecture", "architecture", false},
		{"research", "research", false},
		{"bogus", "", true},
	}
	for _, c := range cases {
		got, err := NormalizeIntentClass(c.in)
		if c.wantErr {
			if err == nil {
				t.Fatalf("NormalizeIntentClass(%q) expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("NormalizeIntentClass(%q) unexpected error: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("NormalizeIntentClass(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
