package configuration

import (
	"cmp"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func summarize(items []Item) Summary {
	summary := Summary{Total: len(items)}
	for _, item := range items {
		switch item.Status.State {
		case biz.ResourceStateReady:
			summary.Ready++
		case biz.ResourceStatePending:
			summary.Pending++
		case biz.ResourceStateError:
			summary.Error++
		case biz.ResourceStateDisabled:
			summary.Disabled++
		}
	}
	return summary
}

func compareItems(a, b Item) int {
	if result := biz.CompareResourceState(a.Status.State, b.Status.State); result != 0 {
		return result
	}
	if result := cmp.Compare(kindPriority(a.Kind), kindPriority(b.Kind)); result != 0 {
		return result
	}
	if result := cmp.Compare(a.Name, b.Name); result != 0 {
		return result
	}
	return cmp.Compare(a.ID, b.ID)
}

func kindPriority(kind resource.Kind) int {
	switch kind {
	case resource.KindGateway:
		return kindPriorityGateway
	case resource.KindRoute:
		return kindPriorityRoute
	case resource.KindUpstream:
		return kindPriorityUpstream
	case resource.KindCertificate:
		return kindPriorityCertificate
	case resource.KindRateLimitPolicy:
		return kindPriorityRateLimitPolicy
	case resource.KindAccessControlPolicy:
		return kindPriorityAccessControlPolicy
	case resource.KindTokenQuotaPolicy:
		return kindPriorityTokenQuotaPolicy
	default:
		return kindPriorityUnknown
	}
}

func displayName(name, id string) string {
	if name != "" {
		return name
	}
	return id
}
