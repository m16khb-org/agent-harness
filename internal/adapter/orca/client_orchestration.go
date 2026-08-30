package orca

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"agent-harness/internal/port"
	"golang.org/x/sync/errgroup"
)

func (c *Client) ListRuns(ctx context.Context) ([]port.OrcaRun, error) {
	inventory, err := c.listRunsInventory(ctx)
	return inventory.Rows, err
}

func (c *Client) ListRunInventory(ctx context.Context) (port.OrcaRunInventory, error) {
	status, err := c.Status(ctx)
	if err != nil {
		return port.OrcaRunInventory{}, err
	}
	runs, err := c.listRunsInventory(ctx)
	if err != nil {
		return port.OrcaRunInventory{}, err
	}
	if err := validateExecutionInventoryRuntime(runs.RuntimeID, status.RuntimeID); err != nil {
		return port.OrcaRunInventory{}, err
	}
	return port.OrcaRunInventory{RuntimeID: runs.RuntimeID, Runs: runs.Rows}, nil
}

func (c *Client) ListAllTasksFromRuns(ctx context.Context, inventory port.OrcaRunInventory) ([]port.OrcaTask, error) {
	return c.listTasksFromRuns(ctx, inventory, "--brief")
}

func (c *Client) ListDispatchedTasksFromRuns(ctx context.Context, inventory port.OrcaRunInventory) ([]port.OrcaTask, error) {
	return c.listTasksFromRuns(ctx, inventory, "--status", "dispatched")
}

func (c *Client) ListGatesFromRuns(ctx context.Context, inventory port.OrcaRunInventory) ([]port.OrcaGate, error) {
	runs, err := validateRunInventory(inventory)
	if err != nil {
		return nil, err
	}
	entries, errs := readRunsBounded(ctx, runs, func(ctx context.Context, run port.OrcaRun) (executionGateInventory, error) {
		return c.listRunGatesInventory(ctx, run.ID)
	})
	result := make([]port.OrcaGate, 0)
	seen := make(map[string]struct{})
	for index, entry := range entries {
		if errs[index] != nil {
			return nil, errs[index]
		}
		if err := validateExecutionInventoryRuntime(entry.RuntimeID, inventory.RuntimeID); err != nil {
			return nil, err
		}
		for _, gate := range entry.Rows {
			if _, duplicate := seen[gate.ID]; duplicate {
				return nil, &port.OrcaError{Code: "gate_inventory_ambiguous", Detail: "Orca returned a duplicate gate identity", Invoked: true}
			}
			seen[gate.ID] = struct{}{}
			result = append(result, gate)
		}
	}
	return result, nil
}

func (c *Client) listTasksFromRuns(ctx context.Context, inventory port.OrcaRunInventory, flags ...string) ([]port.OrcaTask, error) {
	runs, err := validateRunInventory(inventory)
	if err != nil {
		return nil, err
	}
	entries, errs := readRunsBounded(ctx, runs, func(ctx context.Context, run port.OrcaRun) (executionTaskInventory, error) {
		return c.listRunTasksInventory(ctx, run.ID, flags...)
	})
	result := make([]port.OrcaTask, 0)
	seen := make(map[string]struct{})
	for index, entry := range entries {
		if errs[index] != nil {
			return nil, errs[index]
		}
		if err := validateExecutionInventoryRuntime(entry.RuntimeID, inventory.RuntimeID); err != nil {
			return nil, err
		}
		for _, task := range entry.Rows {
			key := task.RunID + "\x00" + task.ID
			if _, duplicate := seen[key]; duplicate {
				return nil, &port.OrcaError{Code: "task_inventory_ambiguous", Detail: "Orca returned a duplicate task identity in one Run", Invoked: true}
			}
			seen[key] = struct{}{}
			result = append(result, task)
		}
	}
	return result, nil
}

