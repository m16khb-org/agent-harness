package core

import "agent-harness/internal/core/contextregion"

const (
	RegionImmutablePrefix = contextregion.RegionImmutablePrefix
	RegionAppendOnlyLog   = contextregion.RegionAppendOnlyLog
	RegionVolatileScratch = contextregion.RegionVolatileScratch
)

var VolatileContextFields = contextregion.VolatileContextFields

func StableProjection(value any) any {
	return contextregion.StableProjection(value)
}

func StableProjectionJSON(value any) (string, error) {
	return contextregion.StableProjectionJSON(value)
}

func ContextSerializationStable(build func() any) (bool, string, error) {
	return contextregion.ContextSerializationStable(build)
}
