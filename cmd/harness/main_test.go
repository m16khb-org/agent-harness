package main

import "testing"

func TestMergeEnvOverridesReplacesExistingKeys(t *testing.T) {
	env := mergeEnvOverrides(
		[]string{"HOME=/real-home", "PATH=/bin", "HOME=/duplicate-home"},
		[]string{"HOME=/fixture-home", "LLM_WIKI_ROOT=/fixture-wiki"},
	)
	values := map[string]string{}
	counts := map[string]int{}
	for _, entry := range env {
		key, ok := envEntryKey(entry)
		if !ok {
			t.Fatalf("invalid env entry remained: %q", entry)
		}
		counts[key]++
		values[key] = entry
	}
	if counts["HOME"] != 1 || values["HOME"] != "HOME=/fixture-home" {
		t.Fatalf("HOME override was not unique and last-wins: counts=%v values=%v env=%v", counts, values, env)
	}
	if values["PATH"] != "PATH=/bin" {
		t.Fatalf("PATH was not preserved: %v", env)
	}
	if values["LLM_WIKI_ROOT"] != "LLM_WIKI_ROOT=/fixture-wiki" {
		t.Fatalf("LLM_WIKI_ROOT override missing: %v", env)
	}
}