func validateRunInventory(inventory port.OrcaRunInventory) ([]port.OrcaRun, error) {
	if strings.TrimSpace(inventory.RuntimeID) == "" || inventory.RuntimeID != strings.TrimSpace(inventory.RuntimeID) {
		return nil, &port.OrcaError{Code: "run_inventory_runtime_invalid", Detail: "Orca Run inventory requires a canonical runtime identity"}
	}
	runs := append([]port.OrcaRun(nil), inventory.Runs...)
	seen := make(map[string]struct{}, len(runs))
	for _, run := range runs {
		if _, err := validateRunID(run.ID); err != nil || run.RuntimeID != inventory.RuntimeID || strings.TrimSpace(run.Objective) == "" || run.Objective != strings.TrimSpace(run.Objective) {
			return nil, &port.OrcaError{Code: "run_inventory_identity_invalid", Detail: "Orca Run inventory contains an invalid Run identity"}
		}
		if _, duplicate := seen[run.ID]; duplicate {
			return nil, &port.OrcaError{Code: "run_inventory_ambiguous", Detail: "Orca Run inventory contains a duplicate Run identity"}
		}
		seen[run.ID] = struct{}{}
	}
	return runs, nil
}

func readRunsBounded[T any](ctx context.Context, runs []port.OrcaRun, read func(context.Context, port.OrcaRun) (T, error)) ([]T, []error) {
	values := make([]T, len(runs))
	errs := make([]error, len(runs))
	workers := min(8, len(runs))
	if workers == 0 {
		return values, errs
	}
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(workers)
	for index := range runs {
		index := index
		group.Go(func() error {
			values[index], errs[index] = read(groupCtx, runs[index])
			return nil
		})
	}
	_ = group.Wait()
	return values, errs
}

func (c *Client) listRunsInventory(ctx context.Context) (executionRunInventory, error) {
	argv := []string{"orca", "orchestration", "run-list", "--json"}
	result := make([]port.OrcaRun, 0)
	seenRuns := make(map[string]struct{})
	seenCursors := make(map[string]struct{})
	runtimeID := ""
	for {
		var payload struct {
			Runs       *[]runPayload   `json:"runs"`
			NextCursor json.RawMessage `json:"nextCursor"`
		}
		pageRuntimeID, err := c.runJSON(ctx, "", readTimeout, argv, &payload)
		if err != nil {
			return executionRunInventory{}, err
		}
		if strings.TrimSpace(pageRuntimeID) == "" || pageRuntimeID != strings.TrimSpace(pageRuntimeID) {
			return executionRunInventory{}, &port.OrcaError{Code: "run_inventory_runtime_invalid", Detail: "Orca Run list page has no canonical runtime identity", Invoked: true}
		}
		if runtimeID == "" {
			runtimeID = pageRuntimeID
		} else if err := validateExecutionInventoryRuntime(pageRuntimeID, runtimeID); err != nil {
			return executionRunInventory{}, err
		}
		if payload.Runs == nil {
			return executionRunInventory{}, &port.OrcaError{Code: "incomplete_list", Detail: "Orca Run list completeness metadata is missing", Invoked: true}
		}
		for _, row := range *payload.Runs {
			value, err := row.portValue(runtimeID)
			if err != nil {
				return executionRunInventory{}, err
			}
			if _, duplicate := seenRuns[value.ID]; duplicate {
				return executionRunInventory{}, &port.OrcaError{Code: "run_inventory_ambiguous", Detail: "Orca returned a duplicate Run identity", Invoked: true}
			}
			seenRuns[value.ID] = struct{}{}
			result = append(result, value)
		}
		nextCursor := strings.TrimSpace(string(payload.NextCursor))
		if nextCursor == "" {
			return executionRunInventory{}, &port.OrcaError{Code: "incomplete_list", Detail: "Orca Run list nextCursor completeness metadata is missing", Invoked: true}
		}
		if nextCursor == "null" {
			break
		}
		var cursor string
		if json.Unmarshal(payload.NextCursor, &cursor) != nil || strings.TrimSpace(cursor) == "" || cursor != strings.TrimSpace(cursor) {
			return executionRunInventory{}, &port.OrcaError{Code: "incomplete_list", Detail: "Orca Run list nextCursor is invalid", Invoked: true}
		}
		if _, duplicate := seenCursors[cursor]; duplicate {
			return executionRunInventory{}, &port.OrcaError{Code: "incomplete_list", Detail: "Orca Run list nextCursor repeated", Invoked: true}
		}
		seenCursors[cursor] = struct{}{}
		argv = []string{"orca", "orchestration", "run-list", "--cursor", cursor, "--json"}
	}
	slices.SortFunc(result, func(left, right port.OrcaRun) int {
		return strings.Compare(left.ID, right.ID)
	})
	return executionRunInventory{RuntimeID: runtimeID, Rows: result}, nil
}

