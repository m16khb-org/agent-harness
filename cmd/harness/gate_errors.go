package main

import (
	"errors"

	"agent-harness/cmd/harness/apidoc"
)

var (
	errAPIDocReviewGateFailed     = apidoc.ErrReviewGateFailed
	errAPIDocStaticGateFailed     = apidoc.ErrStaticGateFailed
	errSelfVerificationGateFailed = errors.New("self-verification quality gate failed")
)

func isAPIDocReviewGateError(err error) bool {
	return errors.Is(err, errAPIDocReviewGateFailed)
}

func isAPIDocStaticGateError(err error) bool {
	return errors.Is(err, errAPIDocStaticGateFailed)
}

func isSelfVerificationGateError(err error) bool {
	return errors.Is(err, errSelfVerificationGateFailed)
}
