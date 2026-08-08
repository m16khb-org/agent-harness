package state

import "context"

type Reader interface {
	Get(bucket, id string) ([]byte, bool, error)
}

type ExistingReader interface {
	GetExisting(dir, bucket, id string) ([]byte, bool, error)
}

type TransactionalStore interface {
	Reader
	Mutate([]Mutation) error
}

type Store interface {
	TransactionalStore
	List(bucket string) ([]string, error)
	WithSpan(context.Context, func(context.Context) error) error
}

type Mutation struct {
	Bucket        string
	ID            string
	Data          []byte
	Delete        bool
	RequireAbsent bool
}
