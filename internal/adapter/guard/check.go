package guard

import (
	"path/filepath"
	"sort"
	"strings"
)

func GuardCheck(req GuardCheckRequest) GuardCheckResult {
	root := absOrOriginal(req.RepoRoot)
	if root == "" {
		root = absOrOriginal(".")
	}
	result := GuardCheckResult{
		OK:           true,
		RepoRoot:     root,
		Mode:         guardMode(req),
		CheckedFiles: []string{},
		Findings:     []GuardFinding{},
		Warnings:     []string{},
	}
	files := guardTargetFiles(root, req)
	result.CheckedFiles = files
	existingSymbols := guardExistingSymbols(root, files)
	hasProdChange := false
	hasTestChange := false
	hasContractSurfaceChange := false
	hasGoldenChange := false
	for _, rel := range files {
		if secretPathRe.MatchString(filepath.ToSlash(rel)) {
			result.Findings = append(result.Findings, GuardFinding{
				Severity: "block",
				Rule:     "secret-like-path",
				File:     rel,
				Message:  "Secret-like paths must not be committed or analyzed as ordinary source.",
			})
			continue
		}
		if isTestPath(rel) {
			hasTestChange = true
		} else if isSourcePath(rel) {
			hasProdChange = true
		}
		if isContractSurfacePath(rel) {
			hasContractSurfaceChange = true
		}
		if strings.Contains(filepath.ToSlash(rel), "testdata/") || strings.Contains(strings.ToLower(rel), "golden") {
			hasGoldenChange = true
		}
		content, ok := guardReadFile(root, rel, req.Staged)
		if !ok {
			continue
		}
		result.Findings = append(result.Findings, guardFileFindings(rel, content, existingSymbols)...)
	}
	if hasProdChange && !hasTestChange {
		result.Findings = append(result.Findings, GuardFinding{
			Severity: "warn",
			Rule:     "prod-change-without-test",
			Message:  "Production source changed without a changed test file; verify this is documentation/config-only or add focused coverage.",
		})
	}
	if hasContractSurfaceChange && !hasGoldenChange {
		result.Findings = append(result.Findings, GuardFinding{
			Severity: "warn",
			Rule:     "contract-surface-without-golden",
			Message:  "CLI/MCP/adapter contract surface changed without a golden/testdata update.",
		})
	}
	result.Findings = dedupeGuardFindings(result.Findings)
	sort.Slice(result.Findings, func(i, j int) bool {
		if guardSeverityRank(result.Findings[i].Severity) != guardSeverityRank(result.Findings[j].Severity) {
			return guardSeverityRank(result.Findings[i].Severity) < guardSeverityRank(result.Findings[j].Severity)
		}
		if result.Findings[i].File != result.Findings[j].File {
			return result.Findings[i].File < result.Findings[j].File
		}
		if result.Findings[i].Line != result.Findings[j].Line {
			return result.Findings[i].Line < result.Findings[j].Line
		}
		return result.Findings[i].Rule < result.Findings[j].Rule
	})
	for _, finding := range result.Findings {
		switch finding.Severity {
		case "block":
			result.Summary.Block++
		case "warn":
			result.Summary.Warn++
		case "review":
			result.Summary.Review++
		default:
			result.Summary.Info++
		}
	}
	result.OK = result.Summary.Block == 0
	return result
}
