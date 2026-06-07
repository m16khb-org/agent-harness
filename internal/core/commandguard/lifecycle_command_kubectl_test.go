package commandguard

import "testing"

func TestKubectlFlagTakesValueRecognizesValueFlags(t *testing.T) {
	for _, flag := range []string{"-n", "--namespace", "--context=prod", "--kubeconfig", "-o", "-l"} {
		if !KubectlFlagTakesValue(flag) {
			t.Fatalf("KubectlFlagTakesValue(%q) = false, want true", flag)
		}
	}

	for _, flag := range []string{"--watch", "--dry-run", "--force"} {
		if KubectlFlagTakesValue(flag) {
			t.Fatalf("KubectlFlagTakesValue(%q) = true, want false", flag)
		}
	}
}
