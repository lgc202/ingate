package diskqueue

import (
	"math"

	"github.com/lgc202/ingate/internal/als/biz"
)

const (
	warningCapacityPercent  int64 = 80
	criticalCapacityPercent int64 = 90
)

type storageUsage struct {
	diskBytes        int64
	largestFileBytes int64
	freeBytes        int64
	blockBytes       int64
}

type storageProbe func(string) (storageUsage, error)

// capacityPolicy 为 WAL 追加保留最坏情况下的分段复制空间。
// tidwall/wal 允许最后一个条目越过分段目标，并在截断队首时先原子写入当前分段副本；
// 每批不超过 segmentBytes 后，两个分段大小可以覆盖这次越界和文件系统分配块取整。
type capacityPolicy struct {
	// capacityBytes 包含正常分段和截断期间短暂存在的分段副本。
	capacityBytes int64
	// minFreeBytes 是不能被 WAL 消耗的文件系统安全余量。
	minFreeBytes int64
	// segmentBytes 同时限制单个 WAL 条目，保证恢复空间具有确定上界。
	segmentBytes int64
}

func (p capacityPolicy) admits(usage storageUsage, growth int64) bool {
	if growth < 0 {
		return false
	}

	recoveryBytes, ok := p.recoveryBytes(usage)
	if !ok || p.capacityBytes < recoveryBytes {
		return false
	}
	writeLimit := p.capacityBytes - recoveryBytes
	if usage.diskBytes > writeLimit || growth > writeLimit-usage.diskBytes {
		return false
	}

	reservedFree, ok := addBytes(p.minFreeBytes, recoveryBytes)
	if !ok || usage.freeBytes < reservedFree || growth > usage.freeBytes-reservedFree {
		return false
	}

	return true
}

func (p capacityPolicy) status(pending pendingUsage, usage storageUsage) biz.QueueStatus {
	state := p.capacityState(usage)
	return biz.QueueStatus{
		State:            state,
		Writable:         state != biz.QueueBlocked,
		PendingEntries:   pending.entries,
		PendingRecords:   pending.records,
		PendingBytes:     pending.bytes,
		OldestEnqueuedAt: pending.oldestAt,
		DiskBytes:        usage.diskBytes,
		CapacityBytes:    p.capacityBytes,
		FreeBytes:        usage.freeBytes,
		MinFreeBytes:     p.minFreeBytes,
	}
}

func (p capacityPolicy) capacityState(usage storageUsage) biz.QueueState {
	minimumGrowth := max(usage.blockBytes, 1)
	if !p.admits(usage, minimumGrowth) {
		return biz.QueueBlocked
	}

	recoveryBytes, _ := p.recoveryBytes(usage)
	writeLimit := p.capacityBytes - recoveryBytes
	reservedFree, _ := addBytes(p.minFreeBytes, recoveryBytes)
	state := ratioState(usage.diskBytes, writeLimit)
	freeHeadroom := usage.freeBytes - reservedFree
	if freeHeadroom <= multiplyBytes(p.segmentBytes, 2) {
		return biz.QueueCritical
	}
	if freeHeadroom <= multiplyBytes(p.segmentBytes, 4) && state < biz.QueueWarning {
		return biz.QueueWarning
	}
	return state
}

func (p capacityPolicy) recoveryBytes(usage storageUsage) (int64, bool) {
	maxSegmentBytes, ok := addBytes(p.segmentBytes, p.segmentBytes)
	if !ok {
		return 0, false
	}
	configuredBytes, ok := allocationSize(maxSegmentBytes, usage.blockBytes)
	if !ok {
		return 0, false
	}
	// 配置缩小后，旧分段仍可能更大；截断队首前必须容得下该分段的完整临时副本。
	return max(configuredBytes, usage.largestFileBytes), true
}

func ratioState(used, limit int64) biz.QueueState {
	if used >= percentage(limit, criticalCapacityPercent) {
		return biz.QueueCritical
	}
	if used >= percentage(limit, warningCapacityPercent) {
		return biz.QueueWarning
	}
	return biz.QueueHealthy
}

func percentage(value, percent int64) int64 {
	return value/100*percent + value%100*percent/100
}

func multiplyBytes(value, factor int64) int64 {
	if value > math.MaxInt64/factor {
		return math.MaxInt64
	}
	return value * factor
}

func addBytes(left, right int64) (int64, bool) {
	if left < 0 || right < 0 || right > math.MaxInt64-left {
		return 0, false
	}
	return left + right, true
}

func allocationSize(size, blockBytes int64) (int64, bool) {
	if size < 0 {
		return 0, false
	}
	if blockBytes <= 1 || size == 0 {
		return size, true
	}
	remainder := size % blockBytes
	if remainder == 0 {
		return size, true
	}
	return addBytes(size, blockBytes-remainder)
}
