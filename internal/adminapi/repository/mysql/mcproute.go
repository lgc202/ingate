package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/types"
)

// MCPRoute 保存单行路由投影，JSON 数组保留父引用的声明顺序。
type MCPRoute struct {
	ID          uint64           `gorm:"column:id;primaryKey;autoIncrement"`
	Namespace   string           `gorm:"column:namespace"`
	Name        string           `gorm:"column:name"`
	DisplayName string           `gorm:"column:display_name"`
	Version     string           `gorm:"column:version"`
	CreatedAt   time.Time        `gorm:"column:created_at;autoCreateTime:false"`
	UpdatedAt   time.Time        `gorm:"column:updated_at;autoCreateTime;autoUpdateTime"`
	Parents     []MCPRouteParent `gorm:"column:parents;serializer:json"`
	Path        string           `gorm:"column:path"`
	BackendName string           `gorm:"column:backend_name"`
	BackendPath string           `gorm:"column:backend_path"`
}

// MCPRouteParent 保存路由 JSON 中的一个 Gateway 引用。
type MCPRouteParent struct {
	Name        string `json:"name"`
	SectionName string `json:"sectionName,omitempty"`
}

// MCPRouteListParams 限定一个命名空间内路由列表的分页和常用筛选条件。
type MCPRouteListParams struct {
	Namespace   string
	PageSize    int32
	PageNum     int32
	DisplayName string
	GatewayName string
}

// MCPRoutePage 保存当前页和应用相同筛选条件的总数。
type MCPRoutePage struct {
	Items      []MCPRoute
	TotalCount int64
}

// TableName 返回路由投影表名。
func (MCPRoute) TableName() string {
	return "mcproutes"
}

// UpsertMCPRoute 在一个事务中替换完整路由投影，不拼接跨表配置。
func (s *Store) UpsertMCPRoute(ctx context.Context, route MCPRoute) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row MCPRoute
		err := tx.Select("id").
			Where("namespace = ? AND name = ?", route.Namespace, route.Name).
			Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(&route).Error; err != nil {
				return fmt.Errorf("create MCPRoute projection: %w", err)
			}

			return nil
		}

		if err != nil {
			return fmt.Errorf("get MCPRoute projection: %w", err)
		}

		if err := tx.Model(&row).
			Select("display_name", "version", "created_at", "parents", "path", "backend_name", "backend_path").
			Updates(&route).Error; err != nil {
			return fmt.Errorf("update MCPRoute projection: %w", err)
		}

		return nil
	})
}

// DeleteMCPRoute 删除整个路由投影，不存在时成功返回。
func (s *Store) DeleteMCPRoute(ctx context.Context, namespace, name string) error {
	query := s.db.WithContext(ctx).Where("namespace = ? AND name = ?", namespace, name)
	if err := query.Delete(&MCPRoute{}).Error; err != nil {
		return fmt.Errorf("delete MCPRoute projection: %w", err)
	}

	return nil
}

// ListMCPRouteKeys 返回所有已投影对象的空间和名称，供启动对账使用。
func (s *Store) ListMCPRouteKeys(ctx context.Context) ([]types.NamespacedName, error) {
	var keys []types.NamespacedName
	query := s.db.WithContext(ctx).Model(&MCPRoute{}).
		Select("namespace", "name").Order("namespace, name")
	if err := query.Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("list MCPRoute keys: %w", err)
	}

	return keys, nil
}

// GetMCPRoute 读取完整路由投影，JSON 解码失败时不返回部分配置。
func (s *Store) GetMCPRoute(ctx context.Context, namespace, name string) (MCPRoute, error) {
	var route MCPRoute
	err := s.db.WithContext(ctx).Where("namespace = ? AND name = ?", namespace, name).Take(&route).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return MCPRoute{}, fmt.Errorf("%w: %w", ErrNotFound, err)
	}

	if err != nil {
		return MCPRoute{}, fmt.Errorf("get MCPRoute projection: %w", err)
	}

	return route, nil
}

// ListMCPRoutes 在指定命名空间的同一快照中按稳定名称分页，筛选和总数使用相同条件。
func (s *Store) ListMCPRoutes(ctx context.Context, params MCPRouteListParams) (MCPRoutePage, error) {
	var result MCPRoutePage
	err := s.readOnlyTransaction(ctx, func(tx *gorm.DB) error {
		query := filterDisplayName(
			tx.Model(&MCPRoute{}).Where("namespace = ?", params.Namespace),
			params.DisplayName,
		)
		if params.GatewayName != "" {
			query = query.Where("JSON_CONTAINS(parents, JSON_OBJECT('name', ?))", params.GatewayName)
		}

		if err := query.Count(&result.TotalCount).Error; err != nil {
			return fmt.Errorf("count MCPRoutes: %w", err)
		}

		if err := page(query.Order("name"), params.PageSize, params.PageNum).Find(&result.Items).Error; err != nil {
			return fmt.Errorf("list MCPRoutes: %w", err)
		}

		return nil
	})
	if err != nil {
		return MCPRoutePage{}, err
	}

	return result, nil
}
