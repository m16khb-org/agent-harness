package installutil

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"syscall"

	"agent-harness/internal/port"
)

// VerifyHookActivation performs semantic readback of only agent-harness hook
// groups while permitting unrelated co-resident hooks to remain untouched.
func VerifyHookActivation(path string, expected map[string]any) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var actual map[string]any
	if err := json.Unmarshal(raw, &actual); err != nil {
		return "", err
	}
	expectedHooks, ok := expected["hooks"].(map[string]any)
	if !ok || len(expectedHooks) == 0 {
		return "", fmt.Errorf("expected hook catalog is empty")
	}
	actualHooks, ok := actual["hooks"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("installed hook catalog is missing")
	}
	for event, groupsValue := range actualHooks {
		groups, ok := groupsValue.([]any)
		if !ok {
			return "", fmt.Errorf("installed hook event %s is malformed", event)
		}
		harnessGroups := []any{}
		for _, group := range groups {
			if HookGroupContainsAgentHarness(group) {
				harnessGroups = append(harnessGroups, group)
			}
		}
		expectedGroupsValue, expectedEvent := expectedHooks[event]
		if !expectedEvent {
			if len(harnessGroups) != 0 {
				return "", fmt.Errorf("installed hook event %s contains an unexpected agent-harness group", event)
			}
			continue
		}
		expectedGroups, ok := expectedGroupsValue.([]any)
		if !ok || len(expectedGroups) != 1 || len(harnessGroups) != 1 || !canonicalJSONEqual(harnessGroups[0], expectedGroups[0]) {
			return "", fmt.Errorf("installed hook event %s does not contain exactly one canonical agent-harness group", event)
		}
	}
	for event := range expectedHooks {
		if _, ok := actualHooks[event]; !ok {
			return "", fmt.Errorf("installed hook event %s is missing", event)
		}
	}
	return SemanticSHA256(expected)
}

func SemanticSHA256(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func CaptureNativeActivationEvidence(host, surface, path, semanticSHA256 string) (port.NativeActivationEvidence, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return port.NativeActivationEvidence{}, err
	}
	path = filepath.Clean(path)
	info, err := os.Lstat(path)
	if err != nil {
		return port.NativeActivationEvidence{}, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Dev == 0 || stat.Ino == 0 || stat.Nlink != 1 || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return port.NativeActivationEvidence{}, fmt.Errorf("activation evidence must be a physical single-link regular file: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return port.NativeActivationEvidence{}, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return port.NativeActivationEvidence{}, err
	}
	opened, err := file.Stat()
	if err != nil {
		return port.NativeActivationEvidence{}, err
	}
	openedStat, ok := opened.Sys().(*syscall.Stat_t)
	if !ok || openedStat.Dev != stat.Dev || openedStat.Ino != stat.Ino || opened.Size() != info.Size() {
		return port.NativeActivationEvidence{}, fmt.Errorf("activation evidence changed during readback: %s", path)
	}
	return port.NativeActivationEvidence{
		Host: host, Surface: surface, Path: path, SemanticSHA256: semanticSHA256,
		SHA256: hex.EncodeToString(hash.Sum(nil)), Mode: uint32(opened.Mode()), Size: opened.Size(),
		Device: uint64(openedStat.Dev), Inode: uint64(openedStat.Ino),
	}, nil
}

func canonicalJSONEqual(left, right any) bool {
	leftRaw, leftErr := json.Marshal(left)
	rightRaw, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	var leftValue, rightValue any
	if json.Unmarshal(leftRaw, &leftValue) != nil || json.Unmarshal(rightRaw, &rightValue) != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}
