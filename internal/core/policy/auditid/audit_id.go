package auditid

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"
)

func Generate(workspaceRoot, cwd string, argv []string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(workspaceRoot))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(cwd))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strings.Join(argv, "\x00")))
	return fmt.Sprintf("audit-%s-%08x", time.Now().UTC().Format("20060102T150405Z"), h.Sum32())
}
