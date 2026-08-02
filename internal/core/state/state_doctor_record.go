package state

import (
	statecontract "agent-harness/internal/contract/state"
	"agent-harness/internal/core/state/statepath"
)

type stateDoctorRecordInspection struct {
	Valid  bool
	Record statecontract.RecordEnvelope
	Issues []StateDoctorIssue
}

// inspectStateDoctorRecord validates one stored record row. path is the
// record's stable identifier path used in issue reports; data is the raw row
// payload.
func inspectStateDoctorRecord(path, key string, data []byte) stateDoctorRecordInspection {
	if _, err := NormalizeStateKey(key); err != nil {
		return stateDoctorRecordInspection{Issues: []StateDoctorIssue{{
			Path:     path,
			Key:      key,
			Severity: "error",
			Code:     "invalid_key",
			Message:  err.Error(),
		}}}
	}
	record, err := decodeStateRecord(key, data)
	if err != nil {
		return invalidStateDoctorInspection(path, key)
	}
	return validateStateDoctorRecord(path, key, record)
}

func validateStateDoctorRecord(path, key string, record statecontract.RecordEnvelope) stateDoctorRecordInspection {
	if _, err := statepath.ParseTime(record.UpdatedAt); err != nil {
		return invalidStateDoctorInspection(path, key)
	}
	return stateDoctorRecordInspection{Valid: true, Record: record}
}

func invalidStateDoctorInspection(path, key string) stateDoctorRecordInspection {
	return stateDoctorRecordInspection{Issues: []StateDoctorIssue{{
		Path:     path,
		Key:      key,
		Severity: "error",
		Code:     "invalid_state",
		Message:  "invalid state",
	}}}
}
