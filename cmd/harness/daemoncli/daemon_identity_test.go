package daemoncli

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"agent-harness/cmd/harness/daemoncli/daemonpaths"
)

func TestCheckDaemonStatusVerifiesInstanceFileSocketAndProcess(t *testing.T) {
	instance := daemonInstance{
		PID:              4242,
		ProcessStartTime: "2026-07-10T04:00:00Z",
		Executable:       "/tmp/agent-harness",
		InstanceNonce:    "nonce-a",
		BuildSHA:         "build-a",
		ProtocolVersion:  daemonProtocolVersion,
		Generation:       "generation-a",
	}
	paths := daemonPaths{Dir: t.TempDir(), Socket: "daemon.sock", PID: "daemon.pid"}

	status := checkDaemonStatusWithDeps(daemonStatusDeps{
		paths: func() (daemonPaths, error) { return paths, nil },
		readInstance: func(string) (daemonInstance, bool, error) {
			return instance, false, nil
		},
		probeIdentity: func(string) (daemonInstance, error) { return instance, nil },
		processAlive:  func(int) bool { return true },
		inspectProcess: func(int) (daemonProcessIdentity, error) {
			return daemonProcessIdentity{StartTime: instance.ProcessStartTime, Executable: instance.Executable}, nil
		},
	})

	if !status.OK || !status.Running || !status.Reachable || !status.IdentityVerified {
		t.Fatalf("expected verified running daemon, got %#v", status)
	}
	if status.Code != daemonStatusReady || status.Instance == nil || !reflect.DeepEqual(*status.Instance, instance) {
		t.Fatalf("unexpected verified identity status: %#v", status)
	}
}

func TestCheckDaemonStatusRechecksInstanceRecordAfterStartupSocketProbe(t *testing.T) {
	instance := daemonInstance{
		PID:              4242,
		ProcessStartTime: "start-a",
		Executable:       "/tmp/agent-harness",
		InstanceNonce:    "nonce-a",
		BuildSHA:         "build-a",
		ProtocolVersion:  daemonProtocolVersion,
		Generation:       "generation-a",
	}
	reads := 0
	status := checkDaemonStatusWithDeps(daemonStatusDeps{
		paths: func() (daemonPaths, error) { return daemonPaths{Socket: "daemon.sock", PID: "daemon.pid"}, nil },
		readInstance: func(string) (daemonInstance, bool, error) {
			reads++
			if reads == 1 {
				return daemonInstance{}, false, os.ErrNotExist
			}
			return instance, false, nil
		},
		probeIdentity: func(string) (daemonInstance, error) { return instance, nil },
		processAlive:  func(int) bool { return true },
		inspectProcess: func(int) (daemonProcessIdentity, error) {
			return daemonProcessIdentity{StartTime: instance.ProcessStartTime, Executable: instance.Executable}, nil
		},
	})

	if !status.OK || !status.Running || !status.Reachable || !status.IdentityVerified || status.Code != daemonStatusReady || reads != 2 {
		t.Fatalf("startup socket probe should recheck its instance record, got status=%#v reads=%d", status, reads)
	}
}

func TestCheckDaemonStatusReportsLiveAdmissionHealth(t *testing.T) {
	instance := daemonInstance{
		PID:              4242,
		ProcessStartTime: "start-a",
		Executable:       "/tmp/agent-harness",
		InstanceNonce:    "nonce-a",
		BuildSHA:         "build-a",
		ProtocolVersion:  daemonProtocolVersion,
		Generation:       "generation-a",
	}
	status := checkDaemonStatusWithDeps(daemonStatusDeps{
		paths:        func() (daemonPaths, error) { return daemonPaths{Socket: "daemon.sock", PID: "daemon.pid"}, nil },
		readInstance: func(string) (daemonInstance, bool, error) { return instance, false, nil },
		probeStatus: func(string) (daemonIdentityResponse, error) {
			return daemonIdentityResponse{
				OK:                true,
				Instance:          instance,
				ActiveConnections: maxConnections,
				MaxConnections:    maxConnections,
				Accepting:         false,
				Draining:          false,
			}, nil
		},
		processAlive: func(int) bool { return true },
		inspectProcess: func(int) (daemonProcessIdentity, error) {
			return daemonProcessIdentity{StartTime: instance.ProcessStartTime, Executable: instance.Executable}, nil
		},
	})

	if !status.OK || status.ActiveConnections != maxConnections || status.MaxConnections != maxConnections || status.Accepting || status.Draining {
		t.Fatalf("daemon status lost live admission health: %#v", status)
	}
}

