//go:build unix

package issueops

import (
	"errors"
	"os"
	"syscall"
)

func publicationImmutableLockFallbackAllowed(err error) bool {
	return errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EROFS)
}

func publicationEffectiveUID() (uint32, bool) {
	return uint32(os.Geteuid()), true
}

func publicationPathIdentity(info os.FileInfo) (uint32, uint64, uint64, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, 0, false
	}
	return stat.Uid, uint64(stat.Dev), uint64(stat.Ino), true
}

func publicationPathWritable(path string) (bool, error) {
	err := syscall.Access(path, 2)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EROFS) {
		return false, nil
	}
	return false, err
}
