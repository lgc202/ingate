package tool

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	defaultObservationHours = 24
	maxObservationHours     = 24 * 7
)

type invalidInputError struct {
	reason string
}

func (e *invalidInputError) Error() string {
	return e.reason
}

func invalidInputf(format string, args ...any) error {
	return &invalidInputError{reason: fmt.Sprintf(format, args...)}
}

// invalidInputReason 识别模型可以通过重新调用工具修正的参数错误。
// 这类错误属于 Agent 循环的正常反馈，不能与网络、存储等系统故障混在一起。
func invalidInputReason(err error) (string, bool) {
	var inputErr *invalidInputError
	if !errors.As(err, &inputErr) {
		return "", false
	}
	return inputErr.Error(), true
}

// recoverableToolError 把模型能够自行修正的错误转换为工具结果。
// 网络、存储和协议故障不在这里降级，仍交给执行层终止任务并记录真实原因。
func recoverableToolError(err error) (summary, status string, ok bool) {
	if reason, matched := invalidInputReason(err); matched {
		return reason, "invalid_input", true
	}
	if errors.Is(err, ErrQueryTargetNotFound) {
		return "the requested resource or retained request record is no longer available; refresh the corresponding list before continuing and do not retry the same identifier", "not_found", true
	}
	return "", "", false
}

func normalizeResourceScope(scopeType, scopeID string) (string, string, error) {
	scopeType = strings.ToLower(strings.TrimSpace(scopeType))
	scopeID = strings.TrimSpace(scopeID)
	if scopeType == "" {
		scopeType = "all"
	}
	if scopeType == "all" {
		if scopeID != "" {
			return "", "", invalidInputf("scope_id must be omitted when scope_type is all")
		}
		return scopeType, scopeID, nil
	}
	if scopeType != "gateway" && scopeType != "route" && scopeType != "service" {
		return "", "", invalidInputf(
			"unsupported scope_type %q; use all, gateway, route, or service",
			scopeType,
		)
	}
	if scopeID == "" {
		return "", "", invalidInputf(
			"scope_id is required when scope_type is %s; use the resource_id returned by analyze_traffic or list_%ss",
			scopeType,
			scopeType,
		)
	}
	return scopeType, scopeID, nil
}

// observationTimeRange 统一观测工具的时间范围语义，使流量排名与失败样本能够复用同一组起止时间。
// 显式时间范围使用 UTC 解析和传递，避免 Assistant 所在节点的时区改变查询结果。
func observationTimeRange(hours int32, startValue, endValue string) (time.Time, time.Time, error) {
	startValue = strings.TrimSpace(startValue)
	endValue = strings.TrimSpace(endValue)
	if startValue == "" && endValue == "" {
		if hours < 0 || hours > maxObservationHours {
			return time.Time{}, time.Time{}, invalidInputf(
				"hours must be omitted or between 1 and %d",
				maxObservationHours,
			)
		}
		if hours == 0 {
			hours = defaultObservationHours
		}
		endTime := time.Now().UTC()
		return endTime.Add(-time.Duration(hours) * time.Hour), endTime, nil
	}
	if startValue == "" || endValue == "" {
		return time.Time{}, time.Time{}, invalidInputf("start_time and end_time must be provided together")
	}
	if hours != 0 {
		return time.Time{}, time.Time{}, invalidInputf(
			"hours cannot be combined with start_time and end_time",
		)
	}

	startTime, err := time.Parse(time.RFC3339, startValue)
	if err != nil {
		return time.Time{}, time.Time{}, invalidInputf("start_time must use RFC3339 format")
	}
	endTime, err := time.Parse(time.RFC3339, endValue)
	if err != nil {
		return time.Time{}, time.Time{}, invalidInputf("end_time must use RFC3339 format")
	}
	if !startTime.Before(endTime) {
		return time.Time{}, time.Time{}, invalidInputf("start_time must be earlier than end_time")
	}
	if endTime.Sub(startTime) > maxObservationHours*time.Hour {
		return time.Time{}, time.Time{}, invalidInputf(
			"time range cannot exceed %d hours",
			maxObservationHours,
		)
	}
	return startTime, endTime, nil
}
