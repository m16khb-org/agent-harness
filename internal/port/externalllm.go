// Package port defines the ExternalLLM interface for external LLM calls.
// All external LLM usage in the harness must go through this interface
// so tests can inject mock providers and call sites are decoupled from
// any specific CLI or API backend.
package port

// ExternalLLM is the interface for calling an external LLM.
// Implementations may use a local CLI (e.g. agy), a cloud API, or a mock.
type ExternalLLM interface {
	// Query sends a prompt to the LLM and returns the raw text response.
	Query(prompt string) (string, error)

	// QueryStructured sends a prompt with an expected JSON schema to the LLM
	// and returns the parsed response as a map. The schema should be a
	// JSON Schema object describing the expected output shape.
	QueryStructured(prompt string, schema map[string]any) (map[string]any, error)
}
