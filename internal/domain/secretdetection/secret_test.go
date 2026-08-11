package secretdetection

import "testing"

func TestContainsRejectsCredentialShapes(t *testing.T) {
	for _, value := range []string{
		"api_key=secret-value",
		"-----BEGIN OPENSSH PRIVATE KEY-----",
		"ghp_123456789012345678901234567890",
	} {
		if !Contains(value) {
			t.Fatalf("secret pattern was not detected: %q", value)
		}
	}
	if Contains("# Plan\nNo credentials here.\n") {
		t.Fatal("ordinary artifact text was rejected")
	}
}
