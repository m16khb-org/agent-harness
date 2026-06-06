package core

import "testing"

func TestContainsAnyMatchesVariadicNeedles(t *testing.T) {
	if !containsAny("codex should review this", "plan", "review") {
		t.Fatal("containsAny should match a later needle")
	}

	if containsAny("codex should review this", "ship", "deploy") {
		t.Fatal("containsAny should reject missing needles")
	}

	if containsAny("codex should review this") {
		t.Fatal("containsAny should reject an empty needle set")
	}
}

func TestContainsAnySliceMatchesNeedles(t *testing.T) {
	if !containsAnySlice("이슈 기반 리팩터", []string{"문서", "이슈"}) {
		t.Fatal("containsAnySlice should match a later needle")
	}

	if containsAnySlice("architecture refactor", []string{"endpoint", "swagger"}) {
		t.Fatal("containsAnySlice should reject missing needles")
	}

	if containsAnySlice("architecture refactor", nil) {
		t.Fatal("containsAnySlice should reject nil needles")
	}
}
