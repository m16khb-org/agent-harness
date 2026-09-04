package apidoc

import "fmt"

func printAPIDocStaticCheck(result apiDocStaticResult) {
	if result.OK {
		fmt.Println(result.Summary)
		return
	}
	fmt.Println(result.Summary)
	for _, v := range result.Violations {
		fmt.Printf("- %s:%d %s: %s\n", v.File, v.Line, v.Code, v.Message)
	}
}
