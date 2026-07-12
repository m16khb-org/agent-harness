package daemonpaths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadInstanceSupportsAdditiveJSONAndLegacyPID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-harness.pid")
	record := InstanceRecord{
		PID:              4242,
		ProcessStartTime: "start-a",
		Executable:       "/tmp/agent-harness",
		InstanceNonce:    "nonce-a",
		BuildSHA:         "build-a",
		ProtocolVersion:  "1",
		Generation:       "generation-a",
	}
	if err := WriteInstance(path, record); err != nil {
		t.Fatal(err)
	}
	got, legacy, err := ReadInstance(path)
	if err != nil || legacy || got != record {
		t.Fatalf("JSON instance round-trip failed: got=%#v legacy=%v err=%v", got, legacy, err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("instance file must be private: info=%v err=%v", info, err)
	}

	if err := os.WriteFile(path, []byte("123\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, legacy, err = ReadInstance(path)
	if err != nil || !legacy || got.PID != 123 {
		t.Fatalf("legacy pid read failed: got=%#v legacy=%v err=%v", got, legacy, err)
	}
}

func TestInspectProcessReturnsStableCurrentIdentity(t *testing.T) {
	first, err := InspectProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	second, err := InspectProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if first.StartTime == "" || first.Executable == "" || first != second {
		t.Fatalf("expected stable current process identity, first=%#v second=%#v", first, second)
	}
}
