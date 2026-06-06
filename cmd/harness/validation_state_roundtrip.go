package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"agent-harness/internal/core"
)

type stateRoundtripCommandRunner func(root, label string, timeout time.Duration, input string, env []string, command ...string) StepResult

type stateRoundtripValidationDeps struct {
	mkdirTemp     func(string, string) (string, error)
	removeAll     func(string) error
	writeFile     func(string, []byte, os.FileMode) error
	stateRead     func(string) (core.StateResult, error)
	writeSnapshot func(string, string, SelfAugmentStateSnapshot) error
	run           stateRoundtripCommandRunner
}

func (deps stateRoundtripValidationDeps) withDefaults() stateRoundtripValidationDeps {
	if deps.mkdirTemp == nil {
		deps.mkdirTemp = os.MkdirTemp
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.writeFile == nil {
		deps.writeFile = os.WriteFile
	}
	if deps.stateRead == nil {
		deps.stateRead = core.StateRead
	}
	if deps.writeSnapshot == nil {
		deps.writeSnapshot = writeSelfAugmentSnapshotRecord
	}
	if deps.run == nil {
		deps.run = func(root, label string, timeout time.Duration, input string, env []string, command ...string) StepResult {
			if len(command) == 0 {
				return failedStep(label, fmt.Errorf("missing command"))
			}
			return runCommandStepEnv(root, label, timeout, input, env, command[0], command[1:]...)
		}
	}
	return deps
}

func validateStateRoundtrip(binary, root string, seed int64) StepResult {
	return validateStateRoundtripWithDeps(binary, root, seed, stateRoundtripValidationDeps{})
}

func validateStateRoundtripWithDeps(binary, root string, seed int64, deps stateRoundtripValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	tempState, err := deps.mkdirTemp("", "agent-harness-state-roundtrip-*")
	if err != nil {
		return failedStep("state roundtrip", err)
	}
	defer deps.removeAll(tempState)

	key := fmt.Sprintf("self-verify-%d", seed)
	content := fmt.Sprintf("seed=%d\nLore: state roundtrip\n", seed)
	env := []string{"HARNESS_STATE_DIR=" + tempState}
	stdoutParts := []string{}
	commands := []string{}
	run := func(label string, command ...string) StepResult {
		step := deps.run(root, label, 30*time.Second, "", env, command...)
		stdoutParts = append(stdoutParts, step.Stdout)
		commands = append(commands, step.Command)
		return step
	}
	fail := func(errs ...string) StepResult {
		return assertionStepWithOutput("state roundtrip", started, errs, stdoutParts, commands)
	}

	write := run("state write", binary, "state", "write", "--key", key, "--value", content, "--json")
	if !write.OK {
		return combineFailedStep("state roundtrip", started, write, stdoutParts, commands)
	}
	var writeResult core.StateResult
	if err := json.Unmarshal([]byte(write.Stdout), &writeResult); err != nil {
		return fail(err.Error())
	}
	if !writeResult.OK || writeResult.Record.Key != key || writeResult.Record.Content != content || writeResult.Record.Bytes != len([]byte(content)) {
		return fail("write result did not match expected record")
	}

	read := run("state read", binary, "state", "read", "--key", key, "--json")
	if !read.OK {
		return combineFailedStep("state roundtrip", started, read, stdoutParts, commands)
	}
	var readResult core.StateResult
	if err := json.Unmarshal([]byte(read.Stdout), &readResult); err != nil {
		return fail(err.Error())
	}
	if !readResult.OK || readResult.Record.Key != key || readResult.Record.Content != content || readResult.Record.Bytes != len([]byte(content)) {
		return fail("read result did not match expected record")
	}

	list := run("state list", binary, "state", "list", "--json")
	if !list.OK {
		return combineFailedStep("state roundtrip", started, list, stdoutParts, commands)
	}
	var listResult core.StateListResult
	if err := json.Unmarshal([]byte(list.Stdout), &listResult); err != nil {
		return fail(err.Error())
	}
	if !listResult.OK || !containsString(listResult.Keys, key) {
		return fail("state list did not include roundtrip key")
	}

	oldKey := key + "-old"
	oldWrite := run("state old write", binary, "state", "write", "--key", oldKey, "--value", "old state", "--json")
	if !oldWrite.OK {
		return combineFailedStep("state roundtrip", started, oldWrite, stdoutParts, commands)
	}
	var oldWriteResult core.StateResult
	if err := json.Unmarshal([]byte(oldWrite.Stdout), &oldWriteResult); err != nil {
		return fail(err.Error())
	}
	oldWriteResult.Record.UpdatedAt = "2000-01-01T00:00:00Z"
	b, err := json.MarshalIndent(oldWriteResult.Record, "", "  ")
	if err != nil {
		return fail(err.Error())
	}
	if err := deps.writeFile(oldWriteResult.Path, append(b, '\n'), 0o600); err != nil {
		return fail(err.Error())
	}

	pruneDry := run("state prune dry-run", binary, "state", "prune", "--max-age", "1h", "--json")
	if !pruneDry.OK {
		return combineFailedStep("state roundtrip", started, pruneDry, stdoutParts, commands)
	}
	var pruneDryResult core.StatePruneResult
	if err := json.Unmarshal([]byte(pruneDry.Stdout), &pruneDryResult); err != nil {
		return fail(err.Error())
	}
	if !pruneDryResult.OK || !pruneDryResult.DryRun || !containsString(pruneDryResult.DeletedKeys, oldKey) || !containsString(pruneDryResult.KeptKeys, key) {
		return fail("state prune dry-run did not classify old/fresh keys")
	}

	pruneConfirm := run("state prune confirm", binary, "state", "prune", "--max-age", "1h", "--confirm", "--json")
	if !pruneConfirm.OK {
		return combineFailedStep("state roundtrip", started, pruneConfirm, stdoutParts, commands)
	}
	var pruneConfirmResult core.StatePruneResult
	if err := json.Unmarshal([]byte(pruneConfirm.Stdout), &pruneConfirmResult); err != nil {
		return fail(err.Error())
	}
	if !pruneConfirmResult.OK || pruneConfirmResult.DryRun || !pruneConfirmResult.Confirm || !containsString(pruneConfirmResult.DeletedKeys, oldKey) {
		return fail("state prune confirm did not delete old key")
	}

	listAfterPrune := run("state list after prune", binary, "state", "list", "--json")
	if !listAfterPrune.OK {
		return combineFailedStep("state roundtrip", started, listAfterPrune, stdoutParts, commands)
	}
	var listAfterPruneResult core.StateListResult
	if err := json.Unmarshal([]byte(listAfterPrune.Stdout), &listAfterPruneResult); err != nil {
		return fail(err.Error())
	}
	if !containsString(listAfterPruneResult.Keys, key) || containsString(listAfterPruneResult.Keys, oldKey) {
		return fail("state prune did not preserve fresh key and remove old key")
	}

	legacyKey := key + "-legacy"
	legacyRecord := core.StateRecord{Key: legacyKey, Content: "legacy state", UpdatedAt: "2000-01-01T00:00:00Z", Bytes: len([]byte("legacy state"))}
	legacyBytes, err := json.MarshalIndent(legacyRecord, "", "  ")
	if err != nil {
		return fail(err.Error())
	}
	if err := deps.writeFile(filepath.Join(tempState, legacyKey+".json"), append(legacyBytes, '\n'), 0o600); err != nil {
		return fail(err.Error())
	}

	migrateDry := run("state migrate dry-run", binary, "state", "migrate", "--json")
	if !migrateDry.OK {
		return combineFailedStep("state roundtrip", started, migrateDry, stdoutParts, commands)
	}
	var migrateDryResult core.StateMigrateResult
	if err := json.Unmarshal([]byte(migrateDry.Stdout), &migrateDryResult); err != nil {
		return fail(err.Error())
	}
	if !migrateDryResult.OK || !migrateDryResult.DryRun || !containsString(migrateDryResult.CandidateKeys, legacyKey) || len(migrateDryResult.MigratedKeys) != 0 {
		return fail("state migrate dry-run did not classify legacy key")
	}

	migrateConfirm := run("state migrate confirm", binary, "state", "migrate", "--confirm", "--json")
	if !migrateConfirm.OK {
		return combineFailedStep("state roundtrip", started, migrateConfirm, stdoutParts, commands)
	}
	var migrateConfirmResult core.StateMigrateResult
	if err := json.Unmarshal([]byte(migrateConfirm.Stdout), &migrateConfirmResult); err != nil {
		return fail(err.Error())
	}
	if !migrateConfirmResult.OK || migrateConfirmResult.DryRun || !migrateConfirmResult.Confirm || !containsString(migrateConfirmResult.MigratedKeys, legacyKey) {
		return fail("state migrate confirm did not migrate legacy key")
	}

	migratedRead := run("state migrated read", binary, "state", "read", "--key", legacyKey, "--json")
	if !migratedRead.OK {
		return combineFailedStep("state roundtrip", started, migratedRead, stdoutParts, commands)
	}
	var migratedReadResult core.StateResult
	if err := json.Unmarshal([]byte(migratedRead.Stdout), &migratedReadResult); err != nil {
		return fail(err.Error())
	}
	if migratedReadResult.Record.SchemaVersion != core.StateCurrentSchemaVersion || migratedReadResult.Record.Content != legacyRecord.Content {
		return fail("state migrate did not preserve content or set current schema")
	}

	doctorHealthy := run("state doctor after migrate", binary, "state", "doctor", "--json")
	if !doctorHealthy.OK {
		return combineFailedStep("state roundtrip", started, doctorHealthy, stdoutParts, commands)
	}
	var doctorHealthyResult core.StateDoctorResult
	if err := json.Unmarshal([]byte(doctorHealthy.Stdout), &doctorHealthyResult); err != nil {
		return fail(err.Error())
	}
	if !doctorHealthyResult.OK || !doctorHealthyResult.Healthy {
		return fail("state doctor was not healthy after migrating legacy fixture")
	}

	return validateStateRoundtripSelfVerifyDeps(validateStateRoundtripSelfVerifyInput{
		binary:      binary,
		root:        root,
		seed:        seed,
		tempState:   tempState,
		key:         key,
		env:         env,
		started:     started,
		stdoutParts: stdoutParts,
		commands:    commands,
		deps:        deps,
	})
}
