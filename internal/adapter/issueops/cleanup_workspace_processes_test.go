package issueops

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"testing"
	"time"

	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func cleanupSnapshotEntry(pid, ppid int, exe string) nativeProcessSnapshotEntry {
	return nativeProcessSnapshotEntry{ParentPID: ppid, Receipt: issueops.NativeProcessReceipt{
		PID: pid, StartedAt: fmt.Sprintf("2026-08-27T00:00:%02dZ", pid%60), Executable: exe,
	}}
}

// 점유 관측은 lsof 점유자와 같은 ps 스냅샷의 receipt·계보를 join한다. 자손 수와
// 워크트리 밖 자손(부수 피해 후보) 수는 공유 서버(tmux)를 알아보는 근거다(#477).
func TestBuildCleanupOccupancyJoinsReceiptsAndCountsDescendants(t *testing.T) {
	snapshot := map[int]nativeProcessSnapshotEntry{
		1:     cleanupSnapshotEntry(1, 0, "launchd"),
		903:   cleanupSnapshotEntry(903, 1, "Orca"),
		500:   cleanupSnapshotEntry(500, 903, "login"),
		501:   cleanupSnapshotEntry(501, 500, "zsh"),
		502:   cleanupSnapshotEntry(502, 501, "sleep"),
		20804: cleanupSnapshotEntry(20804, 1, "tmux"),
		30000: cleanupSnapshotEntry(30000, 20804, "claude"),
		30001: cleanupSnapshotEntry(30001, 30000, "node"),
		700:   cleanupSnapshotEntry(700, 1, "zsh"),
		777:   cleanupSnapshotEntry(777, 700, "issueops"),
	}
	procs := []workspaceProcess{
		{PID: 502, Command: "sleep", FD: "cwd"},
		{PID: 501, Command: "zsh", FD: "cwd"},
		{PID: 501, Command: "zsh", FD: "3", Access: "w"},
		{PID: 20804, Command: "tmux", FD: "cwd"},
	}
	occupancy, err := buildCleanupOccupancy(procs, snapshot, 777)
	if err != nil {
		t.Fatal(err)
	}
	if len(occupancy.Occupants) != 3 {
		t.Fatalf("occupants must be deduplicated by pid and sorted: %+v", occupancy.Occupants)
	}
	want := []issueops.CleanupWorkspaceProcess{
		{PID: 501, Command: "zsh", StartedAt: "2026-08-27T00:00:21Z", Executable: "zsh", Descendants: 1, Collateral: 0},
		{PID: 502, Command: "sleep", StartedAt: "2026-08-27T00:00:22Z", Executable: "sleep"},
		{PID: 20804, Command: "tmux", StartedAt: "2026-08-27T00:00:44Z", Executable: "tmux", Descendants: 2, Collateral: 2},
	}
	for i, w := range want {
		if occupancy.Occupants[i] != w {
			t.Fatalf("occupant[%d] = %+v, want %+v", i, occupancy.Occupants[i], w)
		}
	}
	if got := occupancy.Ancestry[777]; len(got) != 2 || got[0] != 700 || got[1] != 1 {
		t.Fatalf("requester ancestry = %v, want [700 1]", got)
	}
	if got := occupancy.Ancestry[502]; len(got) != 4 || got[0] != 501 || got[3] != 1 {
		t.Fatalf("occupant ancestry = %v, want [501 500 903 1]", got)
	}
}

func TestBuildCleanupOccupancyFailsClosedWhenSelfAncestryMissing(t *testing.T) {
	snapshot := map[int]nativeProcessSnapshotEntry{1: cleanupSnapshotEntry(1, 0, "launchd"), 501: cleanupSnapshotEntry(501, 1, "zsh")}
	if _, err := buildCleanupOccupancy([]workspaceProcess{{PID: 501, Command: "zsh", FD: "cwd"}}, snapshot, 777); err == nil {
		t.Fatal("requester pid missing from the snapshot must be an observation failure, not an exclusion of nothing")
	}
}

// 종료 헬퍼의 fake는 fail-closed다: 모르는 pid에 대한 신호는 실패로 기록되고,
// 관측은 신호에 따라 바뀐다.
type fakeCleanupProcessWorld struct {
	t         *testing.T
	occupants map[int]issueops.CleanupWorkspaceProcess
	ancestry  map[int][]int
	signals   []string
	diesOn    map[int]syscall.Signal
	observed  int
}

func (w *fakeCleanupProcessWorld) observe(string) (port.CleanupWorkspaceOccupancy, error) {
	w.observed++
	out := port.CleanupWorkspaceOccupancy{Ancestry: w.ancestry}
	for pid := 100; pid < 100000; pid++ {
		if o, ok := w.occupants[pid]; ok {
			out.Occupants = append(out.Occupants, o)
		}
	}
	return out, nil
}

func (w *fakeCleanupProcessWorld) signal(pid int, sig syscall.Signal) error {
	w.signals = append(w.signals, fmt.Sprintf("%d:%s", pid, sig))
	if _, ok := w.occupants[pid]; !ok {
		return syscall.ESRCH
	}
	if w.diesOn[pid] == sig {
		delete(w.occupants, pid)
	}
	return nil
}

