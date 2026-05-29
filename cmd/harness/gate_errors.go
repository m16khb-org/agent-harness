package main

import "errors"

var (
	errAPIDocReviewGateFailed     = errors.New("api documentation AI review gate failed")
	errAPIDocStaticGateFailed     = errors.New("api documentation static check gate failed")
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
