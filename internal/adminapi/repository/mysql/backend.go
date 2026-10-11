package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Backend 对应 MySQL backends 表及其 Endpoint 集合。
type Backend struct {
	ID          uint64            `gorm:"column:id;primaryKey;autoIncrement"`
	Name        string            `gorm:"column:name"`
	DisplayName string            `gorm:"column:display_name"`
	Version     string            `gorm:"column:version"`
	CreatedAt   time.Time         `gorm:"column:created_at;autoCreateTime:false"`
	UpdatedAt   time.Time         `gorm:"column:updated_at;autoCreateTime;autoUpdateTime"`
	Endpoints   []BackendEndpoint `gorm:"-"`
}

// BackendEndpoint 对应 MySQL backend_endpoints 表中的一个后端端点。
type BackendEndpoint struct {
	ID        uint64 `gorm:"column:id;primaryKey;autoIncrement"`
	BackendID uint64 `gorm:"column:backend_id"`
	Address   string `gorm:"column:address"`
	Port      uint16 `gorm:"column:port"`
	Weight    int32  `gorm:"column:weight"`
}

// BackendPage 保存一次列表查询的结果和总数。
type BackendPage struct {
	Items      []Backend
	TotalCount int64
}

// TableName 返回 Backend 使用的数据库表名。
func (Backend) TableName() string { return "backends" }

// TableName 返回 BackendEndpoint 使用的数据库表名。
func (BackendEndpoint) TableName() string { return "backend_endpoints" }

// UpsertBackend 在一个事务内更新 Backend 及其 Endpoint 集合。
func (s *Store) UpsertBackend(ctx context.Context, backend Backend) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		id, err := upsertBackendRow(tx, backend)
		if err != nil {
			return err
		}

		if err := tx.Where("backend_id = ?", id).Delete(&BackendEndpoint{}).Error; err != nil {
			return fmt.Errorf("delete backend endpoints: %w", err)
		}

		endpoints := make([]BackendEndpoint, len(backend.Endpoints))
		for i, endpoint := range backend.Endpoints {
			endpoints[i] = BackendEndpoint{
				BackendID: id,
				Address:   endpoint.Address,
				Port:      endpoint.Port,
				Weight:    endpoint.Weight,
			}
		}

		if len(endpoints) == 0 {
			return nil
		}

		if err := tx.Create(&endpoints).Error; err != nil {
			return fmt.Errorf("insert backend endpoints: %w", err)
		}
		return nil
	})
}

// DeleteBackend 删除 Backend 及其 Endpoint；不存在时不报错。
func (s *Store) DeleteBackend(ctx context.Context, name string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var backend Backend
		err := tx.Select("id").Where("name = ?", name).Take(&backend).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("get backend: %w", err)
		}

		if err := tx.Where("backend_id = ?", backend.ID).Delete(&BackendEndpoint{}).Error; err != nil {
			return fmt.Errorf("delete backend endpoints: %w", err)
		}

		if err := tx.Delete(&backend).Error; err != nil {
			return fmt.Errorf("delete backend: %w", err)
		}
		return nil
	})
}

// ListBackendNames 返回所有已投影 Backend 的名称，供启动对账使用。
func (s *Store) ListBackendNames(ctx context.Context) ([]string, error) {
	var names []string
	query := s.db.WithContext(ctx).Model(&Backend{}).Order("name")
	if err := query.Pluck("name", &names).Error; err != nil {
		return nil, fmt.Errorf("list backend names: %w", err)
	}
	return names, nil
}

// GetBackend 在同一个只读事务中读取 Backend 和 Endpoint。
func (s *Store) GetBackend(ctx context.Context, name string) (Backend, error) {
	var backend Backend
	err := s.readOnlyTransaction(ctx, func(tx *gorm.DB) error {
		err := tx.Where("name = ?", name).Take(&backend).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: %w", ErrNotFound, err)
		}
		if err != nil {
			return fmt.Errorf("get backend: %w", err)
		}

		endpointQuery := tx.Where("backend_id = ?", backend.ID).Order("address, port")
		if err := endpointQuery.Find(&backend.Endpoints).Error; err != nil {
			return fmt.Errorf("get backend endpoints: %w", err)
		}

		return nil
	})
	if err != nil {
		return Backend{}, err
	}

	return backend, nil
}

// ListBackends 按名称分页读取 Backend 与 Endpoint。
func (s *Store) ListBackends(ctx context.Context, params ResourceListParams) (BackendPage, error) {
	var result BackendPage
	err := s.readOnlyTransaction(ctx, func(tx *gorm.DB) error {
		query := filterDisplayName(tx.Model(&Backend{}), params.DisplayName)
		if err := query.Count(&result.TotalCount).Error; err != nil {
			return fmt.Errorf("count backends: %w", err)
		}

		backendQuery := page(query.Order("name"), params.PageSize, params.PageNum)
		if err := backendQuery.Find(&result.Items).Error; err != nil {
			return fmt.Errorf("list backends: %w", err)
		}
		if len(result.Items) == 0 {
			return nil
		}

		ids := make([]uint64, len(result.Items))
		indexes := make(map[uint64]int, len(result.Items))
		for i, backend := range result.Items {
			ids[i] = backend.ID
			indexes[backend.ID] = i
		}

		var endpoints []BackendEndpoint
		endpointQuery := tx.Where("backend_id IN ?", ids).Order("backend_id, address, port")
		if err := endpointQuery.Find(&endpoints).Error; err != nil {
			return fmt.Errorf("list backend endpoints: %w", err)
		}

		for _, endpoint := range endpoints {
			i := indexes[endpoint.BackendID]
			result.Items[i].Endpoints = append(result.Items[i].Endpoints, endpoint)
		}

		return nil
	})
	if err != nil {
		return BackendPage{}, err
	}

	return result, nil
}

func upsertBackendRow(tx *gorm.DB, backend Backend) (uint64, error) {
	var row Backend
	err := tx.Select("id").Where("name = ?", backend.Name).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = Backend{
			Name:        backend.Name,
			DisplayName: backend.DisplayName,
			Version:     backend.Version,
			CreatedAt:   backend.CreatedAt,
		}
		if err := tx.Create(&row).Error; err != nil {
			return 0, fmt.Errorf("create backend: %w", err)
		}
		return row.ID, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get backend: %w", err)
	}

	updates := map[string]any{
		"display_name": backend.DisplayName,
		"version":      backend.Version,
		"created_at":   backend.CreatedAt,
	}
	if err := tx.Model(&row).Updates(updates).Error; err != nil {
		return 0, fmt.Errorf("update backend: %w", err)
	}
	return row.ID, nil
}
