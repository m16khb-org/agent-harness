package daemoncli

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"agent-harness/cmd/harness/daemoncli/daemonpaths"
)

const (
	daemonStatusReady              = "ready"
	daemonStatusStopped            = "stopped"
	daemonStatusSocketUnreachable  = "socket_unreachable"
	daemonStatusIdentityMismatch   = "instance_identity_mismatch"
	daemonStatusInstanceUnreadable = "instance_record_unreadable"
)

type daemonStatusDeps struct {
	paths          func() (daemonPaths, error)
	readInstance   func(string) (daemonInstance, error)
	probeStatus    func(string) (daemonIdentityResponse, error)
	probeIdentity  func(string) (daemonInstance, error)
	processAlive   func(int) bool
	inspectProcess func(int) (daemonProcessIdentity, error)
}

func checkDaemonStatus() daemonStatus {
	return checkDaemonStatusWithDeps(daemonStatusDeps{
		paths:          currentDaemonPaths,
		readInstance:   daemonpaths.ReadInstance,
		probeStatus:    probeDaemonStatus,
		processAlive:   processAlive,
		inspectProcess: daemonpaths.InspectProcess,
	})
}

func checkDaemonStatusWithDeps(deps daemonStatusDeps) daemonStatus {
	paths, err := deps.paths()
	if err != nil {
		return daemonStatus{OK: false, Code: daemonStatusInstanceUnreadable, MaxConnections: maxConnections, Message: err.Error()}
	}
	status := daemonStatus{OK: true, Code: daemonStatusStopped, Paths: paths, MaxConnections: maxConnections, Message: "daemon is not running"}
	record, readErr := deps.readInstance(paths.PID)
	var probe daemonIdentityResponse
	probed := false
	if readErr != nil {
		var probeErr error
		probe, probeErr = probeDaemonStatusWithDeps(deps, paths.Socket)
		if probeErr == nil {
			probed = true
			record, readErr = deps.readInstance(paths.PID)
		}
		if readErr != nil && probeErr == nil {
			observed := probe.Instance
			status.Running = true
			status.Reachable = true
			status.PID = observed.PID
			status.Instance = &observed
			applyDaemonAdmissionStatus(&status, probe)
			return daemonIdentityMismatchStatus(status, "daemon socket is reachable without a matching instance record")
		}
		if readErr != nil && !os.IsNotExist(readErr) {
			status.OK = false
			status.Code = daemonStatusInstanceUnreadable
			status.Message = readErr.Error()
		}
		if readErr != nil {
			return status
		}
	}
	status.PID = record.PID
	status.Instance = &record

	if !probed {
		var probeErr error
		probe, probeErr = probeDaemonStatusWithDeps(deps, paths.Socket)
		if probeErr != nil {
			alive := record.PID > 0 && deps.processAlive(record.PID)
			status.Running = alive
			if alive {
				status.OK = false
				status.Code = daemonStatusSocketUnreachable
				status.Message = "daemon pid exists but socket is not reachable"
			}
			return status
		}
	}
	observed := probe.Instance
	status.Running = true
	status.Reachable = true
	applyDaemonAdmissionStatus(&status, probe)
	if observed != record {
		return daemonIdentityMismatchStatus(status, "daemon socket identity does not match instance record")
	}
	if !deps.processAlive(record.PID) {
		return daemonIdentityMismatchStatus(status, "daemon process is not alive")
	}
	process, err := deps.inspectProcess(record.PID)
	if err != nil {
		return daemonIdentityMismatchStatus(status, fmt.Sprintf("inspect daemon process identity: %v", err))
	}
	if !daemonProcessIdentityMatches(record, process) {
		return daemonIdentityMismatchStatus(status, "daemon OS process identity does not match instance record")
	}
	status.OK = true
	status.IdentityVerified = true
	status.Code = daemonStatusReady
	status.Message = "daemon is reachable and identity verified"
	return status
}

