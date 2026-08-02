package lifecycle_test

import (
	"encoding/json"
	"testing"

	"agent-harness/internal/contract/lifecycle"
)

func TestHookRequestNeverAcceptsProcessAncestryFromJSON(t *testing.T) {
	var request lifecycle.HookToolUseLifecycleRequest
	if err := json.Unmarshal([]byte(`{"host":"codex","native_process_ancestry":[{"pid":7}]}`), &request); err != nil {
		t.Fatal(err)
	}
	if len(request.NativeProcessAncestry) != 0 {
		t.Fatal("payload supplied process ancestry became authoritative")
	}
}
