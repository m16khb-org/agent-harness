package state

import (
	"testing"
	"time"

	statecontract "agent-harness/internal/contract/state"
)

func TestMaintenanceServiceOwnsRootOrderingAndSentinelGate(t *testing.T) {
	now := time.Date(2026, 8, 4, 2, 0, 0, 0, time.UTC)
	maintained := []string{}
	touched := 0
	service := NewMaintenanceService(MaintenanceDependencies{
		KnownRoots:  func() []string { return []string{"known"} },
		AllRoots:    func() ([]string, error) { return []string{"present", "absent"}, nil },
		StoreExists: func(root string) bool { return root == "present" },
		MaintainStore: func(root string) (statecontract.StoreMaintainResult, error) {
			maintained = append(maintained, root)
			return statecontract.StoreMaintainResult{Dir: root}, nil
		},
		SentinelModified: func() (time.Time, bool) { return now.Add(-time.Minute), true },
		TouchSentinel:    func(time.Time) { touched++ },
		Now:              func() time.Time { return now },
	})

	skipped, ran, err := service.MaybeMaintain(time.Hour)
	if err != nil || ran || !skipped.OK || len(skipped.Skipped) != 1 || touched != 0 {
		t.Fatalf("MaybeMaintain() = %+v, %v, %v; touched=%d", skipped, ran, err, touched)
	}
	result, err := service.Maintain()
	if err != nil || !result.OK || len(result.Roots) != 1 || result.Roots[0].Dir != "present" || len(result.Skipped) != 1 {
		t.Fatalf("Maintain() = %+v, %v", result, err)
	}
	if len(maintained) != 1 || maintained[0] != "present" {
		t.Fatalf("maintained = %v", maintained)
	}
}