func TestStopWorkspaceProcessesStopsOccupantsHupTermKill(t *testing.T) {
	a := issueops.CleanupWorkspaceProcess{PID: 501, Command: "zsh", StartedAt: "s1", Executable: "zsh"}
	b := issueops.CleanupWorkspaceProcess{PID: 600, Command: "node", StartedAt: "s2", Executable: "node"}
	world := &fakeCleanupProcessWorld{t: t,
		occupants: map[int]issueops.CleanupWorkspaceProcess{501: a, 600: b},
		ancestry:  map[int][]int{501: {500, 1}, 600: {1}, 777: {700, 1}},
		diesOn:    map[int]syscall.Signal{501: syscall.SIGHUP, 600: syscall.SIGKILL},
	}
	slept := 0
	stopped, err := stopCleanupWorkspaceProcesses("/tmp/wt", []issueops.CleanupWorkspaceProcess{a, b}, map[int]bool{903: true}, CleanupProcessDeps{
		Observe: world.observe, Signal: world.signal, Sleep: func(time.Duration) { slept++ }, SelfPID: 777,
	})
	if err != nil {
		t.Fatalf("stop must converge: %v", err)
	}
	if len(stopped) != 2 || stopped[0].PID != 501 || stopped[1].PID != 600 {
		t.Fatalf("stopped = %+v", stopped)
	}
	joined := strings.Join(world.signals, " ")
	for _, want := range []string{"501:hangup", "501:terminated", "600:hangup", "600:terminated", "600:killed"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("signals %v must contain %q", world.signals, want)
		}
	}
	if strings.Contains(joined, "501:killed") {
		t.Fatalf("a process that exited on HUP must not be SIGKILLed: %v", world.signals)
	}
	if world.observed < 3 || slept == 0 {
		t.Fatalf("stop must re-observe occupancy between HUP/TERM, KILL, and the final proof: observed=%d slept=%d", world.observed, slept)
	}
}

func TestStopWorkspaceProcessesRefusesReceiptMismatchAndRequester(t *testing.T) {
	a := issueops.CleanupWorkspaceProcess{PID: 501, Command: "zsh", StartedAt: "s1", Executable: "zsh"}
	t.Run("pid reuse", func(t *testing.T) {
		reused := a
		reused.StartedAt = "s9"
		world := &fakeCleanupProcessWorld{t: t, occupants: map[int]issueops.CleanupWorkspaceProcess{501: reused}, ancestry: map[int][]int{501: {1}, 777: {1}}}
		_, err := stopCleanupWorkspaceProcesses("/tmp/wt", []issueops.CleanupWorkspaceProcess{a}, nil, CleanupProcessDeps{Observe: world.observe, Signal: world.signal, Sleep: func(time.Duration) {}, SelfPID: 777})
		if err == nil || len(world.signals) != 0 {
			t.Fatalf("a changed receipt must refuse before any signal: err=%v signals=%v", err, world.signals)
		}
	})
	t.Run("requester ancestor occupies", func(t *testing.T) {
		shell := issueops.CleanupWorkspaceProcess{PID: 700, Command: "zsh", StartedAt: "s7", Executable: "zsh"}
		world := &fakeCleanupProcessWorld{t: t, occupants: map[int]issueops.CleanupWorkspaceProcess{700: shell}, ancestry: map[int][]int{700: {1}, 777: {700, 1}}}
		_, err := stopCleanupWorkspaceProcesses("/tmp/wt", []issueops.CleanupWorkspaceProcess{shell}, nil, CleanupProcessDeps{Observe: world.observe, Signal: world.signal, Sleep: func(time.Duration) {}, SelfPID: 777})
		if err == nil || len(world.signals) != 0 {
			t.Fatalf("the requester's own shell must never be signalled: err=%v signals=%v", err, world.signals)
		}
	})
	t.Run("descendant of a preview occupant is tolerated", func(t *testing.T) {
		child := issueops.CleanupWorkspaceProcess{PID: 502, Command: "sleep", StartedAt: "s2", Executable: "sleep"}
		world := &fakeCleanupProcessWorld{t: t, occupants: map[int]issueops.CleanupWorkspaceProcess{501: a, 502: child}, ancestry: map[int][]int{501: {1}, 502: {501, 1}, 777: {1}},
			diesOn: map[int]syscall.Signal{501: syscall.SIGTERM, 502: syscall.SIGTERM}}
		stopped, err := stopCleanupWorkspaceProcesses("/tmp/wt", []issueops.CleanupWorkspaceProcess{a}, nil, CleanupProcessDeps{Observe: world.observe, Signal: world.signal, Sleep: func(time.Duration) {}, SelfPID: 777})
		if err != nil || len(stopped) != 2 {
			t.Fatalf("a currently occupying descendant of a preview occupant is a valid target: err=%v stopped=%+v", err, stopped)
		}
	})
	t.Run("unknown occupant is stale", func(t *testing.T) {
		stranger := issueops.CleanupWorkspaceProcess{PID: 900, Command: "vim", StartedAt: "s3", Executable: "vim"}
		world := &fakeCleanupProcessWorld{t: t, occupants: map[int]issueops.CleanupWorkspaceProcess{501: a, 900: stranger}, ancestry: map[int][]int{501: {1}, 900: {1}, 777: {1}}}
		_, err := stopCleanupWorkspaceProcesses("/tmp/wt", []issueops.CleanupWorkspaceProcess{a}, nil, CleanupProcessDeps{Observe: world.observe, Signal: world.signal, Sleep: func(time.Duration) {}, SelfPID: 777})
		if err == nil || len(world.signals) != 0 || !errors.Is(err, errCleanupOccupancyChanged) {
			t.Fatalf("an occupant absent from the preview must be stale, before any signal: err=%v signals=%v", err, world.signals)
		}
	})
}
