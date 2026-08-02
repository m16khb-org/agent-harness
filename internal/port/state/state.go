package state

type Reader interface {
	Get(bucket, id string) ([]byte, bool, error)
}

type TransactionalStore interface {
	Reader
	Mutate([]Mutation) error
}

type Mutation struct {
	Bucket        string
	ID            string
	Data          []byte
	Delete        bool
	RequireAbsent bool
}
