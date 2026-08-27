package state

import (
	statecontract "agent-harness/internal/contract/state"
)

type MaintenanceDependencies struct {
	AllRoots      func() ([]string, error)
	StoreExists   func(root string) bool
	MaintainStore func(root string) (statecontract.StoreMaintainResult, error)
}

type MaintenanceService struct {
	dependencies MaintenanceDependencies
}

func NewMaintenanceService(dependencies MaintenanceDependencies) *MaintenanceService {
	return &MaintenanceService{dependencies: dependencies}
}

func (service *MaintenanceService) Maintain() (statecontract.StateMaintainResult, error) {
	result := statecontract.StateMaintainResult{Roots: []statecontract.StoreMaintainResult{}}
	roots, err := service.dependencies.AllRoots()
	if err != nil {
		return result, err
	}
	for _, root := range roots {
		if !service.dependencies.StoreExists(root) {
			result.Skipped = append(result.Skipped, root)
			continue
		}
		maintained, err := service.dependencies.MaintainStore(root)
		if err != nil {
			return result, err
		}
		result.Roots = append(result.Roots, maintained)
	}
	result.OK = true
	return result, nil
}
