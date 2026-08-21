package dogfood

import (
	"os"
	"testing"
)

func TestMaterializeForE2E(t *testing.T) {
	if os.Getenv("DOGFOOD_E2E_DIR") == "" {
		t.Skip("e2e only")
	}
	if err := Materialize(os.Getenv("DOGFOOD_E2E_DIR"), DirtyFiles()); err != nil {
		t.Fatal(err)
	}
	if err := Materialize(os.Getenv("DOGFOOD_E2E_DIR")+"-clean", CleanFiles()); err != nil {
		t.Fatal(err)
	}
}
