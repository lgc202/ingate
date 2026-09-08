package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/tidwall/wal"
)

func serve(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	address := flags.String("address", ":8081", "HTTP listen address")
	if err := flags.Parse(args); err != nil {
		return err
	}

	server := &http.Server{
		Addr:              *address,
		ReadHeaderTimeout: 5 * time.Second,
		Handler: http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(http.StatusOK)
			_, _ = response.Write([]byte("ok\n"))
		}),
	}

	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func copyCorruptQueue(args []string) (err error) {
	flags := flag.NewFlagSet("corrupt", flag.ContinueOnError)
	source := flags.String("source", "", "source WAL directory")
	destination := flags.String("destination", "", "corrupted WAL directory")
	segmentBytes := flags.Int("segment-bytes", 64<<10, "WAL segment size")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *source == "" || *destination == "" {
		return errors.New("source and destination are required")
	}
	if err := os.MkdirAll(filepath.Dir(*destination), 0o700); err != nil {
		return fmt.Errorf("create corrupt queue parent: %w", err)
	}

	input, err := wal.Open(*source, nil)
	if err != nil {
		return fmt.Errorf("open source WAL: %w", err)
	}
	defer func() { err = errors.Join(err, input.Close()) }()
	first, err := input.FirstIndex()
	if err != nil {
		return fmt.Errorf("read source WAL first index: %w", err)
	}
	last, err := input.LastIndex()
	if err != nil {
		return fmt.Errorf("read source WAL last index: %w", err)
	}
	if last < first || last-first+1 < 3 {
		return fmt.Errorf("source WAL contains fewer than three entries")
	}

	output, err := wal.Open(*destination, &wal.Options{
		NoSync:      false,
		SegmentSize: *segmentBytes,
		LogFormat:   wal.Binary,
		AllowEmpty:  true,
		DirPerms:    0o700,
		FilePerms:   0o600,
	})
	if err != nil {
		return fmt.Errorf("open corrupt WAL: %w", err)
	}
	defer func() { err = errors.Join(err, output.Close()) }()

	middle := first + (last-first)/2
	var destinationIndex uint64 = 1
	for sourceIndex := first; sourceIndex <= last; sourceIndex++ {
		value, err := input.Read(sourceIndex)
		if err != nil {
			return fmt.Errorf("read source WAL entry %d: %w", sourceIndex, err)
		}
		if sourceIndex == middle {
			if len(value) == 0 {
				return fmt.Errorf("source WAL entry %d is empty", sourceIndex)
			}
			// 只破坏 Ingate 自有条目正文并通过 tidwall/wal 公共 API 重新写入，
			// 这样能验证 QueueEntry 校验，而不依赖第三方 WAL 的私有分段格式。
			value[len(value)/2] ^= 0xff
		}
		if err := output.Write(destinationIndex, value); err != nil {
			return fmt.Errorf("write corrupt WAL entry %d: %w", destinationIndex, err)
		}
		destinationIndex++
		if sourceIndex == last {
			break
		}
	}

	return output.Sync()
}
