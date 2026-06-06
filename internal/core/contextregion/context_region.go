package contextregion

import "encoding/json"

// Context region vocabulary derived from DeepSeek-Reasonix's cache-first
// partitioning (Immutable Prefix / Append-Only Log / Volatile Scratch).
//
// agent-harness hosts neither a model session nor a prefix cache, so it does
// not adopt the cache engine. It borrows only the determinism contract: the
// serialized context an agent reuses as a stable prefix must be byte-identical
// across repeated builds, while volatile fields (timestamps, run ids) live in
// a separate region that is allowed to change. Centralizing this vocabulary
// lets docs/contract/state builders and their golden tests share one source of
// truth instead of each re-deciding which fields are volatile.

const (
	// RegionImmutablePrefix holds system-prompt-like context that must stay
	// byte-identical across builds so a host can reuse it as a cache prefix.
	RegionImmutablePrefix = "immutable_prefix"
	// RegionAppendOnlyLog holds turn-by-turn records that may grow but must
	// preserve their existing prefix order.
	RegionAppendOnlyLog = "append_only_log"
	// RegionVolatileScratch holds values expected to differ between builds.
	RegionVolatileScratch = "volatile_scratch"
)

// VolatileContextFields are JSON field names whose values are expected to
// change between otherwise-identical builds. They belong to the volatile
// region and must be excluded before asserting prefix byte-determinism. The
// set mirrors the dynamic time keys normalized by the response-contract golden
// test so the two surfaces cannot drift apart.
var VolatileContextFields = map[string]bool{
	"generated_at": true,
	"updated_at":   true,
	"cutoff":       true,
	"started_at":   true,
	"finished_at":  true,
}

// StableProjection returns a deep copy of value with every volatile context
// field removed, recursively. The result is the portion of a serialized
// context that must stay byte-identical across repeated builds. The input is
// not mutated.
func StableProjection(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			if VolatileContextFields[key] {
				continue
			}
			out[key] = StableProjection(child)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = StableProjection(child)
		}
		return out
	default:
		return v
	}
}

// StableProjectionJSON marshals value, strips volatile context fields, and
// returns the canonical JSON of the stable (immutable-prefix) portion. Map
// keys are sorted by encoding/json, so the output only varies when the
// stable content itself changes.
func StableProjectionJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	var generic any
	if err := json.Unmarshal(encoded, &generic); err != nil {
		return "", err
	}
	out, err := json.Marshal(StableProjection(generic))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ContextSerializationStable builds a reusable context twice and reports
// whether its immutable-prefix portion serialized byte-identically. It is the
// data-level enforcement of the Reasonix determinism contract: regardless of
// how a builder assembles context, repeated builds of unchanged inputs must
// yield an identical stable prefix. The returned JSON is that stable prefix.
func ContextSerializationStable(build func() any) (bool, string, error) {
	first, err := StableProjectionJSON(build())
	if err != nil {
		return false, "", err
	}
	second, err := StableProjectionJSON(build())
	if err != nil {
		return false, "", err
	}
	return first == second, first, nil
}
