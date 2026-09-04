package state

import (
	"testing"

	statecontract "issueops/internal/contract/state"
)

func TestMaintenanceServiceOwnsRootOrdering(t *testing.T) {
	maintained := []string{}
	service := NewMaintenanceService(MaintenanceDependencies{
		AllRoots:    func() ([]string, error) { return []string{"present", "absent"}, nil },
		StoreExists: func(root string) bool { return root == "present" },
		MaintainStore: func(root string) (statecontract.StoreMaintainResult, error) {
			maintained = append(maintained, root)
			return statecontract.StoreMaintainResult{Dir: root}, nil
		},
	})

	result, err := service.Maintain()
	if err != nil || !result.OK || len(result.Roots) != 1 || result.Roots[0].Dir != "present" || len(result.Skipped) != 1 {
		t.Fatalf("Maintain() = %+v, %v", result, err)
	}
	if len(maintained) != 1 || maintained[0] != "present" {
		t.Fatalf("maintained = %v", maintained)
	}
}
