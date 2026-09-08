package diskqueue

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/lgc202/ingate/internal/als/biz"
)

// TestCapacityPolicyStates 验证物理占用和剩余空间都会推进 WAL 容量状态。
func TestCapacityPolicyStates(t *testing.T) {
	policy := capacityPolicy{
		capacityBytes: 1_100,
		minFreeBytes:  100,
		segmentBytes:  100,
	}
	tests := []struct {
		name  string
		usage storageUsage
		want  biz.QueueState
	}{
		{name: "healthy", usage: storageUsage{diskBytes: 100, freeBytes: 1_000}, want: biz.QueueHealthy},
		{name: "capacity warning", usage: storageUsage{diskBytes: 720, freeBytes: 1_000}, want: biz.QueueWarning},
		{name: "capacity critical", usage: storageUsage{diskBytes: 810, freeBytes: 1_000}, want: biz.QueueCritical},
		{name: "capacity blocked", usage: storageUsage{diskBytes: 900, freeBytes: 1_000}, want: biz.QueueBlocked},
		{name: "legacy segment blocked", usage: storageUsage{diskBytes: 500, largestFileBytes: 600, freeBytes: 1_000}, want: biz.QueueBlocked},
		{name: "allocation block cannot fit", usage: storageUsage{diskBytes: 897, freeBytes: 1_000, blockBytes: 4}, want: biz.QueueBlocked},
		{name: "free warning", usage: storageUsage{diskBytes: 100, freeBytes: 600}, want: biz.QueueWarning},
		{name: "free critical", usage: storageUsage{diskBytes: 100, freeBytes: 400}, want: biz.QueueCritical},
		{name: "free blocked", usage: storageUsage{diskBytes: 100, freeBytes: 200}, want: biz.QueueBlocked},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := policy.capacityState(test.usage); got != test.want {
				t.Errorf("capacityPolicy.capacityState() = %s, want %s", got, test.want)
			}
		})
	}
}

// TestCapacityPolicyAdmission 验证追加必须同时满足目录容量和文件系统安全余量。
func TestCapacityPolicyAdmission(t *testing.T) {
	policy := capacityPolicy{
		capacityBytes: 1_100,
		minFreeBytes:  100,
		segmentBytes:  100,
	}
	tests := []struct {
		name   string
		usage  storageUsage
		growth int64
		want   bool
	}{
		{name: "within both limits", usage: storageUsage{diskBytes: 700, freeBytes: 500}, growth: 200, want: true},
		{name: "physical capacity exhausted", usage: storageUsage{diskBytes: 701, freeBytes: 1_000}, growth: 200},
		{name: "filesystem reserve exhausted", usage: storageUsage{diskBytes: 100, freeBytes: 399}, growth: 200},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := policy.admits(test.usage, test.growth); got != test.want {
				t.Errorf("capacityPolicy.admits() = %t, want %t", got, test.want)
			}
		})
	}
}

// TestInspectStorageIncludesEveryFile 验证容量统计不会遗漏临时文件、锁和存储元数据。
func TestInspectStorageIncludesEveryFile(t *testing.T) {
	path := t.TempDir()
	before, err := inspectStorage(path)
	if err != nil {
		t.Fatalf("inspectStorage(empty directory) error = %v, want nil", err)
	}

	data := bytes.Repeat([]byte{0x7f}, 8<<10)
	for _, name := range []string{"00000000000000000001", "segment.TEMP", lockFilename, "metadata"} {
		if err := os.WriteFile(filepath.Join(path, name), data, fileMode); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v, want nil", name, err)
		}
	}
	after, err := inspectStorage(path)
	if err != nil {
		t.Fatalf("inspectStorage(populated directory) error = %v, want nil", err)
	}
	if after.diskBytes <= before.diskBytes {
		t.Errorf("physical usage after adding WAL files = %d, want greater than %d", after.diskBytes, before.diskBytes)
	}
}

// TestInspectStorageFollowsQueueSymlink 验证容量探测和 WAL 使用同一个真实目录。
func TestInspectStorageFollowsQueueSymlink(t *testing.T) {
	realPath := t.TempDir()
	queuePath := filepath.Join(t.TempDir(), "queue")
	if err := os.Symlink(realPath, queuePath); err != nil {
		t.Fatalf("os.Symlink() error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(realPath, "segment"), bytes.Repeat([]byte{1}, 8<<10), fileMode); err != nil {
		t.Fatalf("os.WriteFile(segment) error = %v, want nil", err)
	}

	resolved, err := prepareDirectory(queuePath)
	if err != nil {
		t.Fatalf("prepareDirectory(symlink) error = %v, want nil", err)
	}
	usage, err := inspectStorage(resolved)
	if err != nil {
		t.Fatalf("inspectStorage(resolved path) error = %v, want nil", err)
	}
	if usage.diskBytes == 0 {
		t.Error("inspectStorage(resolved path) disk bytes = 0, want segment usage")
	}
}
