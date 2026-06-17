package remoteparse

import "testing"

func TestSplitGitLabMRPath_whenValidEscapedPath(t *testing.T) {
	cases := []struct {
		name        string
		escapedPath string
		wantProject string
		wantIID     string
	}{
		{
			name:        "nested project",
			escapedPath: "/group/subgroup/project/-/merge_requests/42",
			wantProject: "group/subgroup/project",
			wantIID:     "42",
		},
		{
			name:        "escaped project segment",
			escapedPath: "/platform/team%20space/repo/-/merge_requests/7",
			wantProject: "platform/team space/repo",
			wantIID:     "7",
		},
		{
			name:        "unclean path",
			escapedPath: "group/project/../project/-/merge_requests/9/",
			wantProject: "group/project",
			wantIID:     "9",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parts := SplitGitLabMRPath(tc.escapedPath)
			if parts.Project != tc.wantProject || parts.IID != tc.wantIID {
				t.Fatalf("splitGitLabMRPath(%q) = %+v, want project=%q iid=%q", tc.escapedPath, parts, tc.wantProject, tc.wantIID)
			}
		})
	}
}

func TestSplitGitLabMRPath_whenInvalidPath(t *testing.T) {
	cases := []struct {
		escapedPath string
		wantIID     string
	}{
		{escapedPath: "/group/project/-/issues/42"},
		{escapedPath: "/group/project/-/merge_requests/not-number"},
		{escapedPath: "/-/merge_requests/42", wantIID: "42"},
		{escapedPath: "/group/project/merge_requests/42"},
	}
	for _, tc := range cases {
		t.Run(tc.escapedPath, func(t *testing.T) {
			if parts := SplitGitLabMRPath(tc.escapedPath); parts.Project != "" || parts.IID != tc.wantIID {
				t.Fatalf("splitGitLabMRPath(%q) = %+v, want project empty iid=%q", tc.escapedPath, parts, tc.wantIID)
			}
		})
	}
}

func TestSplitGitLabIssuePath_whenValidEscapedPath(t *testing.T) {
	cases := []struct {
		name        string
		escapedPath string
		wantProject string
		wantIID     string
	}{
		{
			name:        "nested project",
			escapedPath: "/group/subgroup/project/-/issues/42",
			wantProject: "group/subgroup/project",
			wantIID:     "42",
		},
		{
			name:        "escaped project segment",
			escapedPath: "/platform/team%20space/repo/-/issues/7",
			wantProject: "platform/team space/repo",
			wantIID:     "7",
		},
		{
			name:        "work item child",
			escapedPath: "/group/project/-/work_items/8",
			wantProject: "group/project",
			wantIID:     "8",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parts := SplitGitLabIssuePath(tc.escapedPath)
			if parts.Project != tc.wantProject || parts.IID != tc.wantIID {
				t.Fatalf("splitGitLabIssuePath(%q) = %+v, want project=%q iid=%q", tc.escapedPath, parts, tc.wantProject, tc.wantIID)
			}
		})
	}
}

func TestSplitGitLabIssuePath_whenInvalidPath(t *testing.T) {
	cases := []struct {
		escapedPath string
		wantIID     string
	}{
		{escapedPath: "/group/project/-/merge_requests/42"},
		{escapedPath: "/group/project/-/issues/not-number"},
		{escapedPath: "/-/issues/42", wantIID: "42"},
		{escapedPath: "/group/project/issues/42"},
	}
	for _, tc := range cases {
		t.Run(tc.escapedPath, func(t *testing.T) {
			if parts := SplitGitLabIssuePath(tc.escapedPath); parts.Project != "" || parts.IID != tc.wantIID {
				t.Fatalf("splitGitLabIssuePath(%q) = %+v, want project empty iid=%q", tc.escapedPath, parts, tc.wantIID)
			}
		})
	}
}