func probeDaemonStatusWithDeps(deps daemonStatusDeps, socket string) (daemonIdentityResponse, error) {
	if deps.probeStatus != nil {
		return deps.probeStatus(socket)
	}
	instance, err := deps.probeIdentity(socket)
	if err != nil {
		return daemonIdentityResponse{}, err
	}
	return daemonIdentityResponse{
		OK:             true,
		Instance:       instance,
		MaxConnections: maxConnections,
		Accepting:      true,
	}, nil
}

func applyDaemonAdmissionStatus(status *daemonStatus, probe daemonIdentityResponse) {
	status.ActiveConnections = probe.ActiveConnections
	status.MaxConnections = probe.MaxConnections
	status.Accepting = probe.Accepting
	status.Draining = probe.Draining
}

func daemonIdentityMismatchStatus(status daemonStatus, message string) daemonStatus {
	status.OK = false
	status.IdentityVerified = false
	status.Code = daemonStatusIdentityMismatch
	status.Message = message
	return status
}

func daemonProcessIdentityMatches(instance daemonInstance, process daemonProcessIdentity) bool {
	if !daemonpaths.ProcessStartTimeEqual(instance.ProcessStartTime, process.StartTime) {
		return false
	}
	// Linux의 /proc/<pid>/exe는 실행 중인 image를 가리키므로 executable 불일치는
	// 다른 프로세스라는 안정된 증거다. Darwin과 그 밖의 플랫폼에서 ps comm은
	// launcher 경로를 보존하고 EvalSymlinks는 관측 시점의 대상을 해석한다.
	// 따라서 `ah update`가 구 daemon 실행 중 symlink를 새 binary로 옮겨도
	// executable projection만 바뀔 수 있다. 위의 file/socket handshake가
	// executable, nonce, build, protocol, generation을 봉인하고 start time이
	// signal 직전 PID 재사용을 막는다.
	return !process.ExecutablePathStable || instance.Executable == process.Executable
}

func daemonStatusIsReady(status daemonStatus) bool {
	return status.OK && status.Running && status.Reachable && status.IdentityVerified && status.Code == daemonStatusReady && status.Instance != nil && status.PID == status.Instance.PID
}

func daemonStatusBlocksStart(status daemonStatus) bool {
	if daemonStatusIsReady(status) {
		return false
	}
	if status.OK && status.Code == daemonStatusStopped && !status.Running && !status.Reachable {
		return false
	}
	return status.PID > 0 || status.Running || status.Reachable || (status.Code != "" && status.Code != daemonStatusStopped)
}

type daemonProcess interface {
	Signal(os.Signal) error
	Kill() error
}

type daemonStopDeps struct {
	checkStatus    func() daemonStatus
	findProcess    func(int) (daemonProcess, error)
	inspectProcess func(int) (daemonProcessIdentity, error)
	processAlive   func(int) bool
	remove         func(string) error
	now            func() time.Time
	sleep          func(time.Duration)
}

func stopDaemon() (daemonStatus, error) {
	return stopDaemonCoordinatedWithDeps(daemonStopCoordinatorDeps{
		paths:    currentDaemonPaths,
		mkdirAll: os.MkdirAll,
		acquireLock: func(paths daemonPaths) (daemonStartLock, error) {
			return acquireDaemonLock(paths)
		},
		remove: os.Remove,
		stop: func() (daemonStatus, error) {
			return stopDaemonWithDeps(daemonStopDeps{
				checkStatus: checkDaemonStatus,
				findProcess: func(pid int) (daemonProcess, error) {
					return os.FindProcess(pid)
				},
				inspectProcess: daemonpaths.InspectProcess,
				processAlive:   processAlive,
				remove:         os.Remove,
				now:            time.Now,
				sleep:          time.Sleep,
			})
		},
	})
}

type daemonStopCoordinatorDeps struct {
	paths       func() (daemonPaths, error)
	mkdirAll    func(string, os.FileMode) error
	acquireLock func(daemonPaths) (daemonStartLock, error)
	remove      func(string) error
	stop        func() (daemonStatus, error)
}

