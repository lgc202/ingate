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

func hostSetEffectiveContext(uint32) error {
	return errHostUnavailable
}

func hostResumeHTTPRequest() error {
	return errHostUnavailable
}

func hostResumeHTTPResponse() error {
	return errHostUnavailable
}

func hostSendHTTPResponse(uint32, map[string]string, []byte) error {
	return errHostUnavailable
}

func hostLogWarning(string) {}