func TestCheckDaemonStatusReportsHandshakeIdentityMismatch(t *testing.T) {
	fileInstance := daemonInstance{
		PID:              4242,
		ProcessStartTime: "start-a",
		Executable:       "/tmp/agent-harness",
		InstanceNonce:    "nonce-a",
		BuildSHA:         "build-a",
		ProtocolVersion:  daemonProtocolVersion,
		Generation:       "generation-a",
	}
	fields := map[string]func(*daemonInstance){
		"nonce":      func(v *daemonInstance) { v.InstanceNonce = "nonce-b" },
		"build":      func(v *daemonInstance) { v.BuildSHA = "build-b" },
		"protocol":   func(v *daemonInstance) { v.ProtocolVersion = "2" },
		"executable": func(v *daemonInstance) { v.Executable = "/tmp/other" },
		"generation": func(v *daemonInstance) { v.Generation = "generation-b" },
	}
	for name, mutate := range fields {
		t.Run(name, func(t *testing.T) {
			handshake := fileInstance
			mutate(&handshake)
			status := checkDaemonStatusWithDeps(daemonStatusDeps{
				paths:         func() (daemonPaths, error) { return daemonPaths{Socket: "daemon.sock", PID: "daemon.pid"}, nil },
				readInstance:  func(string) (daemonInstance, bool, error) { return fileInstance, false, nil },
				probeIdentity: func(string) (daemonInstance, error) { return handshake, nil },
				processAlive:  func(int) bool { return true },
				inspectProcess: func(int) (daemonProcessIdentity, error) {
					t.Fatal("OS identity must not be trusted after handshake mismatch")
					return daemonProcessIdentity{}, nil
				},
			})
			if status.OK || !status.Running || !status.Reachable || status.IdentityVerified || status.Code != daemonStatusIdentityMismatch {
				t.Fatalf("expected fail-closed handshake mismatch, got %#v", status)
			}
		})
	}
}

func TestCheckDaemonStatusAcceptsExecutableProjectionDriftAfterSymlinkUpdate(t *testing.T) {
	instance := daemonInstance{
		PID:              4242,
		ProcessStartTime: "start-a",
		Executable:       "/repo-before/bin/agent-harness",
		InstanceNonce:    "nonce-a",
		BuildSHA:         "build-a",
		ProtocolVersion:  daemonProtocolVersion,
		Generation:       "generation-a",
	}
	status := checkDaemonStatusWithDeps(daemonStatusDeps{
		paths:         func() (daemonPaths, error) { return daemonPaths{Socket: "daemon.sock", PID: "daemon.pid"}, nil },
		readInstance:  func(string) (daemonInstance, bool, error) { return instance, false, nil },
		probeIdentity: func(string) (daemonInstance, error) { return instance, nil },
		processAlive:  func(int) bool { return true },
		inspectProcess: func(int) (daemonProcessIdentity, error) {
			return daemonProcessIdentity{
				StartTime:  instance.ProcessStartTime,
				Executable: "/repo-after/bin/agent-harness",
			}, nil
		},
	})

	if !status.OK || !status.IdentityVerified || status.Code != daemonStatusReady {
		t.Fatalf("same process and handshake must survive launcher symlink retargeting: %#v", status)
	}
}

func TestCheckDaemonStatusTreatsLegacyPIDAsStatusOnly(t *testing.T) {
	status := checkDaemonStatusWithDeps(daemonStatusDeps{
		paths:         func() (daemonPaths, error) { return daemonPaths{Socket: "daemon.sock", PID: "daemon.pid"}, nil },
		readInstance:  func(string) (daemonInstance, bool, error) { return daemonInstance{PID: 4242}, true, nil },
		probeIdentity: func(string) (daemonInstance, error) { return daemonInstance{PID: 4242}, nil },
		processAlive:  func(int) bool { return true },
	})

	if status.OK || !status.Running || !status.Reachable || status.IdentityVerified || !status.LegacyPID || status.Code != daemonStatusLegacyPID {
		t.Fatalf("legacy pid must be observable but untrusted: %#v", status)
	}
}