func (c *Client) CreateRun(ctx context.Context, req port.OrcaCreateRunRequest) (port.OrcaRun, error) {
	objective := strings.TrimSpace(req.Objective)
	if objective == "" || objective != req.Objective || len(objective) > 4096 || strings.ContainsRune(objective, 0) {
		return port.OrcaRun{}, &port.OrcaError{Code: "run_objective_invalid"}
	}
	if _, err := currentCoordinatorHandle(); err != nil {
		return port.OrcaRun{}, err
	}
	return c.runMutation(ctx, []string{"orca", "orchestration", "run-create", "--objective", objective, "--json"})
}

func (c *Client) CurrentRun(ctx context.Context) (*port.OrcaRun, error) {
	inventory, err := c.currentRunInventory(ctx)
	return inventory.Run, err
}

func (c *Client) currentRunInventory(ctx context.Context) (executionCurrentRunInventory, error) {
	if _, err := currentCoordinatorHandle(); err != nil {
		return executionCurrentRunInventory{}, err
	}
	var payload struct {
		Run json.RawMessage `json:"run"`
	}
	runtimeID, err := c.runJSON(ctx, "", readTimeout, []string{"orca", "orchestration", "run-current", "--json"}, &payload)
	if err != nil {
		return executionCurrentRunInventory{RuntimeID: runtimeID}, err
	}
	projection := strings.TrimSpace(string(payload.Run))
	if projection == "" {
		return executionCurrentRunInventory{RuntimeID: runtimeID}, &port.OrcaError{Code: "incomplete_run_current", Detail: "Orca current Run projection is missing", Invoked: true}
	}
	if projection == "null" {
		return executionCurrentRunInventory{RuntimeID: runtimeID}, nil
	}
	var row runPayload
	if err := json.Unmarshal(payload.Run, &row); err != nil {
		return executionCurrentRunInventory{RuntimeID: runtimeID}, &port.OrcaError{Code: "run_identity_invalid", Detail: "Orca current Run projection is malformed", Invoked: true}
	}
	value, err := row.portValue(runtimeID)
	if err != nil {
		return executionCurrentRunInventory{}, err
	}
	return executionCurrentRunInventory{RuntimeID: runtimeID, Run: &value}, nil
}

func (c *Client) UseRun(ctx context.Context, runID string) (port.OrcaRun, error) {
	runID, err := validateRunID(runID)
	if err != nil {
		return port.OrcaRun{}, err
	}
	if _, err := currentCoordinatorHandle(); err != nil {
		return port.OrcaRun{}, err
	}
	used, err := c.runMutation(ctx, []string{"orca", "orchestration", "run-use", "--id", runID, "--json"})
	if err == nil && used.ID != runID {
		return port.OrcaRun{}, &port.OrcaError{Code: "run_binding_mismatch", Detail: "Orca bound a different Run", Invoked: true}
	}
	return used, err
}

func (c *Client) runMutation(ctx context.Context, argv []string) (port.OrcaRun, error) {
	var payload struct {
		Run runPayload `json:"run"`
	}
	runtimeID, err := c.runJSON(ctx, "", createTimeout, argv, &payload)
	if err != nil {
		return port.OrcaRun{}, err
	}
	return payload.Run.portValue(runtimeID)
}

func currentCoordinatorHandle() (string, error) {
	raw := os.Getenv("ORCA_TERMINAL_HANDLE")
	handle := strings.TrimSpace(raw)
	if raw != handle || !concreteTerminalHandlePattern.MatchString(handle) || len(handle) > 256 {
		return "", &port.OrcaError{Code: "coordinator_identity_unavailable", Detail: "ORCA_TERMINAL_HANDLE must identify the current concrete coordinator terminal"}
	}
	return handle, nil
}

func validateRunID(runID string) (string, error) {
	raw := runID
	runID = strings.TrimSpace(raw)
	if raw != runID || runID == "" || len(runID) > 1024 || strings.ContainsRune(runID, 0) {
		return "", &port.OrcaError{Code: "run_identity_invalid"}
	}
	return runID, nil
}

