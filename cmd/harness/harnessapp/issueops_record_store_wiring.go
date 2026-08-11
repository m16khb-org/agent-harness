package harnessapp

import (
	"io"
	"time"

	"agent-harness/internal/adapter/outbound/issueopsrecord"
)

const issueOpsSlowSpanThreshold = 100 * time.Millisecond

func issueOpsRecordObserver(writer io.Writer) issueopsrecord.Observer {
	return issueopsrecord.NewJSONLineObserver(writer, issueOpsSlowSpanThreshold)
}

func issueOpsRecordStore(
	scope string,
	observers ...issueopsrecord.Observer,
) issueopsrecord.Store {
	var observer issueopsrecord.Observer
	if len(observers) > 0 {
		observer = observers[0]
	}
	return issueopsrecord.Store{Scope: scope, Observer: observer}
}
