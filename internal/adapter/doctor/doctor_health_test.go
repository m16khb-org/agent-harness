package doctor

import "testing"

func TestDoctorHealthyFollowsChecksAndIssues(t *testing.T) {
	tests := []struct {
		name   string
		checks []HarnessDoctorCheck
		issues []HarnessDoctorIssue
		want   bool
	}{
		{name: "healthy checks", checks: []HarnessDoctorCheck{{Name: "ready", Healthy: true}}, want: true},
		{name: "unhealthy check without issue", checks: []HarnessDoctorCheck{{Name: "ready", Healthy: false}}, want: false},
		{name: "warning issue", issues: []HarnessDoctorIssue{{Code: "warning", Severity: "warning"}}, want: false},
		{name: "informational issue", issues: []HarnessDoctorIssue{{Code: "info", Severity: "info"}}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := doctorHealthy(tt.checks, tt.issues); got != tt.want {
				t.Fatalf("doctorHealthy()=%t want %t", got, tt.want)
			}
		})
	}
}

func TestCheckDaemonAdmissionStates(t *testing.T) {
	tests := []struct {
		name         string
		admission    HarnessDoctorDaemonAdmission
		checkHealthy bool
		summary      string
		issueCode    string
	}{
		{
			name:         "not observed",
			admission:    HarnessDoctorDaemonAdmission{Observed: false, MaxConnections: 64},
			checkHealthy: true,
			summary:      "not evaluated",
		},
		{
			name:         "zero capacity not evaluated",
			admission:    HarnessDoctorDaemonAdmission{Observed: true},
			checkHealthy: true,
			summary:      "not evaluated",
		},
		{
			name:         "available",
			admission:    HarnessDoctorDaemonAdmission{Observed: true, ActiveConnections: 12, MaxConnections: 64, Accepting: true},
			checkHealthy: true,
		},
		{
			name:         "saturated",
			admission:    HarnessDoctorDaemonAdmission{Observed: true, ActiveConnections: 64, MaxConnections: 64},
			checkHealthy: false,
			issueCode:    "daemon_connection_limit_reached",
		},
		{
			name:         "under capacity refusal",
			admission:    HarnessDoctorDaemonAdmission{Observed: true, ActiveConnections: 12, MaxConnections: 64},
			checkHealthy: false,
			issueCode:    "daemon_admission_inconsistent",
		},
		{
			name:         "accepting at capacity",
			admission:    HarnessDoctorDaemonAdmission{Observed: true, ActiveConnections: 64, MaxConnections: 64, Accepting: true},
			checkHealthy: false,
			issueCode:    "daemon_admission_inconsistent",
		},
		{
			name:         "negative active connections",
			admission:    HarnessDoctorDaemonAdmission{Observed: true, ActiveConnections: -1, MaxConnections: 64, Accepting: true},
			checkHealthy: false,
			issueCode:    "daemon_admission_inconsistent",
		},
		{
			name:         "draining",
			admission:    HarnessDoctorDaemonAdmission{Observed: true, ActiveConnections: 64, MaxConnections: 64, Draining: true},
			checkHealthy: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HarnessDoctorResult{}
			checkDaemonAdmission(&result, tt.admission)
			if len(result.Checks) != 1 {
				t.Fatalf("expected one daemon admission check, got %+v", result.Checks)
			}
			if result.Checks[0].Healthy != tt.checkHealthy {
				t.Fatalf("daemon admission check=%+v want healthy=%t", result.Checks[0], tt.checkHealthy)
			}
			if tt.summary != "" && result.Checks[0].Summary != tt.summary {
				t.Fatalf("daemon admission summary=%q want %q", result.Checks[0].Summary, tt.summary)
			}
			if tt.issueCode == "" {
				if len(result.Issues) != 0 {
					t.Fatalf("daemon admission issues=%+v want none", result.Issues)
				}
			} else if len(result.Issues) != 1 || result.Issues[0].Code != tt.issueCode {
				t.Fatalf("daemon admission issues=%+v want code=%q", result.Issues, tt.issueCode)
			}
		})
	}
}
