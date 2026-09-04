package apidoc

import (
	"encoding/json"
	"fmt"
)

func mustJSON(value any) []byte {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	return b
}

func printAPIDocReview(result apiDocReviewResult) {
	fmt.Printf("API doc review verdict: %s\n", result.Verdict)
	if result.Summary != "" {
		fmt.Println(result.Summary)
	}
	for _, finding := range result.Findings {
		location := finding.File
		if finding.Line != nil {
			location = fmt.Sprintf("%s:%d", finding.File, *finding.Line)
		}
		fmt.Printf("- [%s] %s %s\n", finding.Severity, location, finding.Message)
	}
}
