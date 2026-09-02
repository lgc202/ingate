package policy

// Page 保存一页策略及其目标展示名称。
type Page[P any] struct {
	Items       []P
	TargetNames PolicyTargetNames
	NextCursor  string
}

// View 保存单条策略及其目标展示名称。
type View[P any] struct {
	Policy      *P
	TargetNames PolicyTargetNames
}
