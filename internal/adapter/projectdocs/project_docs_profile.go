package projectdocs

import (
	projectdoccontract "issueops/internal/contract/projectdoc"
	projectdoc "issueops/internal/domain/projectdoc"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func inferProjectProfile(root string, signals projectdoc.ProjectSignals) projectdoccontract.ProjectProfile {
	profile := projectdoccontract.ProjectProfile{
		VCS:             inferProjectVCS(root),
		Languages:       append([]string{}, signals.Languages...),
		PackageManagers: append([]string{}, signals.PackageManagers...),
		Evidence:        []string{},
	}
	addEvidence := func(v string) { profile.Evidence = appendUnique(profile.Evidence, v) }
	if profile.VCS.RemoteHost != "" || profile.VCS.Provider == "git" || profile.VCS.Provider == "local" {
		addEvidence("git remote/config")
	}
	for _, rel := range signals.Files {
		switch {
		case rel == "go.mod" || strings.HasSuffix(rel, "/go.mod"):
			addEvidence(rel)
		case rel == "package.json" || strings.HasSuffix(rel, "/package.json"):
			addEvidence(rel)
		case rel == "pyproject.toml" || strings.HasSuffix(rel, "/pyproject.toml"):
			addEvidence(rel)
		case rel == "Cargo.toml" || strings.HasSuffix(rel, "/Cargo.toml"):
			addEvidence(rel)
		case rel == "pnpm-workspace.yaml", rel == "turbo.json", rel == "nx.json", rel == "lerna.json":
			addEvidence(rel)
		}
	}
	profile.Frameworks = detectFrameworks(root, signals.Files, addEvidence)
	profile.Monorepo = detectMonorepo(root, signals.Files, addEvidence)
	profile.ProjectTypes = inferProjectTypes(root, signals, profile.Frameworks, profile.Monorepo, addEvidence)
	sort.Strings(profile.Frameworks)
	sort.Strings(profile.ProjectTypes)
	sort.Strings(profile.Evidence)
	return profile
}

func inferProjectVCS(root string) projectdoccontract.ProjectVCSProfile {
	origin := ReadGitOriginURL(root)
	if origin == "" {
		if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
			return projectdoccontract.ProjectVCSProfile{Provider: "git", Hosting: "local", RemoteName: "origin"}
		}
		return projectdoccontract.ProjectVCSProfile{Provider: "none", Hosting: "local"}
	}
	host := remoteHost(origin)
	provider := "git"
	hosting := "self-hosted"
	switch strings.ToLower(host) {
	case "github.com":
		provider, hosting = "github", "managed"
	case "gitlab.com":
		provider, hosting = "gitlab", "managed"
	case "bitbucket.org":
		provider, hosting = "bitbucket", "managed"
	default:
		lowerHost := strings.ToLower(host)
		switch {
		case strings.Contains(lowerHost, "gitlab"):
			provider = "gitlab"
		case strings.Contains(lowerHost, "github"):
			provider = "github"
		case strings.Contains(lowerHost, "bitbucket"):
			provider = "bitbucket"
		}
	}
	if host == "" {
		hosting = "unknown"
	}
	return projectdoccontract.ProjectVCSProfile{Provider: provider, Hosting: hosting, RemoteHost: host, RemoteName: "origin"}
}

func ReadGitOriginURL(root string) string {
	b, err := os.ReadFile(filepath.Join(root, ".git", "config"))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	inOrigin := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inOrigin = trimmed == `[remote "origin"]`
			continue
		}
		if inOrigin && strings.HasPrefix(trimmed, "url") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func remoteHost(remote string) string {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return ""
	}
	if strings.Contains(remote, "://") {
		u, err := url.Parse(remote)
		if err == nil {
			return strings.ToLower(u.Hostname())
		}
	}
	if at := strings.Index(remote, "@"); at >= 0 {
		rest := remote[at+1:]
		if colon := strings.Index(rest, ":"); colon >= 0 {
			return strings.ToLower(rest[:colon])
		}
		if slash := strings.Index(rest, "/"); slash >= 0 {
			return strings.ToLower(rest[:slash])
		}
	}
	return ""
}
