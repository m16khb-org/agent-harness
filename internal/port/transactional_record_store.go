package port

import "context"

// TransactionalRecordStore는 release repository가 proven SQLite store에 요구하는
// 최소 원자 연산만 노출한다. concrete store 선택은 composition root가 소유한다.
type TransactionalRecordStore interface {
	WithSpan(context.Context, func(context.Context) error) error
	Get(string, string) ([]byte, bool, error)
	Apply(context.Context, []RecordMutation) error
}

type RecordMutation struct {
	Bucket        string
	ID            string
	Data          []byte
	Delete        bool
	RequireAbsent bool
}
