package configuration

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/lgc202/ingate/internal/adminapi/biz"
)

const summaryPageSize = 200

// GetSummary 分页扫描全部声明式资源并返回状态汇总
func (u *Usecase) GetSummary(ctx context.Context) (Summary, error) {
	var summaries [resourcePageCount]Summary
	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return collectSummary(ctx, u.gateways.ListPage, gatewayItem, &summaries[resourcePageGateway])
	})
	group.Go(func() error {
		return collectSummary(ctx, u.routes.ListPage, routeItem, &summaries[resourcePageRoute])
	})
	group.Go(func() error {
		return collectSummary(ctx, u.upstreams.ListPage, upstreamItem, &summaries[resourcePageUpstream])
	})
	group.Go(func() error {
		return collectSummary(ctx, u.certificates.ListPage, certificateItem, &summaries[resourcePageCertificate])
	})
	group.Go(func() error {
		return collectSummary(ctx, u.rateLimitPolicies.ListPage, rateLimitPolicyItem, &summaries[resourcePageRateLimitPolicy])
	})
	group.Go(func() error {
		return collectSummary(ctx, u.ipRestrictionPolicies.ListPage, ipRestrictionPolicyItem, &summaries[resourcePageIPRestrictionPolicy])
	})
	group.Go(func() error {
		return collectSummary(ctx, u.tokenQuotaPolicies.ListPage, tokenQuotaPolicyItem, &summaries[resourcePageTokenQuotaPolicy])
	})
	if err := group.Wait(); err != nil {
		return Summary{}, fmt.Errorf("collect configuration summary: %w", err)
	}

	var summary Summary
	for _, current := range summaries {
		summary.merge(current)
	}
	return summary, nil
}

func collectSummary[T any](
	ctx context.Context,
	list func(context.Context, biz.PageRequest) (biz.PageResult[T], error),
	toItem func(T) Item,
	summary *Summary,
) error {
	page := biz.PageRequest{Limit: summaryPageSize}
	for {
		result, err := list(ctx, page)
		if err != nil {
			return err
		}
		for _, current := range result.Items {
			summary.add(toItem(current).Status)
		}
		if result.NextCursor == "" {
			return nil
		}
		page.Cursor = result.NextCursor
	}
}

func (s *Summary) add(status biz.ResourceStatus) {
	s.Total++
	switch status.State {
	case biz.ResourceStateReady:
		s.Ready++
	case biz.ResourceStatePending:
		s.Pending++
	case biz.ResourceStateError:
		s.Error++
	case biz.ResourceStateDisabled:
		s.Disabled++
	}
}

func (s *Summary) merge(other Summary) {
	s.Total += other.Total
	s.Ready += other.Ready
	s.Pending += other.Pending
	s.Error += other.Error
	s.Disabled += other.Disabled
}

func displayName(name, id string) string {
	if name != "" {
		return name
	}
	return id
}