func (c *Client) ListTasks(ctx context.Context) ([]port.OrcaTask, error) {
	return c.listTasksAcrossRuns(ctx, "--ready")
}

func (c *Client) ListDispatchedTasks(ctx context.Context) ([]port.OrcaTask, error) {
	return c.listTasksAcrossRuns(ctx, "--status", "dispatched")
}

func (c *Client) ListAllTasks(ctx context.Context) ([]port.OrcaTask, error) {
	return c.listTasksAcrossRuns(ctx, "--brief")
}

func (c *Client) listAllTasksInventory(ctx context.Context) (executionTaskInventory, error) {
	return c.listTasksAcrossRunsInventory(ctx, "--brief")
}

func (c *Client) ListFailedTasks(ctx context.Context) ([]port.OrcaTask, error) {
	return c.listTasksAcrossRuns(ctx, "--status", "failed")
}

func (c *Client) listTasksAcrossRuns(ctx context.Context, flags ...string) ([]port.OrcaTask, error) {
	inventory, err := c.listTasksAcrossRunsInventory(ctx, flags...)
	return inventory.Rows, err
}

func (c *Client) listTasksAcrossRunsInventory(ctx context.Context, flags ...string) (executionTaskInventory, error) {
	runs, err := c.ListRuns(ctx)
	if err != nil {
		return executionTaskInventory{}, err
	}
	result := executionTaskInventory{}
	seen := make(map[string]struct{})
	for _, run := range runs {
		inventory, err := c.listRunTasksInventory(ctx, run.ID, flags...)
		if err != nil {
			return executionTaskInventory{}, err
		}
		if result.RuntimeID == "" {
			result.RuntimeID = run.RuntimeID
		}
		if err := validateExecutionInventoryRuntime(inventory.RuntimeID, run.RuntimeID); err != nil {
			return executionTaskInventory{}, err
		}
		for _, task := range inventory.Rows {
			key := task.RunID + "\x00" + task.ID
			if _, duplicate := seen[key]; duplicate {
				return executionTaskInventory{}, &port.OrcaError{Code: "task_inventory_ambiguous", Detail: "Orca returned a duplicate task identity in one Run", Invoked: true}
			}
			seen[key] = struct{}{}
			result.Rows = append(result.Rows, task)
		}
	}
	return result, nil
}

func (c *Client) listRunTasksInventory(ctx context.Context, runID string, flags ...string) (executionTaskInventory, error) {
	runID, err := validateRunID(runID)
	if err != nil {
		return executionTaskInventory{}, err
	}
	argv := append([]string{"orca", "orchestration", "task-list"}, flags...)
	argv = append(argv, "--run", runID, "--json")
	var payload struct {
		Tasks []taskPayload `json:"tasks"`
		Count *int          `json:"count"`
		RunID *string       `json:"runId"`
	}
	runtimeID, err := c.runJSON(ctx, "", readTimeout, argv, &payload)
	if err != nil {
		return executionTaskInventory{}, err
	}
	if err := requireReturnedRunID("task", runID, payload.RunID); err != nil {
		return executionTaskInventory{}, err
	}
	if err := requireReturnedCount("task", len(payload.Tasks), payload.Count); err != nil {
		return executionTaskInventory{}, err
	}
	result := make([]port.OrcaTask, 0, len(payload.Tasks))
	for _, task := range payload.Tasks {
		if strings.TrimSpace(task.ID) == "" || strings.TrimSpace(task.Status) == "" {
			return executionTaskInventory{}, fmt.Errorf("Orca task row identity is incomplete")
		}
		value := task.portValue()
		value.RuntimeID = runtimeID
		if value.RunID != "" && value.RunID != runID {
			return executionTaskInventory{}, &port.OrcaError{Code: "task_run_mismatch", Detail: "Orca returned a task from a different Run", Invoked: true}
		}
		value.RunID = runID
		result = append(result, value)
	}
	return executionTaskInventory{RuntimeID: runtimeID, Rows: result}, nil
}

