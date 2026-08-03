//go:build !darwin && !linux

package installutil

import "fmt"

func exchangeManagedCommandPaths(_, _ string) error {
	return fmt.Errorf("atomic managed command exchange is unsupported on this platform")
}
