package state

import (
	"time"

	statecontract "agent-harness/internal/contract/state"
)

type MaintenanceDependencies struct {
	KnownRoots       func() []string
	AllRoots         func() ([]string, error)
	StoreExists      func(root string) bool
	MaintainStore    func(root string) (statecontract.StoreMaintainResult, error)
	SentinelModified func() (time.Time, bool)
	TouchSentinel    func(time.Time)
	Now              func() time.Time
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

func (service *MaintenanceService) MaybeMaintain(minInterval time.Duration) (statecontract.StateMaintainResult, bool, error) {
	now := service.now()
	if modified, ok := service.dependencies.SentinelModified(); ok && now.Sub(modified) < minInterval {
		return statecontract.StateMaintainResult{
			OK:      true,
			Roots:   []statecontract.StoreMaintainResult{},
			Skipped: service.dependencies.KnownRoots(),
		}, false, nil
	}
	result, err := service.Maintain()
	service.dependencies.TouchSentinel(service.now())
	return result, true, err
}

func (service *MaintenanceService) now() time.Time {
	if service.dependencies.Now != nil {
		return service.dependencies.Now()
	}
	return time.Now()
}
