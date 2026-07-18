package auditid

import (
	"fmt"
	"hash/fnv"
	"strings"
	"sync/atomic"
	"time"
)

var sequence atomic.Uint64

func Generate(workspaceRoot, cwd string, argv []string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(workspaceRoot))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(cwd))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strings.Join(argv, "\x00")))
	return fmt.Sprintf("audit-%s-%016x-%08x", time.Now().UTC().Format("20060102T150405.000000000Z"), sequence.Add(1), h.Sum32())
}
