//go:build !unix

package issueops

import (
	"errors"
	"os"
)

func publicationImmutableLockFallbackAllowed(error) bool {
	return false
}

func publicationEffectiveUID() (uint32, bool) {
	return 0, false
}

func publicationPathIdentity(os.FileInfo) (uint32, uint64, uint64, bool) {
	return 0, 0, 0, false
}

func publicationPathWritable(string) (bool, error) {
	return false, errors.New("publication immutable config authority is unsupported on this platform")
}
