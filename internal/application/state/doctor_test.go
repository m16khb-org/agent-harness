package state

import "testing"

func TestDoctorOwnsEntryAndCurrentRecordClassification(t *testing.T) {
	result := Doctor("/state", []DoctorEntry{
		{Name: "harness.db", Path: "/state/harness.db"},
		{Name: "foreign", Path: "/state/foreign", IsDir: true},
	}, []DoctorRow{
		{Key: "good", Path: "/state/good.json", Data: []byte(`{"schema_version":1,"key":"good","content":"ok","updated_at":"2026-08-04T00:00:00Z","bytes":2}`)},
		{Key: "bad", Path: "/state/bad.json", Data: []byte(`{"schema_version":2}`)},
	})
	if !result.OK || result.Healthy || result.Checked != 2 || len(result.ValidKeys) != 1 || result.ValidKeys[0] != "good" {
		t.Fatalf("Doctor() = %+v", result)
	}
	if len(result.Issues) != 2 || result.Issues[0].Code != "invalid_state" || result.Issues[1].Code != "unexpected_directory" {
		t.Fatalf("issues = %+v", result.Issues)
	}
}