func TestCheckDaemonStatusKeepsLiveLegacyPIDFailClosedWithoutSocket(t *testing.T) {
	status := checkDaemonStatusWithDeps(daemonStatusDeps{
		paths:         func() (daemonPaths, error) { return daemonPaths{Socket: "daemon.sock", PID: "daemon.pid"}, nil },
		readInstance:  func(string) (daemonInstance, bool, error) { return daemonInstance{PID: 4242}, true, nil },
		probeIdentity: func(string) (daemonInstance, error) { return daemonInstance{}, errors.New("socket unavailable") },
		processAlive:  func(int) bool { return true },
	})

	if status.OK || !status.Running || status.Reachable || status.IdentityVerified || !status.LegacyPID || status.Code != daemonStatusLegacyPID {
		t.Fatalf("live legacy pid must remain observable and untrusted: %#v", status)
	}
}

func TestDaemonStatusForMCPMatchesCLIIdentityFields(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "ahd-mcp-identity-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("HARNESS_DAEMON_DIR", dir)
	paths, err := currentDaemonPaths()
	if err != nil {
		t.Fatal(err)
	}
	wantInstance := startVerifiedDaemonTestSocket(t, paths)

	cliStatus := checkDaemonStatus()
	mcpStatus := daemonStatusForMCP()
	if !reflect.DeepEqual(mcpStatus, cliStatus) {
		t.Fatalf("CLI/MCP daemon status drift: cli=%#v mcp=%#v", cliStatus, mcpStatus)
	}
	if mcpStatus.Instance == nil || *mcpStatus.Instance != wantInstance {
		t.Fatalf("MCP status lost protocol/build/generation identity: %#v", mcpStatus)
	}
}

func TestEnsureDaemonRunningBlocksUnverifiedExistingProcess(t *testing.T) {
	status, err := ensureDaemonRunningWithDeps(daemonStartDeps{
		checkStatus: func() daemonStatus {
			return daemonStatus{OK: false, PID: 4242, Code: daemonStatusSocketUnreachable, Message: "daemon pid exists but socket is not reachable"}
		},
		paths: func() (daemonPaths, error) {
			t.Fatal("unverified live process must block before launcher setup")
			return daemonPaths{}, nil
		},
	})

	if err == nil || status.PID != 4242 || !strings.Contains(err.Error(), daemonStatusSocketUnreachable) {
		t.Fatalf("expected unverified process to block start, status=%#v err=%v", status, err)
	}
}

func TestServeDaemonConnectionReturnsExactIdentityHandshake(t *testing.T) {
	server, client := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	instance := daemonInstance{
		PID:              4242,
		ProcessStartTime: "start-a",
		Executable:       "/tmp/agent-harness",
		InstanceNonce:    "nonce-a",
		BuildSHA:         "build-a",
		ProtocolVersion:  daemonProtocolVersion,
		Generation:       "generation-a",
	}
	done := make(chan error, 1)
	go func() {
		done <- serveDaemonConnection(server, &daemonServerFakeLog{}, instance, func(net.Conn, daemonServerLogFile) error {
			return errors.New("identity probe must bypass MCP stream")
		})
	}()

	if _, err := io.WriteString(client, daemonIdentityRequest); err != nil {
		t.Fatal(err)
	}
	var response daemonIdentityResponse
	if err := json.NewDecoder(client).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if !response.OK || !reflect.DeepEqual(response.Instance, instance) {
		t.Fatalf("unexpected identity response: %#v", response)
	}
	if err := <-done; err != nil {
		t.Fatalf("identity probe failed: %v", err)
	}
}

func TestServeDaemonConnectionReportsAdmissionHealth(t *testing.T) {
	server, client := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	done := make(chan error, 1)
	go func() {
		done <- serveDaemonConnection(server, &daemonServerFakeLog{}, daemonInstance{}, func(net.Conn, daemonServerLogFile) error {
			return errors.New("identity probe must bypass MCP stream")
		})
	}()

	if _, err := io.WriteString(client, daemonIdentityRequest); err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err := json.NewDecoder(client).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response["active_connections"] != float64(0) || response["max_connections"] != float64(maxConnections) {
		t.Fatalf("identity response lost connection counts: %#v", response)
	}
	if response["accepting"] != true || response["draining"] != false {
		t.Fatalf("identity response lost admission state: %#v", response)
	}
	if err := <-done; err != nil {
		t.Fatalf("identity probe failed: %v", err)
	}
}

