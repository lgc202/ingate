package adminapi

import adminv1 "github.com/lgc202/ingate/api/admin/v1"

func resourceState(state adminv1.ResourceState) string {
	switch state {
	case adminv1.ResourceState_DISABLED:
		return "disabled"
	case adminv1.ResourceState_PENDING:
		return "pending"
	case adminv1.ResourceState_READY:
		return "ready"
	case adminv1.ResourceState_ERROR:
		return "error"
	default:
		return "unknown"
	}
}

func validResourceState(state adminv1.ResourceState) bool {
	switch state {
	case adminv1.ResourceState_DISABLED,
		adminv1.ResourceState_PENDING,
		adminv1.ResourceState_READY,
		adminv1.ResourceState_ERROR:
		return true
	default:
		return false
	}
}
