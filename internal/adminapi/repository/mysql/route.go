package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Route 对应 MySQL routes 表中的一条路由。
type Route struct {
	ID              uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	Name            string    `gorm:"column:name"`
	GatewayName     string    `gorm:"column:gateway_name"`
	ListenerName    string    `gorm:"column:listener_name"`
	PathType        string    `gorm:"column:path_type"`
	PathValue       string    `gorm:"column:path_value"`
	UpstreamName    string    `gorm:"column:upstream_name"`
	ResourceVersion string    `gorm:"column:resource_version"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime:false"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoCreateTime;autoUpdateTime"`
}

// TableName 返回 Route 使用的数据库表名。
func (Route) TableName() string { return "routes" }

// RoutePage 保存一次列表查询的结果和总数。
type RoutePage struct {
	Items      []Route
	TotalCount int64
}

// UpsertRoute 按名称创建或更新 Route。
func (s *Store) UpsertRoute(ctx context.Context, route Route) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row Route
		err := tx.Select("id").Where("name = ?", route.Name).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(&route).Error; err != nil {
				return fmt.Errorf("create route: %w", err)
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("get route: %w", err)
		}

		updates := map[string]any{
			"gateway_name":     route.GatewayName,
			"listener_name":    route.ListenerName,
			"path_type":        route.PathType,
			"path_value":       route.PathValue,
			"upstream_name":    route.UpstreamName,
			"resource_version": route.ResourceVersion,
			"created_at":       route.CreatedAt,
		}
		if err := tx.Model(&row).Updates(updates).Error; err != nil {
			return fmt.Errorf("update route: %w", err)
		}
		return nil
	})
}

// DeleteRoute 删除指定 Route；不存在时不报错。
func (s *Store) DeleteRoute(ctx context.Context, name string) error {
	if err := s.db.WithContext(ctx).Where("name = ?", name).Delete(&Route{}).Error; err != nil {
		return fmt.Errorf("delete route: %w", err)
	}
	return nil
}

// ListRouteNames 返回所有已投影 Route 的名称，供启动对账使用。
func (s *Store) ListRouteNames(ctx context.Context) ([]string, error) {
	var names []string
	query := s.db.WithContext(ctx).Model(&Route{}).Order("name")
	if err := query.Pluck("name", &names).Error; err != nil {
		return nil, fmt.Errorf("list route names: %w", err)
	}
	return names, nil
}

// GetRoute 按名称读取 Route。
func (s *Store) GetRoute(ctx context.Context, name string) (Route, error) {
	var route Route
	err := s.db.WithContext(ctx).Where("name = ?", name).Take(&route).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Route{}, fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	if err != nil {
		return Route{}, fmt.Errorf("get route: %w", err)
	}

	return route, nil
}

// ListRoutes 按名称分页读取 Route，总数和列表来自同一读事务。
func (s *Store) ListRoutes(ctx context.Context, size, number int32) (RoutePage, error) {
	var result RoutePage
	err := s.readOnlyTransaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Model(&Route{}).Count(&result.TotalCount).Error; err != nil {
			return fmt.Errorf("count routes: %w", err)
		}

		routeQuery := page(tx.Model(&Route{}).Order("name"), size, number)
		if err := routeQuery.Find(&result.Items).Error; err != nil {
			return fmt.Errorf("list routes: %w", err)
		}

		return nil
	})
	if err != nil {
		return RoutePage{}, err
	}

	return result, nil
}
