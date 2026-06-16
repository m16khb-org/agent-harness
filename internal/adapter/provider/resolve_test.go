package provider

import "testing"

func TestResolveKnownProviders(t *testing.T) {
	for _, name := range []string{"github", "gitlab"} {
		prov, err := Resolve(name)
		if err != nil {
			t.Fatalf("Resolve(%q) error: %v", name, err)
		}
		if prov == nil {
			t.Fatalf("Resolve(%q) returned nil provider", name)
		}
	}
}

func TestResolveUnknownProvider(t *testing.T) {
	if _, err := Resolve("bad"); err == nil {
		t.Fatal("unknown provider should fail")
	}
}
