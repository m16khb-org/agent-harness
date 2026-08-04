package issueops

import (
	"encoding/hex"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	GeneratedByExecutableFlag  = "--generated-by-executable"
	GeneratedBySHA256Flag      = "--generated-by-sha256"
	GeneratedForGenerationFlag = "--generated-for-generation"
)

type GeneratedCommandProvenance struct {
	ExecutablePath   string `json:"executable_path"`
	ExecutableSHA256 string `json:"executable_sha256"`
	LeaseGeneration  uint64 `json:"lease_generation"`
}

func (p GeneratedCommandProvenance) Validate() error {
	if p.ExecutablePath == "" || !filepath.IsAbs(p.ExecutablePath) || filepath.Clean(p.ExecutablePath) != p.ExecutablePath || len(p.ExecutablePath) > 4096 || containsControl(p.ExecutablePath) {
		return &GeneratedCommandProvenanceError{Code: "generated_command_provenance_invalid", Message: "generated command executable path is invalid", Expected: p}
	}
	if len(p.ExecutableSHA256) != 64 {
		return &GeneratedCommandProvenanceError{Code: "generated_command_provenance_invalid", Message: "generated command executable SHA-256 is invalid", Expected: p}
	}
	if _, err := hex.DecodeString(p.ExecutableSHA256); err != nil || p.ExecutableSHA256 != strings.ToLower(p.ExecutableSHA256) {
		return &GeneratedCommandProvenanceError{Code: "generated_command_provenance_invalid", Message: "generated command executable SHA-256 is invalid", Expected: p}
	}
	if p.LeaseGeneration == 0 {
		return &GeneratedCommandProvenanceError{Code: "generated_command_provenance_invalid", Message: "generated command lease generation is invalid", Expected: p}
	}
	return nil
}

func BindGeneratedCommand(command string, provenance GeneratedCommandProvenance) (string, error) {
	if strings.TrimSpace(command) == "" {
		return "", &GeneratedCommandProvenanceError{Code: "generated_command_provenance_invalid", Message: "generated command is empty", Expected: provenance}
	}
	if err := provenance.Validate(); err != nil {
		return "", err
	}
	tail, matched := strings.CutPrefix(command, "agent-harness")
	if !matched || (tail != "" && !strings.HasPrefix(tail, " ")) {
		return "", &GeneratedCommandProvenanceError{Code: "generated_command_provenance_invalid", Message: "generated command executable token is invalid", Expected: provenance}
	}
	command = quoteGeneratedCommandArg(provenance.ExecutablePath) + tail
	return command +
		" " + GeneratedByExecutableFlag + " " + quoteGeneratedCommandArg(provenance.ExecutablePath) +
		" " + GeneratedBySHA256Flag + " " + provenance.ExecutableSHA256 +
		" " + GeneratedForGenerationFlag + " " + strconv.FormatUint(provenance.LeaseGeneration, 10), nil
}

func ConsumeGeneratedCommandProvenance(args []string) ([]string, GeneratedCommandProvenance, bool, error) {
	clean := make([]string, 0, len(args))
	values := map[string]string{}
	present := false
	for index := 0; index < len(args); index++ {
		name, value, matched := generatedCommandProvenanceArg(args[index])
		if !matched {
			clean = append(clean, args[index])
			continue
		}
		present = true
		if _, duplicate := values[name]; duplicate {
			return nil, GeneratedCommandProvenance{}, true, invalidGeneratedCommandProvenance("generated command provenance flag is duplicated")
		}
		if value == "" {
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") {
				return nil, GeneratedCommandProvenance{}, true, invalidGeneratedCommandProvenance("generated command provenance flag is missing a value")
			}
			index++
			value = args[index]
		}
		values[name] = value
	}
	if !present {
		return append([]string(nil), args...), GeneratedCommandProvenance{}, false, nil
	}
	for _, name := range []string{GeneratedByExecutableFlag, GeneratedBySHA256Flag, GeneratedForGenerationFlag} {
		if values[name] == "" {
			return nil, GeneratedCommandProvenance{}, true, invalidGeneratedCommandProvenance("generated command provenance envelope is incomplete")
		}
	}
	generation, err := strconv.ParseUint(values[GeneratedForGenerationFlag], 10, 64)
	if err != nil {
		return nil, GeneratedCommandProvenance{}, true, invalidGeneratedCommandProvenance("generated command lease generation is invalid")
	}
	evidence := GeneratedCommandProvenance{
		ExecutablePath: values[GeneratedByExecutableFlag], ExecutableSHA256: values[GeneratedBySHA256Flag], LeaseGeneration: generation,
	}
	if err := evidence.Validate(); err != nil {
		return nil, GeneratedCommandProvenance{}, true, err
	}
	return clean, evidence, true, nil
}

func ValidateGeneratedCommandInvocation(expected, observed GeneratedCommandProvenance, durableGeneration uint64) error {
	if err := expected.Validate(); err != nil {
		return err
	}
	if err := observed.Validate(); err != nil {
		return err
	}
	if expected.LeaseGeneration != durableGeneration || observed.LeaseGeneration != durableGeneration {
		return &GeneratedCommandProvenanceError{
			Code: "generated_command_generation_mismatch", Message: "generated command lease generation does not match durable state",
			Expected: expected, Observed: observed, Generation: durableGeneration,
		}
	}
	if expected.ExecutablePath != observed.ExecutablePath || expected.ExecutableSHA256 != observed.ExecutableSHA256 {
		return &GeneratedCommandProvenanceError{
			Code: "generated_command_binary_provenance_mismatch", Message: "generated command binary provenance does not match the executing binary",
			Expected: expected, Observed: observed, Generation: durableGeneration,
		}
	}
	return nil
}

type GeneratedCommandProvenanceError struct {
	Code       string
	Message    string
	Expected   GeneratedCommandProvenance
	Observed   GeneratedCommandProvenance
	Generation uint64
}

func NewGeneratedCommandProvenanceObservationError(error) error {
	return &GeneratedCommandProvenanceError{
		Code:    "generated_command_provenance_observation_failed",
		Message: "generated command binary provenance observation failed",
	}
}

func (e *GeneratedCommandProvenanceError) Error() string {
	if e == nil {
		return "generated command provenance error"
	}
	return e.Message
}

func (e *GeneratedCommandProvenanceError) IssueOpsErrorFields() map[string]any {
	if e == nil {
		return nil
	}
	return map[string]any{
		"code":                e.Code,
		"expected_executable": e.Expected.ExecutablePath,
		"expected_sha256":     e.Expected.ExecutableSHA256,
		"observed_executable": e.Observed.ExecutablePath,
		"observed_sha256":     e.Observed.ExecutableSHA256,
		"lease_generation":    e.Generation,
	}
}

func containsControl(value string) bool {
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return true
		}
	}
	return false
}

func generatedCommandProvenanceArg(arg string) (string, string, bool) {
	for _, name := range []string{GeneratedByExecutableFlag, GeneratedBySHA256Flag, GeneratedForGenerationFlag} {
		if arg == name {
			return name, "", true
		}
		if strings.HasPrefix(arg, name+"=") {
			return name, strings.TrimPrefix(arg, name+"="), true
		}
	}
	return "", "", false
}

func invalidGeneratedCommandProvenance(message string) error {
	return &GeneratedCommandProvenanceError{Code: "generated_command_provenance_invalid", Message: message}
}

func quoteGeneratedCommandArg(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
