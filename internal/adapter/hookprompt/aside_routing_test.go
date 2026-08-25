package hookprompt

import (
	"strings"
	"testing"
)

func TestAsideCLIRulePrefersInstalledBrowserAutomation(t *testing.T) {
	var asideRule *HookRoutingRule
	for i := range hookRoutingRules {
		if hookRoutingRules[i].Tool == "aside-cli" {
			asideRule = &hookRoutingRules[i]
			break
		}
	}
	if asideRule == nil {
		t.Fatal("aside-cli routing rule missing")
	}

	english := "take a screenshot of the login page in a browser and verify the flow"
	lower := strings.ToLower(english)
	if !asideRule.Matches(english, lower, false) {
		t.Fatal("english browser prompt should route to aside-cli")
	}

	korean := "브라우저에서 로그인 화면을 확인하고 스크린샷을 남겨줘"
	if !asideRule.Matches(korean, strings.ToLower(korean), false) {
		t.Fatal("korean browser prompt should route to aside-cli")
	}

	unrelated := "refactor the parser module and add table tests"
	if asideRule.Matches(unrelated, strings.ToLower(unrelated), false) {
		t.Fatal("non-browser prompt must not route to aside-cli")
	}
}
