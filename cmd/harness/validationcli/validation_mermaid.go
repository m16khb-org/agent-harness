package validationcli

import "path/filepath"

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
