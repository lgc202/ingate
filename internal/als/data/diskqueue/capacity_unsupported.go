//go:build !darwin && !linux

package diskqueue

import "errors"

func inspectStorage(string) (storageUsage, error) {
	return storageUsage{}, errors.New("disk queue capacity inspection is unsupported on this platform")
}