func (c *Client) ListGates(ctx context.Context) ([]port.OrcaGate, error) {
	var payload struct {
		Gates []struct {
			ID     string `json:"id"`
			TaskID string `json:"task_id"`
			Status string `json:"status"`
		} `json:"gates"`
		Count *int `json:"count"`
	}
	runtimeID, err := c.runJSON(ctx, "", readTimeout, []string{"orca", "orchestration", "gate-list", "--json"}, &payload)
	if err != nil {
		return nil, err
	}
	if err := requireReturnedCount("gate", len(payload.Gates), payload.Count); err != nil {
		return nil, err
	}
	result := make([]port.OrcaGate, 0, len(payload.Gates))
	for _, gate := range payload.Gates {
		if strings.TrimSpace(gate.ID) == "" || strings.TrimSpace(gate.TaskID) == "" || strings.TrimSpace(gate.Status) == "" {
			return nil, fmt.Errorf("Orca gate row identity is incomplete")
		}
		result = append(result, port.OrcaGate{RuntimeID: runtimeID, ID: gate.ID, TaskID: gate.TaskID, Status: gate.Status})
	}
	return result, nil
}

func (c *Client) listRunGatesInventory(ctx context.Context, runID string) (executionGateInventory, error) {
	runID, err := validateRunID(runID)
	if err != nil {
		return executionGateInventory{}, err
	}
	var payload struct {
		Gates []struct {
			ID     string `json:"id"`
			TaskID string `json:"task_id"`
			Status string `json:"status"`
		} `json:"gates"`
		Count *int    `json:"count"`
		RunID *string `json:"runId"`
	}
	runtimeID, err := c.runJSON(ctx, "", readTimeout, []string{"orca", "orchestration", "gate-list", "--run", runID, "--json"}, &payload)
	if err != nil {
		return executionGateInventory{}, err
	}
	if err := requireReturnedRunID("gate", runID, payload.RunID); err != nil {
		return executionGateInventory{}, err
	}
	if err := requireReturnedCount("gate", len(payload.Gates), payload.Count); err != nil {
		return executionGateInventory{}, err
	}
	result := make([]port.OrcaGate, 0, len(payload.Gates))
	for _, gate := range payload.Gates {
		if strings.TrimSpace(gate.ID) == "" || strings.TrimSpace(gate.TaskID) == "" || strings.TrimSpace(gate.Status) == "" {
			return executionGateInventory{}, fmt.Errorf("Orca gate row identity is incomplete")
		}
		result = append(result, port.OrcaGate{RuntimeID: runtimeID, ID: gate.ID, TaskID: gate.TaskID, Status: gate.Status})
	}
	return executionGateInventory{RuntimeID: runtimeID, Rows: result}, nil
}

func (c *Client) InboxPresence(ctx context.Context) (port.OrcaInboxPresence, error) {
	var payload struct {
		Messages []struct{} `json:"messages"`
		Count    *int       `json:"count"`
	}
	runtimeID, err := c.runJSON(ctx, "", readTimeout, []string{"orca", "orchestration", "inbox", "--limit", "1", "--json"}, &payload)
	if err != nil {
		return port.OrcaInboxPresence{}, err
	}
	if payload.Count == nil {
		return port.OrcaInboxPresence{}, fmt.Errorf("Orca inbox completeness metadata is missing")
	}
	count := *payload.Count
	rows := len(payload.Messages)
	return port.OrcaInboxPresence{RuntimeID: runtimeID, Count: count, RowCount: rows, CompleteAbsence: count == 0 && rows == 0}, nil
}

func (c *Client) CreateTask(ctx context.Context, req port.OrcaCreateTaskRequest) (port.OrcaTask, error) {
	runID, err := validateRunID(req.RunID)
	if err != nil {
		return port.OrcaTask{}, err
	}
	if _, err := currentCoordinatorHandle(); err != nil {
		return port.OrcaTask{}, err
	}
	argv := []string{"orca", "orchestration", "task-create", "--spec", req.Spec, "--task-title", req.Title, "--display-name", req.DisplayName, "--run", runID, "--json"}
	var payload struct {
		Task taskPayload `json:"task"`
	}
	runtimeID, err := c.runJSON(ctx, "", createTimeout, argv, &payload)
	created := payload.Task.portValue()
	created.RuntimeID = runtimeID
	if err == nil && created.RunID != "" && created.RunID != runID {
		return port.OrcaTask{}, &port.OrcaError{Code: "task_run_mismatch", Invoked: true}
	}
	created.RunID = runID
	return created, err
}

