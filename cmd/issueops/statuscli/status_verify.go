package statuscli

import (
	"flag"
	"fmt"
	doctorcontract "issueops/internal/contract/doctor"
	"os"
	"strings"

	"issueops/cmd/issueops/daemoncli"
	inspect "issueops/internal/contract/inspect"
	statecontract "issueops/internal/contract/state"
	workercontract "issueops/internal/contract/worker"
)

type HarnessStatus struct {
	OK         bool                               `json:"ok"`
	Kind       string                             `json:"kind"`
	Version    string                             `json:"version"`
	Repo       string                             `json:"repo"`
	Inspect    inspect.InspectInfo                `json:"inspect"`
	Doctor     doctorcontract.HarnessDoctorResult `json:"doctor"`
	Daemon     daemoncli.Status                   `json:"daemon"`
	State      statecontract.StateListResult      `json:"state"`
	Workers    workercontract.WorkerListResult    `json:"workers"`
	SelfVerify SelfVerifyStatus                   `json:"self_verify"`
	Warnings   []string                           `json:"warnings"`
}

type SelfVerifyStatus struct {
	LatestKey string `json:"latest_key,omitempty"`
	Found     bool   `json:"found"`
	UpdatedAt string `json:"updated_at,omitempty"`
	Bytes     int    `json:"bytes,omitempty"`
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		*repo = fs.Arg(0)
	}
	status := buildHarnessStatus(*repo)
	if *jsonOut {
		return printJSON(status)
	}
	fmt.Printf("issueops system-status: ok=%v repo=%s\n", status.OK, status.Repo)
	fmt.Printf("doctor healthy: %v\n", status.Doctor.Healthy)
	fmt.Printf("daemon running: %v (%s)\n", status.Daemon.Running, status.Daemon.Message)
	fmt.Printf("state records: %d\n", len(status.State.Records))
	fmt.Printf("worker jobs: %d\n", len(status.Workers.Jobs))
	for _, warning := range status.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	return nil
}

func buildHarnessStatus(repo string) HarnessStatus {
	home, _ := os.UserHomeDir()
	inspect := deps.InspectHarness(repo)
	daemon := deps.CheckDaemonStatus()
	doctor, doctorErr := harnessDoctor(doctorcontract.HarnessDoctorRequest{
		RepoRoot:     repo,
		IssueOpsRoot: deps.IssueOpsRoot(),
		Home:         home,
		Version:      deps.Version,
		DaemonAdmission: doctorcontract.HarnessDoctorDaemonAdmission{
			Observed:          daemon.Running && daemon.Reachable && daemon.IdentityVerified,
			ActiveConnections: daemon.ActiveConnections,
			MaxConnections:    daemon.MaxConnections,
			Accepting:         daemon.Accepting,
			Draining:          daemon.Draining,
		},
	})
	state, stateErr := StateList()
	workers, workerErr := ListWorkerJobs()
	warnings := []string{}
	if doctorErr != nil {
		warnings = append(warnings, "doctor: "+doctorErr.Error())
	}
	if stateErr != nil {
		warnings = append(warnings, "state: "+stateErr.Error())
	}
	if workerErr != nil {
		warnings = append(warnings, "workers: "+workerErr.Error())
	}
	selfVerify := SelfVerifyStatus{}
	for _, record := range state.Records {
		if record.Key == "self-verify-latest" || strings.HasPrefix(record.Key, "self-verify") {
			selfVerify = SelfVerifyStatus{LatestKey: record.Key, Found: true, UpdatedAt: record.UpdatedAt, Bytes: record.Bytes}
			break
		}
	}
	ok := len(warnings) == 0 && doctor.OK && state.OK && workers.OK
	return HarnessStatus{
		OK:         ok,
		Kind:       "harness_status",
		Version:    deps.Version,
		Repo:       deps.ResolveTarget(repo),
		Inspect:    inspect,
		Doctor:     doctor,
		Daemon:     daemon,
		State:      state,
		Workers:    workers,
		SelfVerify: selfVerify,
		Warnings:   warnings,
	}
}
