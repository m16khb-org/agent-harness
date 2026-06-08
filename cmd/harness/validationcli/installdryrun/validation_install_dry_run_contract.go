package installdryrun

import "path/filepath"

var requiredInstallDryRunHosts = []string{"codex", "claude", "reasonix"}

func installDryRunValidationErrors(result installDryRunSmokeResult, tempHome, tempRoot string, pathExists func(string) bool) []string {
	errs := []string{}
	if !result.OK || !result.DryRun || !result.ProjectLocal {
		errs = append(errs, "install dry-run result flags mismatch")
	}
	seenHosts := map[string]bool{}
	for _, host := range result.Hosts {
		seenHosts[host.Host] = true
		if !host.OK || !host.DryRun {
			errs = append(errs, "install dry-run host mismatch:"+host.Host)
		}
	}
	for _, host := range requiredInstallDryRunHosts {
		if !seenHosts[host] {
			errs = append(errs, "install dry-run missing host:"+host)
		}
	}
	if !containsString(result.SkillNames, skillName) {
		errs = append(errs, "install dry-run did not discover smoke skill")
	}
	plannedWrite := false
	for _, file := range result.Files {
		if file.Written {
			errs = append(errs, "install dry-run reported written file:"+file.Path)
		}
		if file.WouldWrite {
			plannedWrite = true
		}
	}
	plannedLink := false
	for _, link := range result.Links {
		if link.Created {
			errs = append(errs, "install dry-run reported created link:"+link.Path)
		}
		if link.WouldCreate {
			plannedLink = true
		}
	}
	if !plannedWrite || !plannedLink {
		errs = append(errs, "install dry-run did not expose planned writes and links")
	}
	for _, path := range []string{
		filepath.Join(tempHome, ".codex"),
		filepath.Join(tempHome, ".claude"),
		filepath.Join(tempRoot, "configs"),
		filepath.Join(tempRoot, ".mcp.json"),
		filepath.Join(tempRoot, ".claude"),
	} {
		if pathExists(path) {
			errs = append(errs, "install dry-run wrote unexpected path:"+path)
		}
	}
	return errs
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
