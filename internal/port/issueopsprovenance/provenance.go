package issueopsprovenance

import (
	"context"
)

type Receipt struct {
	ExecutablePath   string
	ExecutableSHA256 string
}

type Observer interface {
	Observe(context.Context) (Receipt, error)
}
