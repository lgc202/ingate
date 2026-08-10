package configuration

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/biz"
)

type resourcePageKind int

const (
	resourcePageGateway resourcePageKind = iota
	resourcePageRoute
	resourcePageUpstream
	resourcePageCertificate
	resourcePageRateLimitPolicy
	resourcePageIPRestrictionPolicy
	resourcePageCount
)

type pageCursor struct {
	Kind     resourcePageKind `json:"kind"`
	Continue string           `json:"continue,omitempty"`
}

// ListItems 按资源类型和 API Server Continue Token 返回一页配置状态
func (u *Usecase) ListItems(ctx context.Context, page biz.PageRequest) (biz.PageResult[Item], error) {
	cursor, err := decodePageCursor(page.Cursor)
	if err != nil {
		return biz.PageResult[Item]{}, err
	}

	items := make([]Item, 0, page.Limit)
	for cursor.Kind < resourcePageCount && int64(len(items)) < page.Limit {
		current, err := u.listItemPage(ctx, cursor.Kind, biz.PageRequest{
			Limit:  page.Limit - int64(len(items)),
			Cursor: cursor.Continue,
		})
		if err != nil {
			return biz.PageResult[Item]{}, err
		}
		items = append(items, current.Items...)
		if current.NextCursor != "" {
			cursor.Continue = current.NextCursor
			return biz.PageResult[Item]{Items: items, NextCursor: encodePageCursor(cursor)}, nil
		}
		cursor.Kind++
		cursor.Continue = ""
	}

	nextCursor := ""
	if cursor.Kind < resourcePageCount {
		nextCursor = encodePageCursor(cursor)
	}
	return biz.PageResult[Item]{Items: items, NextCursor: nextCursor}, nil
}

func (u *Usecase) listItemPage(
	ctx context.Context,
	kind resourcePageKind,
	page biz.PageRequest,
) (biz.PageResult[Item], error) {
	switch kind {
	case resourcePageGateway:
		return mapPage(ctx, page, u.gateways.ListPage, gatewayItem)
	case resourcePageRoute:
		return mapPage(ctx, page, u.routes.ListPage, routeItem)
	case resourcePageUpstream:
		return mapPage(ctx, page, u.upstreams.ListPage, upstreamItem)
	case resourcePageCertificate:
		return mapPage(ctx, page, u.certificates.ListPage, certificateItem)
	case resourcePageRateLimitPolicy:
		return mapPage(ctx, page, u.rateLimitPolicies.ListPage, rateLimitPolicyItem)
	case resourcePageIPRestrictionPolicy:
		return mapPage(ctx, page, u.ipRestrictionPolicies.ListPage, ipRestrictionPolicyItem)
	default:
		return biz.PageResult[Item]{}, fmt.Errorf("%w: unknown resource page", biz.ErrInvalidCursor)
	}
}

func mapPage[T any](
	ctx context.Context,
	page biz.PageRequest,
	list func(context.Context, biz.PageRequest) (biz.PageResult[T], error),
	toItem func(T) Item,
) (biz.PageResult[Item], error) {
	result, err := list(ctx, page)
	if err != nil {
		return biz.PageResult[Item]{}, err
	}
	items := make([]Item, 0, len(result.Items))
	for _, current := range result.Items {
		items = append(items, toItem(current))
	}
	return biz.PageResult[Item]{Items: items, NextCursor: result.NextCursor}, nil
}

func decodePageCursor(value string) (pageCursor, error) {
	if value == "" {
		return pageCursor{Kind: resourcePageGateway}, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return pageCursor{}, fmt.Errorf("%w: decode configuration cursor", biz.ErrInvalidCursor)
	}
	var cursor pageCursor
	if err := json.Unmarshal(data, &cursor); err != nil || cursor.Kind < resourcePageGateway || cursor.Kind >= resourcePageCount {
		return pageCursor{}, fmt.Errorf("%w: decode configuration cursor", biz.ErrInvalidCursor)
	}
	return cursor, nil
}

func encodePageCursor(cursor pageCursor) string {
	data, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(data)
}
