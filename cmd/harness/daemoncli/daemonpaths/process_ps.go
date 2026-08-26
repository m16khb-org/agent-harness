//go:build !linux

package daemonpaths

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// processFieldWithCLocale는 `ps`로 프로세스 필드를 읽는 darwin/other 구현이
// 공유한다. Linux는 /proc을 읽으므로 이 helper를 쓰지 않으며, 그 빌드에서는
// unused로 남지 않도록 build constraint로 제외한다.
func processFieldWithCLocale(pid int, field string) ([]byte, error) {
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", field)
	env := make([]string, 0, len(os.Environ())+3)
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "LANG=") ||
			strings.HasPrefix(value, "LC_ALL=") ||
			strings.HasPrefix(value, "LC_TIME=") {
			continue
		}
		env = append(env, value)
	}
	cmd.Env = append(env, "LANG=C", "LC_ALL=C", "LC_TIME=C")
	return cmd.Output()
}
