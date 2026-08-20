package mcpcli

import (
	"agent-harness/internal/adapter/looprun"
	"agent-harness/internal/adapter/projectdocs"
)

func init() {
	LoopStart = looprun.Start
	LoopRecordAttempt = looprun.RecordAttempt
	LoopStop = looprun.Stop
	LoopStatus = looprun.Status
	RouteProjectDocs = projectdocs.RouteProjectDocs
	ReadProjectDoc = projectdocs.ReadProjectDoc
	ReviseProjectDoc = projectdocs.ReviseProjectDoc
	AppendProjectDocsEntry = projectdocs.AppendProjectDocsEntry
}
