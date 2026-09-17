package lock

import (
	"errors"
	"os"
	"time"
)

// ErrTimeout is returned by Acquire when the exclusive lock cannot be taken
// before the deadline.
var ErrTimeout = errors.New("lock: timeout")

const pollInterval = 50 * time.Millisecond

// Acquire takes an exclusive advisory lock on path (creating the file).
// Blocks up to timeout, polling tryLock every 50ms. The returned release
// function unlocks and closes the file but does not remove it.
func Acquire(path string, timeout time.Duration) (release func() error, err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timeout)
	for {
		err := tryLock(f)
		if err == nil {
			return func() error {
				uerr := tryUnlock(f)
				cerr := f.Close()
				if uerr != nil {
					return uerr
				}
				return cerr
			}, nil
		}
		if !isBusy(err) {
			f.Close()
			return nil, err
		}
		if timeout == 0 || !time.Now().Before(deadline) {
			f.Close()
			return nil, ErrTimeout
		}
		sleep := pollInterval
		if rem := time.Until(deadline); rem < sleep {
			sleep = rem
		}
		if sleep <= 0 {
			f.Close()
			return nil, ErrTimeout
		}
		time.Sleep(sleep)
	}
}
