package policy

import "os"

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func existsForTest(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
