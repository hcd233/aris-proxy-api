//go:build darwin || linux

package tracecli

import (
	"os"
	"path/filepath"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"golang.org/x/sys/unix"
)

func withFileLock(path string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "create lock directory")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "open file lock")
	}
	defer func() { _ = file.Close() }() //nolint:errcheck // best-effort close
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		return ierr.Wrap(ierr.ErrResourceLocked, err, "acquire file lock")
	}
	defer func() { _ = unix.Flock(int(file.Fd()), unix.LOCK_UN) }() //nolint:errcheck // best-effort unlock
	return fn()
}
