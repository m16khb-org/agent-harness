package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var mermaidUnquotedBracketTextRe = regexp.MustCompile(`\[[^"\]]`)

func validateMermaidDocs(root string) []string {
	return validateMermaidDocsWithDeps(root, docsValidationDeps{})
}

func validateMermaidDocsWithDeps(root string, deps docsValidationDeps) []string {
	deps = deps.withDefaults()
	errs := []string{}
	for _, path := range deps.listDocs(root) {
		b, err := deps.readFile(path)
		if err != nil {
			errs = append(errs, "read mermaid doc "+path+": "+err.Error())
			continue
		}
		rel, err := deps.rel(root, path)
		if err != nil {
			rel = path
		}
		for _, issue := range lintMermaidBlocks(filepath.ToSlash(rel), string(b)) {
			errs = append(errs, issue)
		}
	}
	return errs
}

func lintMermaidBlocks(relPath, text string) []string {
	errs := []string{}
	lines := strings.Split(text, "\n")
	inMermaid := false
	ignoreBlock := false
	currentHeading := ""
	ignoreNextMermaid := false
	for i, line := range lines {
		lineNo := i + 1
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "harness:mermaid-lint ignore") {
			ignoreNextMermaid = true
		}
		if strings.HasPrefix(trimmed, "#") {
			currentHeading = trimmed
		}
		if strings.HasPrefix(trimmed, "```") {
			if !inMermaid {
				if strings.HasPrefix(trimmed, "```mermaid") {
					inMermaid = true
					ignoreBlock = ignoreNextMermaid || strings.Contains(currentHeading, "잘못된 예시")
					ignoreNextMermaid = false
				}
				continue
			}
			inMermaid = false
			ignoreBlock = false
			continue
		}
		if !inMermaid || ignoreBlock {
			continue
		}
		if strings.Contains(line, "<br>") {
			errs = append(errs, fmt.Sprintf("%s:%d mermaid uses <br>; use <br/>", relPath, lineNo))
		}
		if mermaidUnquotedBracketTextRe.MatchString(line) {
			errs = append(errs, fmt.Sprintf("%s:%d mermaid node text must start with a quote", relPath, lineNo))
		}
		if strings.HasPrefix(trimmed, "subgraph ") {
			title := strings.TrimSpace(strings.TrimPrefix(trimmed, "subgraph "))
			if title != "" && !strings.HasPrefix(title, `"`) {
				errs = append(errs, fmt.Sprintf("%s:%d mermaid subgraph title must be quoted", relPath, lineNo))
			}
		}
	}
	return errs
}
