package failurecause

import (
	failurecausecontract "issueops/internal/contract/failurecause"
	"reflect"
	"strings"
	"testing"
)

func TestClassifyPrecedence(t *testing.T) {
	for _, tt := range []struct {
		failed bool
		items  []failurecausecontract.Evidence
		want   failurecausecontract.Cause
	}{{false, nil, failurecausecontract.None}, {true, nil, failurecausecontract.Unknown}, {true, []failurecausecontract.Evidence{{Cause: failurecausecontract.Model}}, failurecausecontract.Model}, {true, []failurecausecontract.Evidence{{Cause: failurecausecontract.Model}, {Cause: failurecausecontract.ContractInput}}, failurecausecontract.ContractInput}, {true, []failurecausecontract.Evidence{{Cause: failurecausecontract.Model}, {Cause: failurecausecontract.HarnessEnvironment}}, failurecausecontract.HarnessEnvironment}, {true, []failurecausecontract.Evidence{{Cause: failurecausecontract.Model}, {Cause: failurecausecontract.Transport}}, failurecausecontract.Transport}} {
		if got := Classify(tt.failed, tt.items).Cause; got != tt.want {
			t.Fatalf("got %s want %s", got, tt.want)
		}
	}
}

func TestClassifySanitizesSortsAndBuildsReasonFromCodes(t *testing.T) {
	result := Classify(true, []failurecausecontract.Evidence{
		{Cause: failurecausecontract.Transport, Code: "second code\nwith detail", Source: "child pipe\nfreeform stderr"},
		{Cause: failurecausecontract.Transport, Code: "first/code", Source: "mcp initialize"},
		{Cause: failurecausecontract.Model, Code: "ignored", Source: "model"},
	})
	if result.Cause != failurecausecontract.Transport || result.Reason != "transport:first_code+second_code_with_detail" {
		t.Fatalf("result=%#v", result)
	}
	want := []failurecausecontract.Evidence{
		{Cause: failurecausecontract.Model, Code: "ignored", Source: "model"},
		{Cause: failurecausecontract.Transport, Code: "first_code", Source: "mcp_initialize"},
		{Cause: failurecausecontract.Transport, Code: "second_code_with_detail", Source: "child_pipe_freeform_stderr"},
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
	result := Classify(true, []failurecausecontract.Evidence{{Cause: failurecausecontract.Transport, Code: "password=topsecret", Source: "authorization=Bearer abc"}})
	encoded := result.Reason + result.Evidence[0].Code + result.Evidence[0].Source
	if strings.Contains(encoded, "topsecret") || strings.Contains(encoded, "bearer") || strings.Contains(encoded, "abc") {
		t.Fatalf("secret-bearing evidence survived: %#v", result)
	}
}
