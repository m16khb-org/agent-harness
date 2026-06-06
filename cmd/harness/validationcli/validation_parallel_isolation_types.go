package validationcli

type parallelIsolationProbe struct {
	Worker       int      `json:"worker"`
	TempRoot     string   `json:"temp_root"`
	StateDir     string   `json:"state_dir"`
	DaemonDir    string   `json:"daemon_dir"`
	ArtifactPath string   `json:"artifact_path"`
	Key          string   `json:"key"`
	Commands     []string `json:"commands"`
	Error        string   `json:"error,omitempty"`
}

type parallelIsolationValidationDeps struct {
	runProbe func(string, string, int64, int) parallelIsolationProbe
}

func (deps parallelIsolationValidationDeps) withDefaults() parallelIsolationValidationDeps {
	if deps.runProbe == nil {
		deps.runProbe = runParallelIsolationProbe
	}
	return deps
}
