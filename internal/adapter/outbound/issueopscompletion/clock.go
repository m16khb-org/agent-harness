package issueopscompletion

import "time"

type UTCClock struct{}

func (UTCClock) Now() time.Time { return time.Now().UTC() }
