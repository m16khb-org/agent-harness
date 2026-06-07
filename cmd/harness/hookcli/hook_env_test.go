package hookcli

import (
	"os"
	"testing"
)

func TestEnvFloatParsesConfiguredFloat(t *testing.T) {
	t.Setenv("AGENT_HARNESS_TEST_FLOAT", " 12.5 ")

	got := EnvFloat("AGENT_HARNESS_TEST_FLOAT")

	if got != 12.5 {
		t.Fatalf("expected configured float, got %v", got)
	}
}

func TestEnvFloatReturnsZeroForMissingEmptyAndInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
		set   bool
	}{
		{name: "missing", set: false},
		{name: "empty", value: "   ", set: true},
		{name: "invalid", value: "not-a-float", set: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set {
				t.Setenv("AGENT_HARNESS_TEST_FLOAT", tt.value)
			} else {
				old, hadOld := os.LookupEnv("AGENT_HARNESS_TEST_FLOAT")
				t.Cleanup(func() {
					if hadOld {
						if err := os.Setenv("AGENT_HARNESS_TEST_FLOAT", old); err != nil {
							t.Fatalf("restore env: %v", err)
						}
					} else {
						if err := os.Unsetenv("AGENT_HARNESS_TEST_FLOAT"); err != nil {
							t.Fatalf("restore unset env: %v", err)
						}
					}
				})
				if err := os.Unsetenv("AGENT_HARNESS_TEST_FLOAT"); err != nil {
					t.Fatalf("unset env: %v", err)
				}
			}

			got := EnvFloat("AGENT_HARNESS_TEST_FLOAT")

			if got != 0 {
				t.Fatalf("expected zero fallback, got %v", got)
			}
		})
	}
}
