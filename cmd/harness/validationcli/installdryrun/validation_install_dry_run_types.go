package installdryrun

type installDryRunSmokeResult struct {
	OK           bool                     `json:"ok"`
	DryRun       bool                     `json:"dry_run"`
	ProjectLocal bool                     `json:"project_local"`
	Hosts        []installDryRunSmokeHost `json:"hosts"`
	Files        []installDryRunSmokeFile `json:"files"`
	Links        []installDryRunSmokeLink `json:"links"`
	SkillNames   []string                 `json:"skill_names"`
	Messages     []string                 `json:"messages"`
}

type installDryRunSmokeHost struct {
	Host   string `json:"host"`
	OK     bool   `json:"ok"`
	DryRun bool   `json:"dry_run"`
}

type installDryRunSmokeFile struct {
	Path       string `json:"path"`
	Written    bool   `json:"written"`
	WouldWrite bool   `json:"would_write"`
}

type installDryRunSmokeLink struct {
	Path        string `json:"path"`
	Created     bool   `json:"created"`
	WouldCreate bool   `json:"would_create"`
}
