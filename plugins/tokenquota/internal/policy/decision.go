package policy

const (
	unavailableStatusCode = 503
	unavailableMessage    = "Token quota is temporarily unavailable"
	unavailableErrorType  = "server_error"
	unavailableErrorCode  = "token_quota_unavailable"
	exceededErrorType     = "insufficient_quota"
	exceededErrorCode     = "insufficient_quota"
)

// Decide 按配置顺序合并全部预算池检查结果
func Decide(checks []Check, outcomes []CheckOutcome) Decision {
	for i, check := range checks {
		if i >= len(outcomes) || outcomes[i].Err != nil {
			if check.Policy.FailOpen() {
				continue
			}
			return Decision{
				StatusCode: unavailableStatusCode,
				Message:    unavailableMessage,
				ErrorType:  unavailableErrorType,
				ErrorCode:  unavailableErrorCode,
			}
		}
		if outcomes[i].Allowed {
			continue
		}
		return Decision{
			StatusCode: check.Policy.RejectedStatusCode(),
			Message:    check.Policy.RejectedMessage(),
			ErrorType:  exceededErrorType,
			ErrorCode:  exceededErrorCode,
			RetryAfter: outcomes[i].ResetSeconds,
		}
	}
	return Decision{Allowed: true}
}