func TestDaemonStatusJSONIncludesAdmissionHealth(t *testing.T) {
	raw, err := json.Marshal(daemonStatus{})
	if err != nil {
		t.Fatal(err)
	}
	var status map[string]any
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"active_connections", "max_connections", "accepting", "draining"} {
		if _, ok := status[field]; !ok {
			t.Fatalf("daemon status JSON is missing %q: %s", field, raw)
		}
	}
}

func TestServeDaemonConnectionPreservesMCPInput(t *testing.T) {
	server, client := net.Pipe()
	payload := "{\"jsonrpc\":\"2.0\",\"id\":1}\n"
	done := make(chan error, 1)
	go func() {
		done <- serveDaemonConnection(server, &daemonServerFakeLog{}, daemonInstance{}, func(conn net.Conn, _ daemonServerLogFile) error {
			got, err := io.ReadAll(conn)
			if err != nil {
				return err
			}
			if string(got) != payload {
				return errors.New("MCP input was not replayed exactly")
			}
			return nil
		})
	}()
	if _, err := io.WriteString(client, payload); err != nil {
		t.Fatal(err)
	}
	_ = client.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestDaemonExecutableSHAUsesBinaryBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.WriteFile(path, []byte("abc"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := daemonExecutableSHA(path)
	if err != nil {
		t.Fatal(err)
	}
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Fatalf("executable SHA = %q, want %q", got, want)
	}
}

func TestStopDaemonRechecksOSIdentityBeforeAnySignal(t *testing.T) {
	instance := daemonInstance{PID: 4242, ProcessStartTime: "start-a", Executable: "/tmp/agent-harness"}
	tests := map[string]daemonProcessIdentity{
		"pid reuse":           {StartTime: "reused-pid", Executable: instance.Executable},
		"executable mismatch": {StartTime: instance.ProcessStartTime, Executable: "/tmp/other", ExecutablePathStable: true},
	}
	for name, processIdentity := range tests {
		t.Run(name, func(t *testing.T) {
			proc := &fakeDaemonProcess{}
			status, err := stopDaemonWithDeps(daemonStopDeps{
				checkStatus: func() daemonStatus {
					return daemonStatus{OK: true, Running: true, Reachable: true, IdentityVerified: true, PID: instance.PID, Code: daemonStatusReady, Instance: &instance}
				},
				findProcess:    func(int) (daemonProcess, error) { return proc, nil },
				inspectProcess: func(int) (daemonProcessIdentity, error) { return processIdentity, nil },
				processAlive:   func(int) bool { return true },
			})

			if err == nil || status.Code != daemonStatusIdentityMismatch {
				t.Fatalf("expected identity rejection, status=%#v err=%v", status, err)
			}
			if len(proc.signals) != 0 || proc.kills != 0 {
				t.Fatalf("identity mismatch must send zero signals: signals=%v kills=%d", proc.signals, proc.kills)
			}
		})
	}
}

func TestStopDaemonRejectsLivePIDWithoutSocketWithoutSignal(t *testing.T) {
	mutated := false
	status, err := stopDaemonWithDeps(daemonStopDeps{
		checkStatus: func() daemonStatus {
			return daemonStatus{OK: false, Running: true, Reachable: false, PID: 4242, Code: daemonStatusSocketUnreachable}
		},
		findProcess: func(int) (daemonProcess, error) {
			mutated = true
			return &fakeDaemonProcess{}, nil
		},
	})

	if err == nil || status.Code != daemonStatusSocketUnreachable {
		t.Fatalf("missing socket stop must fail closed: status=%#v err=%v", status, err)
	}
	if mutated {
		t.Fatal("missing socket stop must not signal a process")
	}
}

func TestStopDaemonLeavesUnrelatedLiveProcessAlive(t *testing.T) {
	child := exec.Command("sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = child.Process.Kill()
		_ = child.Wait()
	})
	instance := daemonInstance{
		PID:              child.Process.Pid,
		ProcessStartTime: "different-process-start",
		Executable:       "/tmp/agent-harness",
	}
	status, err := stopDaemonWithDeps(daemonStopDeps{
		checkStatus: func() daemonStatus {
			return daemonStatus{OK: true, Running: true, Reachable: true, IdentityVerified: true, PID: instance.PID, Code: daemonStatusReady, Instance: &instance}
		},
		findProcess:    func(pid int) (daemonProcess, error) { return os.FindProcess(pid) },
		inspectProcess: daemonpaths.InspectProcess,
		processAlive:   processAlive,
	})

	if err == nil || status.Code != daemonStatusIdentityMismatch {
		t.Fatalf("unrelated live process must be rejected: status=%#v err=%v", status, err)
	}
	if err := child.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("unrelated child did not survive rejected stop: %v", err)
	}
}

