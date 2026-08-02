package state

import "errors"

var ErrInvalidState = errors.New("invalid state")

func Invalid(_ string) error { return ErrInvalidState }
