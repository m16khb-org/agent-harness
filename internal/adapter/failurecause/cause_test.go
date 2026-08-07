package failurecause

import (
	"reflect"
	"strings"
	"testing"
)

func TestClassifyPrecedence(t *testing.T) {
	for _, tt := range []struct {
		failed bool
		items  []Evidence
		want   Cause
	}{{false, nil, None}, {true, nil, Unknown}, {true, []Evidence{{Cause: Model}}, Model}, {true, []Evidence{{Cause: Model}, {Cause: ContractInput}}, ContractInput}, {true, []Evidence{{Cause: Model}, {Cause: HarnessEnvironment}}, HarnessEnvironment}, {true, []Evidence{{Cause: Model}, {Cause: Transport}}, Transport}} {
		if got := Classify(tt.failed, tt.items).Cause; got != tt.want {
			t.Fatalf("got %s want %s", got, tt.want)
		}
	}
}

func TestClassifySanitizesSortsAndBuildsReasonFromCodes(t *testing.T) {
	result := Classify(true, []Evidence{
		{Cause: Transport, Code: "second code\nwith detail", Source: "child pipe\nfreeform stderr"},
		{Cause: Transport, Code: "first/code", Source: "mcp initialize"},
		{Cause: Model, Code: "ignored", Source: "model"},
	})
	if result.Cause != Transport || result.Reason != "transport:first_code+second_code_with_detail" {
		t.Fatalf("result=%#v", result)
	}
	want := []Evidence{
		{Cause: Model, Code: "ignored", Source: "model"},
		{Cause: Transport, Code: "first_code", Source: "mcp_initialize"},
		{Cause: Transport, Code: "second_code_with_detail", Source: "child_pipe_freeform_stderr"},
	}
	if !reflect.DeepEqual(result.Evidence, want) {
		t.Fatalf("evidence=%#v", result.Evidence)
	}
	for _, evidence := range result.Evidence {
		if strings.ContainsAny(evidence.Code+evidence.Source, " \n\t") || len(evidence.Code) > 96 || len(evidence.Source) > 96 {
			t.Fatalf("unsanitized evidence=%#v", evidence)
		}
	}
}

func TestClassifyRedactsSecretBearingEvidenceTokens(t *testing.T) {
	result := Classify(true, []Evidence{{Cause: Transport, Code: "password=topsecret", Source: "authorization=Bearer abc"}})
	encoded := result.Reason + result.Evidence[0].Code + result.Evidence[0].Source
	if strings.Contains(encoded, "topsecret") || strings.Contains(encoded, "bearer") || strings.Contains(encoded, "abc") {
		t.Fatalf("secret-bearing evidence survived: %#v", result)
	}
}