// taskStatusCompleted는 정상 종료된 task의 terminal 상태다. 어떤 값이 종결로
// 취급되는지는 Orca의 지식이므로 호출자가 리터럴을 반복하지 않도록 여기서 정한다.
const taskStatusCompleted = "completed"

// SettleTask는 명시적으로 권한을 받은 Orca task mutation에서 task를 terminal 상태로
// 옮긴다. IssueOps execution complete는 durable lifecycle만 완료하므로 이 메서드를
// 호출하지 않는다.
//
// 이 메서드가 별도로 있는 이유는 어떤 status가 종결인지가 Orca 쪽 지식이기
// 때문이다. 호출자는 "종결시켜라"만 말하고 값은 알 필요가 없다.
func (c *Client) SettleTask(ctx context.Context, runID, id string) error {
	err := c.UpdateTask(ctx, runID, id, taskStatusCompleted, "")
	if err == nil || !isConsumerFenced(err) {
		return err
	}
	// Orca는 task mutation을 Run 단위로 격리하고 호출 terminal이 그 Run의
	// current consumer인지 인증한다. coordinator가 그 사이 다른 Run에
	// 바인딩됐으면 task mutation이 consumer_fenced로 실패한다(#325).
	//
	// 봉인된 Run은 record가 이미 알고 있으므로 다시 바인딩하면 authority가
	// 회복된다. 실측(relay 0.1.0+66c426c5173c):
	//   task-update --run A → consumer_fenced ("bound to B, not A")
	//   run-use --id A      → ok
	//   task-update --run A → ok
	//
	// 재시도는 정확히 한 번이다. 반복하면 fence가 풀리지 않는 상황에서
	// 무한히 매달린다. 바인딩 자체가 실패하면 원래 fence 진단을 그대로
	// 돌려준다 — 그것이 사용자가 볼 근본 원인이다.
	//
	// 바인딩은 UseRun으로 한다. UseRun은 Orca가 돌려준 Run이 요청한 Run과
	// 같은지까지 확인하므로, 엉뚱한 Run에 바인딩된 채로 재시도해 잘못된
	// authority로 mutation하는 경로가 없다. 바인딩을 되돌리지도 않는다 —
	// 이전 바인딩을 복원하면 그 사이 일어난 다른 mutation의 authority를
	// 되돌리는 셈이 된다.
	if _, bindErr := c.UseRun(ctx, runID); bindErr != nil {
		return err
	}
	return c.UpdateTask(ctx, runID, id, taskStatusCompleted, "")
}

// isConsumerFenced는 오류가 Orca의 Run consumer fence인지 보고한다.
func isConsumerFenced(err error) bool {
	typed, ok := errors.AsType[*port.OrcaError](err)
	return ok && typed.Code == "consumer_fenced"
}

// UpdateTask는 Orca task 상태를 명시적으로 변경하는 저수준 명령이다. IssueOps
// execution complete는 Orca dispatch의 terminal authority가 아니므로 이 경로를
// 사용하지 않는다.
//
// #121이 이 명령을 residue 해법으로 기각했던 근거("상태를 바꿔도 소유자 조회가
// 0건이라 분류기가 잔여물로 보고한다")는 #121 자신의 수정으로 사라졌다. 그
// 수정이 종결된 task를 면제하게 만들었으므로 이제 상태 변경이 곧 해법이다.
func (c *Client) UpdateTask(ctx context.Context, runID, id, status, result string) error {
	runID, err := validateRunID(runID)
	if err != nil {
		return err
	}
	if _, err := currentCoordinatorHandle(); err != nil {
		return err
	}
	argv := []string{"orca", "orchestration", "task-update", "--id", id, "--status", status}
	if result != "" {
		argv = append(argv, "--result", result)
	}
	argv = append(argv, "--run", runID, "--json")
	_, err = c.runJSON(ctx, "", readTimeout, argv, &struct{}{})
	return err
}

