package main

import (
	"encoding/json"
	"fmt"
	"time"

	"agent-harness/internal/core"
)

func validateWorkerLifecycle(binary, root string, seed int64) StepResult {
	return validateWorkerLifecycleWithDeps(binary, root, seed, contractAuditWorkerValidationDeps{})
}

func validateWorkerLifecycleWithDeps(binary, root string, seed int64, deps contractAuditWorkerValidationDeps) StepResult {
	deps = deps.withDefaults()
	workerDir, err := deps.mkdirTemp("", "agent-harness-worker-*")
	if err != nil {
		return failedStep("worker lifecycle smoke", err)
	}
	defer deps.removeAll(workerDir)
	env := []string{"HARNESS_WORKER_DIR=" + workerDir}
	enqueue := deps.runCommandStepEnv(root, "worker lifecycle enqueue", 30*time.Second, "", env, binary, "worker", "enqueue", "--kind", "smoke", "--payload", fmt.Sprintf("seed=%d", seed), "--json")
	if !enqueue.OK {
		return enqueue
	}
	var job core.WorkerJob
	if err := json.Unmarshal([]byte(enqueue.Stdout), &job); err != nil {
		return failedStep("worker lifecycle smoke", err)
	}
	status := deps.runCommandStepEnv(root, "worker lifecycle status", 30*time.Second, "", env, binary, "worker", "status", "--id", job.ID, "--json")
	cancel := deps.runCommandStepEnv(root, "worker lifecycle cancel", 30*time.Second, "", env, binary, "worker", "cancel", "--id", job.ID, "--json")
	list := deps.runCommandStepEnv(root, "worker lifecycle list", 30*time.Second, "", env, binary, "worker", "list", "--json")
	errs := []string{}
	for _, step := range []StepResult{status, cancel, list} {
		if !step.OK {
			errs = append(errs, step.Label+" failed")
		}
	}
	if !job.NoShell || job.Status != core.WorkerStatusQueued {
		errs = append(errs, "worker job is not queued no-shell")
	}
	return assertionStep("worker lifecycle smoke", time.Now(), errs)
}
