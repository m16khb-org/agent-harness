package cli

import (
	"strings"
	"testing"
)

func TestUsageIncludesCommandCatalog(t *testing.T) {
	usage := Usage("test")
	for _, command := range Commands() {
		if !strings.Contains(usage, "harness "+command.Name) && command.Name != "version" {
			t.Fatalf("usage does not mention command %q\n%s", command.Name, usage)
		}
	}
	for _, want := range []string{"policy audit", "contract schema", "worker enqueue"} {
		if !strings.Contains(usage, want) {
			t.Fatalf("usage missing %q", want)
		}
	}
}
