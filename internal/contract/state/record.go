package state

const SchemaVersion = 1

type RecordEnvelope struct {
	SchemaVersion int    `json:"schema_version"`
	Key           string `json:"key"`
	Content       string `json:"content"`
	UpdatedAt     string `json:"updated_at"`
	Bytes         int    `json:"bytes"`
}
