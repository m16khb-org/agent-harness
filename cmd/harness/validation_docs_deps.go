package main

import (
	"os"
	"path/filepath"

	"agent-harness/internal/core"
)

type docsValidationDeps struct {
	readFile   func(string) ([]byte, error)
	listDocs   func(string) []string
	listSkills func(string) ([]string, error)
	exists     func(string) bool
	glob       func(string) ([]string, error)
	rel        func(string, string) (string, error)
}

func (deps docsValidationDeps) withDefaults() docsValidationDeps {
	if deps.readFile == nil {
		deps.readFile = os.ReadFile
	}
	if deps.listDocs == nil {
		deps.listDocs = core.ListDocs
	}
	if deps.listSkills == nil {
		deps.listSkills = core.ListSkillNames
	}
	if deps.exists == nil {
		deps.exists = exists
	}
	if deps.glob == nil {
		deps.glob = filepath.Glob
	}
	if deps.rel == nil {
		deps.rel = filepath.Rel
	}
	return deps
}
