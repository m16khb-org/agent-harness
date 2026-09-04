package issueopsrecord

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
	statecontract "issueops/internal/contract/state"
)

func Decode(id string, data []byte) (issueopscontract.IssueOpsRecord, error) {
	invalid := issueopscontract.IssueOpsRecord{
		OK:            false,
		ID:            id,
		Invalid:       true,
		InvalidReason: statecontract.ErrInvalidState.Error(),
	}
	normalizedID, err := NormalizeID(id)
	if err != nil || normalizedID != id {
		return invalid, statecontract.ErrInvalidState
	}
	var record issueopscontract.IssueOpsRecord
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return invalid, statecontract.ErrInvalidState
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return invalid, statecontract.ErrInvalidState
	}
	if record.ID != id || record.SchemaVersion != issueopscontract.IssueOpsSchemaVersion {
		return invalid, statecontract.ErrInvalidState
	}
	if err := issueopscontract.ValidateRecord(record); err != nil {
		return invalid, statecontract.ErrInvalidState
	}
	record.OK = true
	return record, nil
}

func Encode(record issueopscontract.IssueOpsRecord) ([]byte, error) {
	if _, err := NormalizeID(record.ID); err != nil {
		return nil, err
	}
	if err := issueopscontract.ValidateRecord(record); err != nil {
		return nil, statecontract.ErrInvalidState
	}
	record.OK = true
	return json.MarshalIndent(record, "", "  ")
}

func NormalizeID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}
	if !strings.HasPrefix(id, "io-") || strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return "", fmt.Errorf("invalid issueops id %q", id)
	}
	return id, nil
}
