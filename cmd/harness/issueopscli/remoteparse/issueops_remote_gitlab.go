package remoteparse

import (
	"net/url"
	"path"
	"strconv"
	"strings"
)

type GitLabMRPathParts struct {
	Project string
	IID     string
}

type GitLabIssuePathParts struct {
	Project string
	IID     string
	Kind    string
}

func SplitGitLabMRPath(escapedPath string) GitLabMRPathParts {
	trimmed := strings.Trim(path.Clean("/"+escapedPath), "/")
	parts := strings.Split(trimmed, "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] != "-" || parts[i+1] != "merge_requests" {
			continue
		}
		iid := parts[i+2]
		if _, err := strconv.Atoi(iid); err != nil {
			return GitLabMRPathParts{}
		}
		projectParts := parts[:i]
		for index, part := range projectParts {
			projectParts[index], _ = url.PathUnescape(part)
		}
		return GitLabMRPathParts{Project: strings.Join(projectParts, "/"), IID: iid}
	}
	return GitLabMRPathParts{}
}

func SplitGitLabIssuePath(escapedPath string) GitLabIssuePathParts {
	trimmed := strings.Trim(path.Clean("/"+escapedPath), "/")
	parts := strings.Split(trimmed, "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] != "-" || (parts[i+1] != "issues" && parts[i+1] != "work_items") {
			continue
		}
		kind := parts[i+1]
		iid := parts[i+2]
		if _, err := strconv.Atoi(iid); err != nil {
			return GitLabIssuePathParts{}
		}
		projectParts := parts[:i]
		for index, part := range projectParts {
			projectParts[index], _ = url.PathUnescape(part)
		}
		return GitLabIssuePathParts{Project: strings.Join(projectParts, "/"), IID: iid, Kind: kind}
	}
	return GitLabIssuePathParts{}
}