func (c *Client) Dispatch(ctx context.Context, req port.OrcaDispatchRequest) (port.OrcaDispatch, error) {
	runID, err := validateRunID(req.RunID)
	if err != nil {
		return port.OrcaDispatch{}, err
	}
	if _, err := currentCoordinatorHandle(); err != nil {
		return port.OrcaDispatch{}, err
	}
	argv := []string{"orca", "orchestration", "dispatch", "--task", req.TaskID, "--to", req.ToHandle, "--run", runID}
	if req.Inject {
		argv = append(argv, "--inject")
	}
	if req.ReturnPreamble {
		argv = append(argv, "--return-preamble")
	}
	argv = append(argv, "--json")
	return c.dispatchResult(ctx, argv)
}

func (c *Client) ShowDispatch(ctx context.Context, taskID string) (port.OrcaDispatch, error) {
	return c.dispatchResult(ctx, []string{"orca", "orchestration", "dispatch-show", "--task", taskID, "--json"})
}

func (c *Client) showDispatchInventory(ctx context.Context, taskID string) (executionDispatchInventory, error) {
	return c.dispatchInventoryResult(ctx, []string{"orca", "orchestration", "dispatch-show", "--task", taskID, "--json"})
}

func (c *Client) ShowDispatchFrom(ctx context.Context, taskID, fromHandle string) (port.OrcaDispatch, error) {
	return c.dispatchResult(ctx, []string{"orca", "orchestration", "dispatch-show", "--task", taskID, "--preamble", "--from", fromHandle, "--json"})
}

// SendWorkerDone은 core와 CLI에서 호출되지 않는다(#127에서 보존 결정). 판단
// 근거는 port의 OrcaWorkerDoneClient 주석에 있다.
//
// 삭제 대신 보존한 이유: 아래 검증(응답 정체 일치, payload 일치, bounded 핸들과
// 경로 요구)은 Orca dispatch 프로토콜의 완료 보고를 신뢰 가능하게 만드는 부분이며
// 재작성 비용이 작지 않다. #130이 task 종결 경로를 설계할 때 이 구현이 후보다.
func (c *Client) SendWorkerDone(ctx context.Context, req port.OrcaWorkerDoneRequest) (port.OrcaWorkerDoneResult, error) {
	if err := validateWorkerDoneRequest(req); err != nil {
		return port.OrcaWorkerDoneResult{}, &port.OrcaError{Code: "worker_done_invalid", Detail: err.Error()}
	}
	argv := []string{
		"orca", "orchestration", "send",
		"--run", req.RunID,
		"--to", req.ToHandle,
		"--from", req.FromHandle,
		"--type", "worker_done",
		"--subject", req.Subject,
		"--body", req.Body,
		"--task-id", req.TaskID,
		"--dispatch-id", req.DispatchID,
		"--outcome", req.Outcome,
	}
	if len(req.ChangedFiles) > 0 {
		argv = append(argv, "--files-modified", strings.Join(req.ChangedFiles, ","))
	}
	argv = append(argv, "--report-path", req.ReportPath, "--json")
	output, err := c.runner.Run(ctx, "", createTimeout, argv)
	if err != nil {
		return port.OrcaWorkerDoneResult{}, err
	}
	var payload struct {
		Message struct {
			ID         string `json:"id"`
			FromHandle string `json:"from_handle"`
			ToHandle   string `json:"to_handle"`
			Type       string `json:"type"`
			Subject    string `json:"subject"`
			Body       string `json:"body"`
			Payload    string `json:"payload"`
			Sequence   int64  `json:"sequence"`
		} `json:"message"`
	}
	if _, err := decodeResult(output, &payload); err != nil {
		return port.OrcaWorkerDoneResult{}, &port.OrcaError{Code: "worker_done_response_malformed", Detail: boundedDiagnostic(err.Error()), Invoked: output.Invoked}
	}
	message := payload.Message
	if message.ID == "" || len(message.ID) > 1024 || message.Sequence <= 0 || message.FromHandle != req.FromHandle || message.ToHandle != req.ToHandle || message.Type != "worker_done" || message.Subject != req.Subject || message.Body != req.Body {
		return port.OrcaWorkerDoneResult{}, &port.OrcaError{Code: "worker_done_response_mismatch", Detail: "Orca message identity or evidence does not match the requested projection", Invoked: true}
	}
	var evidence struct {
		TaskID        string   `json:"taskId"`
		DispatchID    string   `json:"dispatchId"`
		Outcome       string   `json:"outcome"`
		FilesModified []string `json:"filesModified"`
		ReportPath    string   `json:"reportPath"`
	}
	if len(message.Payload) > 64*1024 || json.Unmarshal([]byte(message.Payload), &evidence) != nil || evidence.TaskID != req.TaskID || evidence.DispatchID != req.DispatchID || evidence.Outcome != req.Outcome || !slices.Equal(evidence.FilesModified, req.ChangedFiles) || evidence.ReportPath != req.ReportPath {
		return port.OrcaWorkerDoneResult{}, &port.OrcaError{Code: "worker_done_response_mismatch", Detail: "Orca message payload does not match the requested projection", Invoked: true}
	}
	return port.OrcaWorkerDoneResult{MessageID: message.ID, Sequence: message.Sequence}, nil
}

