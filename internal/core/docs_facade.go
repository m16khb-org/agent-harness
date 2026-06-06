package core

import coredocs "agent-harness/internal/core/docs"

type DocsIndexResult = coredocs.DocsIndexResult
type DocIndexInfo = coredocs.DocIndexInfo

func ListDocs(root string) []string {
	return coredocs.ListDocs(root)
}

func DocsIndex(root, version string) DocsIndexResult {
	return coredocs.DocsIndex(root, version)
}

func readDocHeadings(path string) (string, []string) {
	return coredocs.ReadHeadings(path)
}