func stopDaemonCoordinatedWithDeps(deps daemonStopCoordinatorDeps) (daemonStatus, error) {
	paths, err := deps.paths()
	if err != nil {
		return daemonStatus{}, err
	}
	if err := deps.mkdirAll(paths.Dir, 0o700); err != nil {
		return daemonStatus{}, err
	}
	lock, err := deps.acquireLock(paths)
	if err != nil {
		return daemonStatus{OK: false, Paths: paths, Message: err.Error()}, err
	}
	defer func() {
		_ = lock.Close()
		_ = deps.remove(paths.Lock)
	}()
	return deps.stop()
}

func stopDaemonWithDeps(deps daemonStopDeps) (daemonStatus, error) {
	status := deps.checkStatus()
	if status.Code == daemonStatusStopped && status.PID == 0 && !status.Reachable {
		status.OK = true
		status.Running = false
		status.Message = "agent-harness daemon already stopped"
		return status, nil
	}
	// Stale artifacts: the pid file references a dead process and the socket
	// is absent. Stopping is idempotent here — clean the stale files instead
	// of refusing, so post-install refresh (stop → start) cannot wedge on a
	// leftover pid file from a crashed or externally killed daemon.
	if status.Code == daemonStatusStopped && !status.Running && !status.Reachable && status.PID > 0 && !deps.processAlive(status.PID) {
		return daemonStoppedStatus(status, status.PID, deps.remove, "agent-harness daemon already stopped (cleaned stale pid file)"), nil
	}
	if !daemonStatusCanStop(status) {
		status.OK = false
		return status, fmt.Errorf("refusing to stop unverified daemon: %s", status.Code)
	}
	instance := *status.Instance
	proc, err := deps.findProcess(instance.PID)
	if err != nil {
		status.OK = false
		status.Message = err.Error()
		return status, err
	}
	processIdentity, err := deps.inspectProcess(instance.PID)
	if err != nil || !daemonProcessIdentityMatches(instance, processIdentity) {
		status = daemonIdentityMismatchStatus(status, "daemon OS process identity changed before stop")
		return status, fmt.Errorf("refusing to signal unverified daemon process")
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		status.OK = false
		status.Message = err.Error()
		return status, err
	}
	deadline := deps.now().Add(3 * time.Second)
	for deps.now().Before(deadline) {
		if !deps.processAlive(instance.PID) {
			return daemonStoppedStatus(status, instance.PID, deps.remove, "agent-harness daemon stopped"), nil
		}
		deps.sleep(50 * time.Millisecond)
	}
	if !deps.processAlive(instance.PID) {
		return daemonStoppedStatus(status, instance.PID, deps.remove, "agent-harness daemon stopped"), nil
	}
	processIdentity, err = deps.inspectProcess(instance.PID)
	if err != nil || !daemonProcessIdentityMatches(instance, processIdentity) {
		// TERM 처리 직후 종료된 프로세스를 PID 재사용으로 오인하지 않도록
		// 강제 종료 직전의 생존 상태를 다시 확인한다.
		if !deps.processAlive(instance.PID) {
			return daemonStoppedStatus(status, instance.PID, deps.remove, "agent-harness daemon stopped"), nil
		}
		status = daemonIdentityMismatchStatus(status, "daemon OS process identity changed before forced stop")
		return status, fmt.Errorf("refusing to kill unverified daemon process")
	}
	if err := proc.Kill(); err != nil {
		status.OK = false
		status.Message = err.Error()
		return status, err
	}
	return daemonStoppedStatus(status, instance.PID, deps.remove, "agent-harness daemon killed after timeout"), nil
}

func daemonStatusCanStop(status daemonStatus) bool {
	if daemonStatusIsReady(status) {
		return true
	}
	return status.Code == daemonStatusSocketUnreachable &&
		status.Running &&
		!status.Reachable &&
		status.Instance != nil &&
		status.PID > 0 &&
		status.PID == status.Instance.PID
}

func daemonStoppedStatus(status daemonStatus, pid int, remove func(string) error, message string) daemonStatus {
	_ = remove(status.Paths.Socket)
	_ = remove(status.Paths.PID)
	status.OK = true
	status.Running = false
	status.Reachable = false
	status.IdentityVerified = false
	status.PID = pid
	status.Code = daemonStatusStopped
	status.Message = message
	return status
}

func daemonStatusForMCP() daemonStatus {
	return checkDaemonStatus()
}
