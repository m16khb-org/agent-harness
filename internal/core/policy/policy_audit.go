package policy

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"
)

func makeAuditLogID(req CommandPolicyRequest) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(req.WorkspaceRoot))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(req.CWD))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strings.Join(req.Argv, "\x00")))
	return fmt.Sprintf("audit-%s-%08x", time.Now().UTC().Format("20060102T150405Z"), h.Sum32())
}
