package looprun

import (
	"strings"
	"testing"

	"agent-harness/internal/core/sqlstore"
)

func TestReadLoopRefusesFutureSchema(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	loop := startLoopForTest(t, "future-schema", 2)
	db, err := sqlstore.Open(StateRoot())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(loopBucket, loop.ID, []byte(`{"ok":true,"schema_version":99,"id":"`+loop.ID+`"}`+"\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLoop(loop.ID); err == nil || !strings.Contains(err.Error(), "unsupported loop schema_version") {
		t.Fatalf("ReadLoop err=%v, want future schema refusal", err)
	}
}
