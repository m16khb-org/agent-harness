package daemonlock

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func Acquire(path string, currentPID func() int, processAlive func(int) bool) (*os.File, error) {
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = f.WriteString(strconv.Itoa(currentPID()) + "\n")
			return f, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if stale(path, 30*time.Second, processAlive) {
			_ = os.Remove(path)
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("cannot acquire daemon lock %s", path)
}

func stale(path string, maxAge time.Duration, processAlive func(int) bool) bool {
	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	if time.Since(info.ModTime()) > maxAge {
		return true
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return false
	}
	return !processAlive(pid)
}
