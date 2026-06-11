package lifecycle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"agent-harness/internal/core/lifecycle/fingerprint"
	"agent-harness/internal/core/repopath"
)

func ResolveProjectLifecycleState(repoRoot string) (ProjectLifecycleStatePlan, error) {
	root, err := repopath.NormalizeRoot(repoRoot)
	if err != nil {
		return ProjectLifecycleStatePlan{OK: false, StateRoot: StateDir(), SchemaVersion: ProjectLifecycleSchemaVersion}, err
	}
	projectFingerprint := fingerprint.ForRoot(root)
	repoID := fingerprint.RepoID(projectFingerprint)
	stateRoot := StateDir()
	projectDir := filepath.Join(stateRoot, "projects", repoID)
	plan := ProjectLifecycleStatePlan{
		OK:              true,
		SchemaVersion:   ProjectLifecycleSchemaVersion,
		RepoRoot:        root,
		RepoID:          repoID,
		StateRoot:       stateRoot,
		ProjectStateDir: projectDir,
		ProjectJSONPath: filepath.Join(projectDir, projectLifecycleProfileFile),
		QueuePath:       filepath.Join(projectDir, docUpkeepQueueFile),
		CompactPath:     filepath.Join(projectDir, compactCapsuleFile),
		Fingerprint:     projectFingerprint,
		Warnings:        []string{},
	}
	profile, err := readProjectLifecycleProfile(plan.ProjectJSONPath)
	if os.IsNotExist(err) {
		return plan, nil
	}
	if err != nil {
		plan.Warnings = append(plan.Warnings, "project_json_read_error")
		return plan, nil
	}
	plan.Exists = true
	plan.Profile = &profile
	plan.NamespaceValid = fingerprint.Equal(profile.Fingerprint, projectFingerprint) && profile.RepoID == repoID && profile.SchemaVersion == ProjectLifecycleSchemaVersion
	if !plan.NamespaceValid {
		plan.Warnings = append(plan.Warnings, "namespace_mismatch")
	}
	return plan, nil
}

func InitProjectLifecycleState(repoRoot string, confirm bool, metadata ...ProjectProfile) (ProjectLifecycleStatePlan, error) {
	plan, err := ResolveProjectLifecycleState(repoRoot)
	if err != nil || !confirm {
		return plan, err
	}
	if plan.Exists && !plan.NamespaceValid {
		return plan, nil
	}
	if err := os.MkdirAll(plan.ProjectStateDir, 0o700); err != nil {
		plan.OK = false
		return plan, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	createdAt := now
	if plan.Profile != nil && plan.Profile.CreatedAt != "" {
		createdAt = plan.Profile.CreatedAt
	}
	var meta *ProjectProfile
	if len(metadata) > 0 {
		m := metadata[0]
		meta = &m
	} else if plan.Profile != nil {
		meta = plan.Profile.Metadata
	}
	profile := ProjectLifecycleProfile{
		SchemaVersion: ProjectLifecycleSchemaVersion,
		RepoID:        plan.RepoID,
		Fingerprint:   plan.Fingerprint,
		Metadata:      meta,
		CreatedAt:     createdAt,
		UpdatedAt:     now,
	}
	if !plan.Exists {
		// Use O_EXCL to avoid a race where two concurrent sessions
		// both pass the existence check and both write the profile.
		if err := createJSONAtomic(plan.ProjectJSONPath, profile, 0o600); err == nil {
			plan.Exists = true
			plan.NamespaceValid = true
			plan.Profile = &profile
			return plan, nil
		} else if !os.IsExist(err) {
			plan.OK = false
			return plan, err
		}
		// Another session won the race — read its profile.
		existing, err := readProjectLifecycleProfile(plan.ProjectJSONPath)
		if err != nil {
			plan.OK = false
			return plan, err
		}
		plan.Exists = true
		plan.Profile = &existing
		plan.NamespaceValid = fingerprint.Equal(existing.Fingerprint, plan.Fingerprint) &&
			existing.RepoID == plan.RepoID &&
			existing.SchemaVersion == ProjectLifecycleSchemaVersion
		if !plan.NamespaceValid {
			plan.Warnings = append(plan.Warnings, "namespace_mismatch")
		}
		return plan, nil
	}
	if err := writeJSONAtomic(plan.ProjectJSONPath, profile, 0o600); err != nil {
		plan.OK = false
		return plan, err
	}
	plan.Exists = true
	plan.NamespaceValid = true
	plan.Profile = &profile
	return plan, nil
}

func ValidateProjectLifecycleState(repoRoot string) (ProjectLifecycleStatePlan, error) {
	return ResolveProjectLifecycleState(repoRoot)
}

func readProjectLifecycleProfile(path string) (ProjectLifecycleProfile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return ProjectLifecycleProfile{}, err
	}
	var profile ProjectLifecycleProfile
	if err := json.Unmarshal(b, &profile); err != nil {
		return ProjectLifecycleProfile{}, err
	}
	return profile, nil
}

func writeJSONAtomic(path string, value any, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	writeErr := func() error {
		if _, err := tmp.Write(append(b, '\n')); err != nil {
			return err
		}
		if err := tmp.Chmod(perm); err != nil {
			return err
		}
		return tmp.Close()
	}()
	if writeErr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return writeErr
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func createJSONAtomic(path string, value any, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	writeErr := func() error {
		if _, err := tmp.Write(append(b, '\n')); err != nil {
			return err
		}
		if err := tmp.Chmod(perm); err != nil {
			return err
		}
		return tmp.Close()
	}()
	if writeErr != nil {
		_ = tmp.Close()
		return writeErr
	}
	// Hard-link the fully written temp file into place. Link fails with
	// EEXIST when the path already exists (os.IsExist unwraps the LinkError),
	// preserving O_EXCL's single-winner semantics while guaranteeing the file
	// is complete the moment it becomes visible — no read-while-write window
	// for the losing session.
	return os.Link(tmpName, path)
}
