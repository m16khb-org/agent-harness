package installcli

import (
	"testing"

	"issueops/internal/testsupport"
)

func captureStatusVerifyStdout(t *testing.T, fn func() error) string {
	t.Helper()
	return testsupport.CaptureStdout(t, fn)
}