func TestStopDaemonRejectsLegacyPIDWithoutMutation(t *testing.T) {
	mutated := false
	status, err := stopDaemonWithDeps(daemonStopDeps{
		checkStatus: func() daemonStatus {
			return daemonStatus{OK: false, Running: true, Reachable: true, LegacyPID: true, PID: 4242, Code: daemonStatusLegacyPID}
		},
		findProcess: func(int) (daemonProcess, error) {
			mutated = true
			return &fakeDaemonProcess{}, nil
		},
		remove: func(string) error {
			mutated = true
			return nil
		},
	})

	if err == nil || status.Code != daemonStatusLegacyPID {
		t.Fatalf("legacy stop must fail closed: status=%#v err=%v", status, err)
	}
	if mutated {
		t.Fatal("legacy stop must not signal processes or remove state")
	}
}

func TestStopDaemonSignalsOnlyVerifiedInstance(t *testing.T) {
	instance := daemonInstance{PID: 4242, ProcessStartTime: "start-a", Executable: "/tmp/agent-harness"}
	proc := &fakeDaemonProcess{}
	aliveChecks := 0
	status, err := stopDaemonWithDeps(daemonStopDeps{
		checkStatus: func() daemonStatus {
			return daemonStatus{OK: true, Running: true, Reachable: true, IdentityVerified: true, PID: instance.PID, Code: daemonStatusReady, Instance: &instance, Paths: daemonPaths{Socket: "daemon.sock", PID: "daemon.pid"}}
		},
		findProcess: func(int) (daemonProcess, error) { return proc, nil },
		inspectProcess: func(int) (daemonProcessIdentity, error) {
			return daemonProcessIdentity{StartTime: instance.ProcessStartTime, Executable: instance.Executable}, nil
		},
		processAlive: func(int) bool {
			aliveChecks++
			return aliveChecks == 1
		},
		remove: func(string) error { return nil },
		now:    func() time.Time { return time.Unix(100, 0) },
		sleep:  func(time.Duration) {},
	})

	if err != nil || status.Running || status.Code != daemonStatusStopped {
		t.Fatalf("verified stop failed: status=%#v err=%v", status, err)
	}
	if !reflect.DeepEqual(proc.signals, []os.Signal{syscall.SIGTERM}) || proc.kills != 0 {
		t.Fatalf("expected one TERM and no kill: signals=%v kills=%d", proc.signals, proc.kills)
	}
}

type fakeDaemonProcess struct {
	signals []os.Signal
	kills   int
}

func (p *fakeDaemonProcess) Signal(signal os.Signal) error {
	p.signals = append(p.signals, signal)
	return nil
}

func (p *fakeDaemonProcess) Kill() error {
	p.kills++
	return nil
}

func startVerifiedDaemonTestSocket(t *testing.T, paths daemonPaths) daemonInstance {
	t.Helper()
	instance := writeVerifiedDaemonTestInstance(t, paths)
	listener, err := net.Listen("unix", paths.Socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = serveDaemonConnection(conn, &daemonServerFakeLog{}, instance, func(net.Conn, daemonServerLogFile) error {
					return errors.New("unexpected MCP stream")
				})
			}()
		}
	}()
	return instance
}

func writeVerifiedDaemonTestInstance(t *testing.T, paths daemonPaths) daemonInstance {
	t.Helper()
	process, err := daemonpaths.InspectProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	instance := daemonInstance{
		PID:              os.Getpid(),
		ProcessStartTime: process.StartTime,
		Executable:       process.Executable,
		InstanceNonce:    "test-nonce",
		BuildSHA:         "test-build",
		ProtocolVersion:  daemonProtocolVersion,
		Generation:       "test-generation",
	}
	if err := daemonpaths.WriteInstance(paths.PID, instance); err != nil {
		t.Fatal(err)
	}
	return instance
}
