package log

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type closeTrackingBuffer struct {
	bytes.Buffer
	closeCalls int
}

func TestFileOutputIsLazyAndMatchesStandardOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "app.log")
	var stdout, stderr bytes.Buffer
	logger, closeLog, err := newLogger(Config{Outputs: []Output{OutputStdout, OutputFile}, File: FileConfig{Path: path}}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("newLogger(stdout, file) error = %v, want nil", err)
	}

	cleanupLog(t, closeLog)
	if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("New(file) parent directory error = %v, want not created before first write", err)
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "visible", 0)
	if err := logger.Handler().Handle(t.Context(), record); err != nil {
		t.Fatalf("Handle(file) error = %v, want nil", err)
	}

	if err := closeLog(); err != nil {
		t.Errorf("Close(file) error = %v, want nil", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(log) error = %v, want nil", err)
	}

	if string(data) != stdout.String() || !strings.Contains(string(data), `"msg":"visible"`) || stderr.Len() != 0 {
		t.Errorf("file/stdout/stderr = %q/%q/%q, want identical visible record in file/stdout only", data, stdout.String(), stderr.String())
	}
}

func TestFileOutputAppendsExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	if err := os.WriteFile(path, []byte("existing\n"), 0o600); err != nil {
		t.Fatalf("write existing log: %v", err)
	}

	logger, closeLog, err := New(Config{Outputs: []Output{OutputFile}, File: FileConfig{Path: path}})
	if err != nil {
		t.Fatalf("New(existing file) error = %v, want nil", err)
	}

	cleanupLog(t, closeLog)
	logger.Info("appended")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(appended log) error = %v, want nil", err)
	}

	if !strings.HasPrefix(string(data), "existing\n") || !strings.Contains(string(data), `"msg":"appended"`) {
		t.Errorf("existing log after write = %q, want existing contents followed by appended record", data)
	}
}

func TestClosingLoggerDoesNotCloseStandardStreams(t *testing.T) {
	var stdout, stderr closeTrackingBuffer
	logger, closeLog, err := newLogger(Config{Outputs: []Output{OutputStdout, OutputStderr}}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("newLogger(standard streams) error = %v, want nil", err)
	}

	if err := closeLog(); err != nil {
		t.Errorf("Close(standard streams) error = %v, want nil", err)
	}

	logger.Info("still writable")
	if stdout.closeCalls != 0 || stderr.closeCalls != 0 || stdout.Len() == 0 || stdout.String() != stderr.String() {
		t.Errorf("standard stream close calls = %d/%d, outputs = %q/%q, want 0/0 and identical records", stdout.closeCalls, stderr.closeCalls, stdout.String(), stderr.String())
	}
}

func TestNewRejectsUnsafeFilePathsAndRotationLimits(t *testing.T) {
	dir := t.TempDir()
	parentFile := filepath.Join(dir, "parent-file")
	if err := os.WriteFile(parentFile, []byte("unchanged"), 0o600); err != nil {
		t.Fatalf("create non-directory parent: %v", err)
	}

	tests := []struct {
		name string
		file FileConfig
	}{
		{name: "missing path"},
		{name: "directory", file: FileConfig{Path: dir}},
		{name: "nonregular file", file: FileConfig{Path: os.DevNull}},
		{name: "stat failure", file: FileConfig{Path: filepath.Join(parentFile, "app.log")}},
		{name: "negative size", file: FileConfig{Path: filepath.Join(dir, "app.log"), MaxSizeMB: -1}},
		{name: "negative backups", file: FileConfig{Path: filepath.Join(dir, "app.log"), MaxBackups: -1}},
		{name: "negative age", file: FileConfig{Path: filepath.Join(dir, "app.log"), MaxAgeDays: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, closeLog, err := New(Config{Outputs: []Output{OutputFile}, File: tt.file})
			if logger != nil || closeLog != nil || err == nil {
				t.Errorf("New(%s) = %v, close present=%t, error=%v, want nil/nil/error", tt.name, logger, closeLog != nil, err)
			}
		})
	}

	data, err := os.ReadFile(parentFile)
	if err != nil || string(data) != "unchanged" {
		t.Errorf("unsafe path validation changed parent file: contents=%q, error=%v, want unchanged/nil", data, err)
	}
}

func TestLaterInvalidOutputDoesNotCreateLogFile(t *testing.T) {
	for _, outputs := range [][]Output{{OutputFile, "invalid"}, {OutputFile, OutputFile}} {
		path := filepath.Join(t.TempDir(), "logs", "app.log")
		logger, closeLog, err := New(Config{Outputs: outputs, File: FileConfig{Path: path}})
		if logger != nil || closeLog != nil || err == nil {
			t.Errorf("New(outputs=%v) = %v, close present=%t, error=%v, want nil/nil/error", outputs, logger, closeLog != nil, err)
		}

		if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("New(outputs=%v) parent directory error = %v, want no filesystem side effect", outputs, err)
		}
	}
}

func TestFileSizeRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	seed := bytes.Repeat([]byte("x"), 1024*1024)
	if err := os.WriteFile(path, seed, 0o600); err != nil {
		t.Fatalf("write full log: %v", err)
	}

	logger, closeLog, err := New(Config{Outputs: []Output{OutputFile}, File: FileConfig{Path: path, MaxSizeMB: 1}})
	if err != nil {
		t.Fatalf("New(size-limited file) error = %v, want nil", err)
	}

	cleanupLog(t, closeLog)
	if err := logger.Handler().Handle(t.Context(), slog.NewRecord(time.Now(), slog.LevelInfo, "rotated", 0)); err != nil {
		t.Fatalf("Handle(rotation) error = %v, want nil", err)
	}

	backups, err := filepath.Glob(filepath.Join(dir, "app-*.log"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("rotation backups = %v, error=%v, want one backup", backups, err)
	}

	data, err := os.ReadFile(backups[0])
	if err != nil || !bytes.Equal(data, seed) {
		t.Errorf("rotation backup size=%d, error=%v, want original %d bytes", len(data), err, len(seed))
	}

	current, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(current), `"msg":"rotated"`) || len(current) >= len(seed) {
		t.Errorf("current log size=%d, error=%v, want only new rotated record", len(current), err)
	}
}

func TestFileBackupRetentionAndCompression(t *testing.T) {
	tests := []struct {
		name    string
		file    FileConfig
		keepOld bool
	}{
		{name: "count", file: FileConfig{MaxBackups: 1, Compress: true}},
		{name: "age", file: FileConfig{MaxAgeDays: 1, Compress: true}},
		{name: "zero limits preserve all backups", file: FileConfig{Compress: true}, keepOld: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			old := filepath.Join(dir, "app-2000-01-01T00-00-00.000.log")
			fresh := filepath.Join(dir, "app-"+time.Now().UTC().Add(-time.Hour).Format("2006-01-02T15-04-05.000")+".log")
			for _, path := range []string{old, fresh} {
				if err := os.WriteFile(path, []byte("backup contents\n"), 0o600); err != nil {
					t.Fatalf("seed backup: %v", err)
				}
			}

			tt.file.Path = filepath.Join(dir, "app.log")
			logger, closeLog, err := New(Config{Outputs: []Output{OutputFile}, File: tt.file})
			if err != nil {
				t.Fatalf("New(%s retention) error = %v, want nil", tt.name, err)
			}

			cleanupLog(t, closeLog)
			logger.Info("trigger cleanup")
			waitForLogMaintenance(t, old, fresh, tt.keepOld)
			file, err := os.Open(fresh + ".gz")
			if err != nil {
				t.Fatalf("Open(compressed backup) error = %v, want nil", err)
			}

			defer func() {
				if err := file.Close(); err != nil {
					t.Errorf("Close(compressed backup) error = %v, want nil", err)
				}
			}()
			reader, err := gzip.NewReader(file)
			if err != nil {
				t.Fatalf("gzip.NewReader() error = %v, want nil", err)
			}

			data, err := io.ReadAll(reader)
			if err != nil || string(data) != "backup contents\n" {
				t.Errorf("compressed backup contents=%q, error=%v, want original contents/nil", data, err)
			}

			if err := reader.Close(); err != nil {
				t.Errorf("Close(gzip reader) error = %v, want nil", err)
			}
		})
	}
}

func (b *closeTrackingBuffer) Close() error {
	b.closeCalls++
	return nil
}

func cleanupLog(t *testing.T, closeLog func() error) {
	t.Helper()
	t.Cleanup(func() {
		if err := closeLog(); err != nil {
			t.Errorf("Close(log file) error = %v, want nil", err)
		}
	})
}

func waitForLogMaintenance(t *testing.T, old, fresh string, keepOld bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		_, oldErr := os.Stat(old)
		_, freshErr := os.Stat(fresh)
		_, compressedErr := os.Stat(fresh + ".gz")
		_, oldCompressedErr := os.Stat(old + ".gz")
		oldMatches := (keepOld && oldCompressedErr == nil) || (!keepOld && errors.Is(oldCompressedErr, os.ErrNotExist))
		if errors.Is(oldErr, os.ErrNotExist) && errors.Is(freshErr, os.ErrNotExist) && compressedErr == nil && oldMatches {
			return
		}

		select {
		case <-ctx.Done():
			t.Fatalf("log maintenance old/fresh/compressed/old-compressed errors=%v/%v/%v/%v, want missing/missing/nil and old retained=%t", oldErr, freshErr, compressedErr, oldCompressedErr, keepOld)
		case <-ticker.C:
		}
	}
}
