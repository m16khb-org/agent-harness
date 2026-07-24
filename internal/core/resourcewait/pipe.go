package resourcewait

import (
	"errors"
	"os"
	"syscall"
	"time"
)

func MeasurePipeCapacity() (int, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return 0, err
	}
	defer reader.Close()
	defer writer.Close()

	progress := make(chan int, 64)
	done := make(chan error, 1)
	total := 0
	go func() {
		chunk := make([]byte, 512)
		for {
			n, writeErr := writer.Write(chunk)
			if n > 0 {
				progress <- n
			}
			if writeErr != nil {
				done <- writeErr
				return
			}
		}
	}()

	idle := time.NewTimer(100 * time.Millisecond)
	defer idle.Stop()
	for total < 1<<20 {
		select {
		case n := <-progress:
			total += n
			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			idle.Reset(100 * time.Millisecond)
		case writeErr := <-done:
			if errors.Is(writeErr, os.ErrClosed) || errors.Is(writeErr, syscall.EPIPE) {
				return total, nil
			}
			return total, writeErr
		case <-idle.C:
			_ = reader.Close()
			_ = writer.Close()
			select {
			case writeErr := <-done:
				if writeErr != nil && !errors.Is(writeErr, os.ErrClosed) && !errors.Is(writeErr, syscall.EPIPE) {
					return total, writeErr
				}
			case <-time.After(time.Second):
			}
			return total, nil
		}
	}
	return total, nil
}
