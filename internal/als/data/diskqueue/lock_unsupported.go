//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package diskqueue

import (
	"errors"
	"os"
)

func tryLock(*os.File) error {
	return errors.New("disk queue file locking is not supported on this operating system")
}

func unlock(*os.File) error {
	return nil
}
