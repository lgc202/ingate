package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// AIBackend 保存模型服务的单行查询投影，不保存凭据或资源状态。
type AIBackend struct {
	ID           uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	Name         string    `gorm:"column:name"`
	DisplayName  string    `gorm:"column:display_name"`
	BackendName  string    `gorm:"column:backend_name"`
	SchemaName   string    `gorm:"column:schema_name"`
	SchemaPrefix string    `gorm:"column:schema_prefix"`
	Version      string    `gorm:"column:version"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime:false"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoCreateTime;autoUpdateTime"`
}

// AIBackendPage 保存当前页和应用相同筛选条件的总数。
type AIBackendPage struct {
	Items      []AIBackend
	TotalCount int64
}

// TableName 返回模型服务投影表名。
func (AIBackend) TableName() string {
	return "aibackends"
}

// UpsertAIBackend 在一个事务中替换完整模型服务投影，包括空展示名与路径前缀。
func (s *Store) UpsertAIBackend(ctx context.Context, backend AIBackend) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row AIBackend
		err := tx.Select("id").Where("name = ?", backend.Name).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(&backend).Error; err != nil {
				return fmt.Errorf("create AIBackend projection: %w", err)
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("get AIBackend projection: %w", err)
		}

		if err := tx.Model(&row).
			Select("display_name", "backend_name", "schema_name", "schema_prefix", "version", "created_at").
			Updates(&backend).Error; err != nil {
			return fmt.Errorf("update AIBackend projection: %w", err)
		}

		return nil
	})
}

// DeleteAIBackend 删除整个模型服务投影，不存在时成功返回。
func (s *Store) DeleteAIBackend(ctx context.Context, name string) error {
	if err := s.db.WithContext(ctx).Where("name = ?", name).Delete(&AIBackend{}).Error; err != nil {
		return fmt.Errorf("delete AIBackend projection: %w", err)
	}

	return nil
}

// ListAIBackendNames 返回已投影的稳定名称，供启动对账使用。
func (s *Store) ListAIBackendNames(ctx context.Context) ([]string, error) {
	var names []string
	if err := s.db.WithContext(ctx).Model(&AIBackend{}).Order("name").Pluck("name", &names).Error; err != nil {
		return nil, fmt.Errorf("list AIBackend names: %w", err)
	}

	return names, nil
}

// GetAIBackend 读取完整模型服务投影。
func (s *Store) GetAIBackend(ctx context.Context, name string) (AIBackend, error) {
	var backend AIBackend
	err := s.db.WithContext(ctx).Where("name = ?", name).Take(&backend).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AIBackend{}, fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	if err != nil {
		return AIBackend{}, fmt.Errorf("get AIBackend projection: %w", err)
	}

	return backend, nil
}

// ListAIBackends 在同一快照中按稳定名称分页，筛选和总数使用相同条件。
func (s *Store) ListAIBackends(ctx context.Context, params ResourceListParams) (AIBackendPage, error) {
	var result AIBackendPage
	err := s.readOnlyTransaction(ctx, func(tx *gorm.DB) error {
		query := filterDisplayName(tx.Model(&AIBackend{}), params.DisplayName)

		if err := query.Count(&result.TotalCount).Error; err != nil {
			return fmt.Errorf("count AIBackends: %w", err)
		}

		if err := page(query.Order("name"), params.PageSize, params.PageNum).Find(&result.Items).Error; err != nil {
			return fmt.Errorf("list AIBackends: %w", err)
		}

		return nil
	})
	if err != nil {
		return AIBackendPage{}, err
	}

	return result, nil
}
