package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/types"
)

// ModelRoute 保存单行路由投影，JSON 数组保留配置的声明顺序。
type ModelRoute struct {
	ID          uint64             `gorm:"column:id;primaryKey;autoIncrement"`
	Namespace   string             `gorm:"column:namespace"`
	Name        string             `gorm:"column:name"`
	DisplayName string             `gorm:"column:display_name"`
	Version     string             `gorm:"column:version"`
	CreatedAt   time.Time          `gorm:"column:created_at;autoCreateTime:false"`
	UpdatedAt   time.Time          `gorm:"column:updated_at;autoCreateTime;autoUpdateTime"`
	Parents     []ModelRouteParent `gorm:"column:parents;serializer:json"`
	Rules       []ModelRouteRule   `gorm:"column:rules;serializer:json"`
}

// ModelRouteParent 保存路由 JSON 中的一个 Gateway 引用。
type ModelRouteParent struct {
	Name        string `json:"name"`
	SectionName string `json:"sectionName,omitempty"`
}

// ModelRouteRule 保存有序规则配置，Name 不决定匹配优先级。
type ModelRouteRule struct {
	Name        string                 `json:"name,omitempty"`
	Matches     []ModelRouteMatch      `json:"matches"`
	BackendRefs []ModelRouteBackendRef `json:"backendRefs"`
}

// ModelRouteMatch 保存 AND 关系的模型 Header 条件；空条件表示所有模型。
type ModelRouteMatch struct {
	Headers []ModelRouteHeaderMatch `json:"headers,omitempty"`
}

// ModelRouteHeaderMatch 保存模型名称的匹配条件，不改写名称大小写或值。
type ModelRouteHeaderMatch struct {
	Name  string `json:"name"`
	Type  string `json:"type,omitempty"`
	Value string `json:"value"`
}

// ModelRouteBackendRef 保存 AIBackend 引用及可选的出站模型名覆盖。
type ModelRouteBackendRef struct {
	Name              string `json:"name"`
	ModelNameOverride string `json:"modelNameOverride,omitempty"`
}

// ModelRouteListParams 限定一个命名空间内路由列表的分页和常用筛选条件。
type ModelRouteListParams struct {
	Namespace   string
	PageSize    int32
	PageNum     int32
	DisplayName string
	GatewayName string
}

// ModelRoutePage 保存当前页和应用相同筛选条件的总数。
type ModelRoutePage struct {
	Items      []ModelRoute
	TotalCount int64
}

// TableName 返回路由投影表名。
func (ModelRoute) TableName() string {
	return "modelroutes"
}

// UpsertModelRoute 在一个事务中替换完整路由投影，不拼接跨表配置。
func (s *Store) UpsertModelRoute(ctx context.Context, route ModelRoute) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row ModelRoute
		err := tx.Select("id").
			Where("namespace = ? AND name = ?", route.Namespace, route.Name).
			Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(&route).Error; err != nil {
				return fmt.Errorf("create ModelRoute projection: %w", err)
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("get ModelRoute projection: %w", err)
		}

		if err := tx.Model(&row).
			Select("display_name", "version", "created_at", "parents", "rules").
			Updates(&route).Error; err != nil {
			return fmt.Errorf("update ModelRoute projection: %w", err)
		}

		return nil
	})
}

// DeleteModelRoute 删除整个路由投影，不存在时成功返回。
func (s *Store) DeleteModelRoute(ctx context.Context, namespace, name string) error {
	query := s.db.WithContext(ctx).Where("namespace = ? AND name = ?", namespace, name)
	if err := query.Delete(&ModelRoute{}).Error; err != nil {
		return fmt.Errorf("delete ModelRoute projection: %w", err)
	}

	return nil
}

// ListModelRouteKeys 返回所有已投影对象的空间和名称，供启动对账使用。
func (s *Store) ListModelRouteKeys(ctx context.Context) ([]types.NamespacedName, error) {
	var keys []types.NamespacedName
	query := s.db.WithContext(ctx).Model(&ModelRoute{}).
		Select("namespace", "name").Order("namespace, name")
	if err := query.Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("list ModelRoute keys: %w", err)
	}
	return keys, nil
}

// GetModelRoute 读取完整路由投影，JSON 解码失败时不返回部分配置。
func (s *Store) GetModelRoute(ctx context.Context, namespace, name string) (ModelRoute, error) {
	var route ModelRoute
	err := s.db.WithContext(ctx).Where("namespace = ? AND name = ?", namespace, name).Take(&route).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ModelRoute{}, fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	if err != nil {
		return ModelRoute{}, fmt.Errorf("get ModelRoute projection: %w", err)
	}

	return route, nil
}

// ListModelRoutes 在指定命名空间的同一快照中按稳定名称分页，筛选和总数使用相同条件。
func (s *Store) ListModelRoutes(ctx context.Context, params ModelRouteListParams) (ModelRoutePage, error) {
	var result ModelRoutePage
	err := s.readOnlyTransaction(ctx, func(tx *gorm.DB) error {
		query := filterDisplayName(
			tx.Model(&ModelRoute{}).Where("namespace = ?", params.Namespace),
			params.DisplayName,
		)
		if params.GatewayName != "" {
			query = query.Where("JSON_CONTAINS(parents, JSON_OBJECT('name', ?))", params.GatewayName)
		}

		if err := query.Count(&result.TotalCount).Error; err != nil {
			return fmt.Errorf("count ModelRoutes: %w", err)
		}

		if err := page(query.Order("name"), params.PageSize, params.PageNum).Find(&result.Items).Error; err != nil {
			return fmt.Errorf("list ModelRoutes: %w", err)
		}

		return nil
	})
	if err != nil {
		return ModelRoutePage{}, err
	}

	return result, nil
}
