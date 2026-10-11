package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// HTTPRoute 保存单行路由投影，JSON 数组保留配置的声明顺序。
type HTTPRoute struct {
	ID          uint64            `gorm:"column:id;primaryKey;autoIncrement"`
	Name        string            `gorm:"column:name"`
	DisplayName string            `gorm:"column:display_name"`
	Version     string            `gorm:"column:version"`
	CreatedAt   time.Time         `gorm:"column:created_at;autoCreateTime:false"`
	UpdatedAt   time.Time         `gorm:"column:updated_at;autoCreateTime;autoUpdateTime"`
	Parents     []HTTPRouteParent `gorm:"column:parents;serializer:json"`
	Rules       []HTTPRouteRule   `gorm:"column:rules;serializer:json"`
}

// HTTPRouteParent 保存路由 JSON 中的一个 Gateway 引用。
type HTTPRouteParent struct {
	Name        string `json:"name"`
	SectionName string `json:"sectionName,omitempty"`
}

// HTTPRouteRule 保存有序规则配置，Name 不决定匹配优先级。
type HTTPRouteRule struct {
	Name        string                `json:"name,omitempty"`
	Matches     []HTTPRouteMatch      `json:"matches"`
	BackendRefs []HTTPRouteBackendRef `json:"backendRefs"`
}

// HTTPRouteMatch 保存 API Server 已展开缺省值的路径条件。
type HTTPRouteMatch struct {
	PathType  string `json:"pathType"`
	PathValue string `json:"pathValue"`
}

// HTTPRouteBackendRef 保存规则引用的稳定 Backend 名称。
type HTTPRouteBackendRef struct {
	Name string `json:"name"`
}

// HTTPRouteListParams 限定路由列表的分页和常用筛选条件。
type HTTPRouteListParams struct {
	PageSize    int32
	PageNum     int32
	DisplayName string
	GatewayName string
}

// HTTPRoutePage 保存当前页和应用相同筛选条件的总数。
type HTTPRoutePage struct {
	Items      []HTTPRoute
	TotalCount int64
}

// TableName 返回路由投影表名。
func (HTTPRoute) TableName() string {
	return "httproutes"
}

// UpsertHTTPRoute 在一个事务中替换完整路由投影，不拼接跨表配置。
func (s *Store) UpsertHTTPRoute(ctx context.Context, route HTTPRoute) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row HTTPRoute
		err := tx.Select("id").Where("name = ?", route.Name).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(&route).Error; err != nil {
				return fmt.Errorf("create HTTPRoute projection: %w", err)
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("get HTTPRoute projection: %w", err)
		}

		if err := tx.Model(&row).
			Select("display_name", "version", "created_at", "parents", "rules").
			Updates(&route).Error; err != nil {
			return fmt.Errorf("update HTTPRoute projection: %w", err)
		}

		return nil
	})
}

// DeleteHTTPRoute 删除整个路由投影，不存在时成功返回。
func (s *Store) DeleteHTTPRoute(ctx context.Context, name string) error {
	if err := s.db.WithContext(ctx).Where("name = ?", name).Delete(&HTTPRoute{}).Error; err != nil {
		return fmt.Errorf("delete HTTPRoute projection: %w", err)
	}

	return nil
}

// ListHTTPRouteNames 返回已投影的稳定名称，供启动对账使用。
func (s *Store) ListHTTPRouteNames(ctx context.Context) ([]string, error) {
	var names []string
	if err := s.db.WithContext(ctx).Model(&HTTPRoute{}).Order("name").Pluck("name", &names).Error; err != nil {
		return nil, fmt.Errorf("list HTTPRoute names: %w", err)
	}

	return names, nil
}

// GetHTTPRoute 读取完整路由投影，JSON 解码失败时不返回部分配置。
func (s *Store) GetHTTPRoute(ctx context.Context, name string) (HTTPRoute, error) {
	var route HTTPRoute
	err := s.db.WithContext(ctx).Where("name = ?", name).Take(&route).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return HTTPRoute{}, fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	if err != nil {
		return HTTPRoute{}, fmt.Errorf("get HTTPRoute projection: %w", err)
	}

	return route, nil
}

// ListHTTPRoutes 在同一快照中按稳定名称分页，筛选和总数使用相同条件。
func (s *Store) ListHTTPRoutes(ctx context.Context, params HTTPRouteListParams) (HTTPRoutePage, error) {
	var result HTTPRoutePage
	err := s.readOnlyTransaction(ctx, func(tx *gorm.DB) error {
		query := filterDisplayName(tx.Model(&HTTPRoute{}), params.DisplayName)
		if params.GatewayName != "" {
			query = query.Where("JSON_CONTAINS(parents, JSON_OBJECT('name', ?))", params.GatewayName)
		}

		if err := query.Count(&result.TotalCount).Error; err != nil {
			return fmt.Errorf("count HTTPRoutes: %w", err)
		}
		if err := page(query.Order("name"), params.PageSize, params.PageNum).Find(&result.Items).Error; err != nil {
			return fmt.Errorf("list HTTPRoutes: %w", err)
		}

		return nil
	})
	if err != nil {
		return HTTPRoutePage{}, err
	}

	return result, nil
}
