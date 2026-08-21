package port

import "testing"

func TestOrcaErrorMessage(t *testing.T) {
	err := &OrcaError{Code: "E_ORCA_CLI", Detail: "orca probe failed"}
	if err.Error() == "" {
		t.Fatal("orca error message must be non-empty")
	}
}