func validateWorkerDoneRequest(req port.OrcaWorkerDoneRequest) error {
	if _, err := validateRunID(req.RunID); err != nil {
		return fmt.Errorf("worker_done requires a concrete current Run identity")
	}
	if !concreteTerminalHandlePattern.MatchString(req.FromHandle) || !concreteTerminalHandlePattern.MatchString(req.ToHandle) || req.FromHandle == req.ToHandle || len(req.FromHandle) > 256 || len(req.ToHandle) > 256 {
		return fmt.Errorf("worker_done requires distinct concrete bounded Orca terminal handles")
	}
	for name, value := range map[string]struct {
		value string
		limit int
	}{
		"subject": {req.Subject, 256}, "body": {req.Body, 4096}, "task id": {req.TaskID, 1024}, "dispatch id": {req.DispatchID, 1024},
	} {
		if strings.TrimSpace(value.value) == "" || value.value != strings.TrimSpace(value.value) || len(value.value) > value.limit || strings.ContainsRune(value.value, 0) {
			return fmt.Errorf("worker_done %s is missing, non-canonical, or unbounded", name)
		}
	}
	if req.Outcome != "succeeded" && req.Outcome != "failed" {
		return fmt.Errorf("worker_done outcome must be succeeded or failed")
	}
	if len(req.ChangedFiles) > 512 {
		return fmt.Errorf("worker_done changed files are unbounded")
	}
	for _, path := range req.ChangedFiles {
		if path == "" || strings.ContainsAny(path, ",\x00") {
			return fmt.Errorf("worker_done changed files cannot be represented exactly")
		}
	}
	if !filepath.IsAbs(req.ReportPath) || filepath.Clean(req.ReportPath) != req.ReportPath || len(req.ReportPath) > 4096 || strings.ContainsRune(req.ReportPath, 0) {
		return fmt.Errorf("worker_done report path must be an exact bounded absolute path")
	}
	return nil
}

func (c *Client) dispatchResult(ctx context.Context, argv []string) (port.OrcaDispatch, error) {
	inventory, err := c.dispatchInventoryResult(ctx, argv)
	if err != nil {
		return port.OrcaDispatch{}, err
	}
	if inventory.Dispatch == nil {
		return port.OrcaDispatch{}, &port.OrcaError{Code: "not_found"}
	}
	return *inventory.Dispatch, nil
}

func (c *Client) dispatchInventoryResult(ctx context.Context, argv []string) (executionDispatchInventory, error) {
	var payload struct {
		Dispatch *struct {
			ID             string `json:"id"`
			TaskID         string `json:"task_id"`
			AssigneeHandle string `json:"assignee_handle"`
			Status         string `json:"status"`
		} `json:"dispatch"`
		Injected bool   `json:"injected"`
		Preamble string `json:"preamble"`
	}
	runtimeID, err := c.runJSON(ctx, "", createTimeout, argv, &payload)
	if err != nil {
		return executionDispatchInventory{}, err
	}
	if payload.Dispatch == nil {
		return executionDispatchInventory{RuntimeID: runtimeID}, nil
	}
	dispatch := port.OrcaDispatch{RuntimeID: runtimeID, ID: payload.Dispatch.ID, TaskID: payload.Dispatch.TaskID, AssigneeHandle: payload.Dispatch.AssigneeHandle, Status: payload.Dispatch.Status, Injected: payload.Injected, Preamble: payload.Preamble}
	return executionDispatchInventory{RuntimeID: runtimeID, Dispatch: &dispatch}, nil
}
