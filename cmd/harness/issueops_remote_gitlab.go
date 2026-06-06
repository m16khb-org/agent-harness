package main

import (
	"net/url"
	"path"
	"strconv"
	"strings"
)

type gitLabMRPathParts struct {
	project string
	iid     string
}

type gitLabIssuePathParts struct {
	project string
	iid     string
}

func splitGitLabMRPath(escapedPath string) gitLabMRPathParts {
	trimmed := strings.Trim(path.Clean("/"+escapedPath), "/")
	parts := strings.Split(trimmed, "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] != "-" || parts[i+1] != "merge_requests" {
			continue
		}
		iid := parts[i+2]
		if _, err := strconv.Atoi(iid); err != nil {
			return gitLabMRPathParts{}
		}
		projectParts := parts[:i]
		for index, part := range projectParts {
			projectParts[index], _ = url.PathUnescape(part)
		}
		return gitLabMRPathParts{project: strings.Join(projectParts, "/"), iid: iid}
	}
	return gitLabMRPathParts{}
}

func splitGitLabIssuePath(escapedPath string) gitLabIssuePathParts {
	trimmed := strings.Trim(path.Clean("/"+escapedPath), "/")
	parts := strings.Split(trimmed, "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] != "-" || parts[i+1] != "issues" {
			continue
		}
		iid := parts[i+2]
		if _, err := strconv.Atoi(iid); err != nil {
			return gitLabIssuePathParts{}
		}
		projectParts := parts[:i]
		for index, part := range projectParts {
			projectParts[index], _ = url.PathUnescape(part)
		}
		return gitLabIssuePathParts{project: strings.Join(projectParts, "/"), iid: iid}
	}
	return gitLabIssuePathParts{}
}
