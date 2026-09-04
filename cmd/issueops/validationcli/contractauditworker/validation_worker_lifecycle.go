package contractauditworker

import (
	"encoding/json"
	"fmt"
	"time"

	"issueops/cmd/issueops/commandstep"
	workercontract "issueops/internal/contract/worker"
)

func ValidateWorkerLifecycle(binary, root string, seed int64) StepResult {
	return ValidateWorkerLifecycleWithDeps(binary, root, seed, ValidationDeps{})
}

func ValidateWorkerLifecycleWithDeps(binary, root string, seed int64, deps ValidationDeps) StepResult {
	deps = deps.withDefaults()
	workerDir, err := deps.MkdirTemp("", "issueops-worker-*")
	if err != nil {
		return commandstep.FailedStep("worker lifecycle smoke", err)
	}
	defer func() { _ = deps.RemoveAll(workerDir) }()
	env := []string{"ISSUEOPS_WORKER_DIR=" + workerDir}
	enqueue := deps.RunCommandStepEnv(root, "worker lifecycle enqueue", 30*time.Second, "", env, binary, "worker", "enqueue", "--kind", "smoke", "--payload", fmt.Sprintf("seed=%d", seed), "--json")
	if !enqueue.OK {
		return enqueue
	}
	var job workercontract.WorkerJob
	if err := json.Unmarshal([]byte(enqueue.Stdout), &job); err != nil {
		return commandstep.FailedStep("worker lifecycle smoke", err)
	}
	status := deps.RunCommandStepEnv(root, "worker lifecycle status", 30*time.Second, "", env, binary, "worker", "status", "--id", job.ID, "--json")
	cancel := deps.RunCommandStepEnv(root, "worker lifecycle cancel", 30*time.Second, "", env, binary, "worker", "cancel", "--id", job.ID, "--json")
	list := deps.RunCommandStepEnv(root, "worker lifecycle list", 30*time.Second, "", env, binary, "worker", "list", "--json")
	errs := []string{}
	for _, step := range []StepResult{status, cancel, list} {
		if !step.OK {
			errs = append(errs, step.Label+" failed")
		}
	}
	if !job.NoShell || job.Status != workercontract.WorkerStatusQueued {
		errs = append(errs, "worker job is not queued no-shell")
	}
	return commandstep.AssertionStep("worker lifecycle smoke", time.Now(), errs)
}
