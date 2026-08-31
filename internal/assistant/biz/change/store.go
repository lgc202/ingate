package change

import "context"

// Store 定义审批请求链路使用的持久化边界。
type Store interface {
	ListProposedChanges(
		ctx context.Context,
		actorID string,
		conversationID string,
	) ([]ProposedChange, error)
	ResumeProposedChange(
		ctx context.Context,
		actorID string,
		changeID string,
		approved bool,
		feedback string,
	) (ProposedChange, error)
}

// ProposalStore 定义获批工具返回后收敛变更结果所需的窄持久化边界。
type ProposalStore interface {
	CompleteProposedChange(
		ctx context.Context,
		executionID string,
		workerID string,
		changeID string,
		state State,
		resource CreatedResource,
		errorCode FailureCode,
	) error
}
