//go:build !wasm

package redisabi

import (
	"errors"
	"time"
)

var errHostUnavailable = errors.New("redis ABI hostcalls require a Wasm build")

func hostInit(string, time.Duration) error {
	return errHostUnavailable
}

func hostCall(string, []byte) (uint32, error) {
	return 0, errHostUnavailable
}

func hostResponse(int32) ([]byte, error) {
	return nil, errHostUnavailable
}
