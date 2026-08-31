package change

import "context"

// Usecase 管理配置变更的查询，并把审批结果交回原 Eino 执行。
type Usecase struct {
	store Store
}

// NewUsecase 创建配置变更用例。
func NewUsecase(store Store) *Usecase {
	return &Usecase{store: store}
}

// List 返回当前管理员在指定会话中可以看到的配置变更。
func (uc *Usecase) List(
	ctx context.Context,
	actorID string,
	conversationID string,
) ([]ProposedChange, error) {
	return uc.store.ListProposedChanges(ctx, actorID, conversationID)
}

// Approve 批准当前配置，并让后台 Worker 从原 checkpoint 继续执行。
func (uc *Usecase) Approve(
	ctx context.Context,
	actorID string,
	changeID string,
) (ProposedChange, error) {
	return uc.store.ResumeProposedChange(ctx, actorID, changeID, true, "")
}

// Reject 拒绝当前配置，并让 Agent 在原上下文中结束本次请求。
func (uc *Usecase) Reject(
	ctx context.Context,
	actorID string,
	changeID string,
) (ProposedChange, error) {
	return uc.store.ResumeProposedChange(ctx, actorID, changeID, false, "")
}

// Revise 把文字作为拒绝原因恢复原中断，使 Agent 根据反馈重新决定后续工具调用。
func (uc *Usecase) Revise(
	ctx context.Context,
	actorID string,
	changeID string,
	feedback string,
) (ProposedChange, error) {
	return uc.store.ResumeProposedChange(ctx, actorID, changeID, false, feedback)
}
