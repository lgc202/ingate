package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Upstream 对应 MySQL upstreams 表及其 Endpoint 集合。
type Upstream struct {
	ID              uint64             `gorm:"column:id;primaryKey;autoIncrement"`
	Name            string             `gorm:"column:name"`
	ResourceVersion string             `gorm:"column:resource_version"`
	CreatedAt       time.Time          `gorm:"column:created_at;autoCreateTime:false"`
	UpdatedAt       time.Time          `gorm:"column:updated_at;autoCreateTime;autoUpdateTime"`
	Endpoints       []UpstreamEndpoint `gorm:"-"`
}

// TableName 返回 Upstream 使用的数据库表名。
func (Upstream) TableName() string { return "upstreams" }

// UpstreamEndpoint 对应 MySQL upstream_endpoints 表中的一个后端端点。
type UpstreamEndpoint struct {
	ID         uint64 `gorm:"column:id;primaryKey;autoIncrement"`
	UpstreamID uint64 `gorm:"column:upstream_id"`
	Address    string `gorm:"column:address"`
	Port       uint16 `gorm:"column:port"`
	Weight     int32  `gorm:"column:weight"`
}

// TableName 返回 UpstreamEndpoint 使用的数据库表名。
func (UpstreamEndpoint) TableName() string { return "upstream_endpoints" }

// UpstreamPage 保存一次列表查询的结果和总数。
type UpstreamPage struct {
	Items      []Upstream
	TotalCount int64
}

// UpsertUpstream 在一个事务内更新 Upstream 及其 Endpoint 集合。
func (s *Store) UpsertUpstream(ctx context.Context, upstream Upstream) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		id, err := upsertUpstreamRow(tx, upstream)
		if err != nil {
			return err
		}

		if err := tx.Where("upstream_id = ?", id).Delete(&UpstreamEndpoint{}).Error; err != nil {
			return fmt.Errorf("delete upstream endpoints: %w", err)
		}

		endpoints := make([]UpstreamEndpoint, len(upstream.Endpoints))
		for i, endpoint := range upstream.Endpoints {
			endpoints[i] = UpstreamEndpoint{
				UpstreamID: id,
				Address:    endpoint.Address,
				Port:       endpoint.Port,
				Weight:     endpoint.Weight,
			}
		}

		if len(endpoints) == 0 {
			return nil
		}

		if err := tx.Create(&endpoints).Error; err != nil {
			return fmt.Errorf("insert upstream endpoints: %w", err)
		}
		return nil
	})
}

// DeleteUpstream 删除 Upstream 及其 Endpoint；不存在时不报错。
func (s *Store) DeleteUpstream(ctx context.Context, name string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var upstream Upstream
		err := tx.Select("id").Where("name = ?", name).Take(&upstream).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("get upstream: %w", err)
		}

		if err := tx.Where("upstream_id = ?", upstream.ID).Delete(&UpstreamEndpoint{}).Error; err != nil {
			return fmt.Errorf("delete upstream endpoints: %w", err)
		}

		if err := tx.Delete(&upstream).Error; err != nil {
			return fmt.Errorf("delete upstream: %w", err)
		}
		return nil
	})
}

// ListUpstreamNames 返回所有已投影 Upstream 的名称，供启动对账使用。
func (s *Store) ListUpstreamNames(ctx context.Context) ([]string, error) {
	var names []string
	query := s.db.WithContext(ctx).Model(&Upstream{}).Order("name")
	if err := query.Pluck("name", &names).Error; err != nil {
		return nil, fmt.Errorf("list upstream names: %w", err)
	}
	return names, nil
}

// GetUpstream 在同一个只读事务中读取 Upstream 和 Endpoint。
func (s *Store) GetUpstream(ctx context.Context, name string) (Upstream, error) {
	var upstream Upstream
	err := s.readOnlyTransaction(ctx, func(tx *gorm.DB) error {
		err := tx.Where("name = ?", name).Take(&upstream).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: %w", ErrNotFound, err)
		}
		if err != nil {
			return fmt.Errorf("get upstream: %w", err)
		}

		endpointQuery := tx.Where("upstream_id = ?", upstream.ID).Order("address, port")
		if err := endpointQuery.Find(&upstream.Endpoints).Error; err != nil {
			return fmt.Errorf("get upstream endpoints: %w", err)
		}

		return nil
	})
	if err != nil {
		return Upstream{}, err
	}

	return upstream, nil
}

// ListUpstreams 按名称分页读取 Upstream 与 Endpoint。
func (s *Store) ListUpstreams(ctx context.Context, size, number int32) (UpstreamPage, error) {
	var result UpstreamPage
	err := s.readOnlyTransaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Model(&Upstream{}).Count(&result.TotalCount).Error; err != nil {
			return fmt.Errorf("count upstreams: %w", err)
		}

		upstreamQuery := page(tx.Model(&Upstream{}).Order("name"), size, number)
		if err := upstreamQuery.Find(&result.Items).Error; err != nil {
			return fmt.Errorf("list upstreams: %w", err)
		}
		if len(result.Items) == 0 {
			return nil
		}

		ids := make([]uint64, len(result.Items))
		indexes := make(map[uint64]int, len(result.Items))
		for i, upstream := range result.Items {
			ids[i] = upstream.ID
			indexes[upstream.ID] = i
		}

		var endpoints []UpstreamEndpoint
		endpointQuery := tx.Where("upstream_id IN ?", ids).Order("upstream_id, address, port")
		if err := endpointQuery.Find(&endpoints).Error; err != nil {
			return fmt.Errorf("list upstream endpoints: %w", err)
		}

		for _, endpoint := range endpoints {
			i := indexes[endpoint.UpstreamID]
			result.Items[i].Endpoints = append(result.Items[i].Endpoints, endpoint)
		}

		return nil
	})
	if err != nil {
		return UpstreamPage{}, err
	}

	return result, nil
}

func upsertUpstreamRow(tx *gorm.DB, upstream Upstream) (uint64, error) {
	var row Upstream
	err := tx.Select("id").Where("name = ?", upstream.Name).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = Upstream{
			Name:            upstream.Name,
			ResourceVersion: upstream.ResourceVersion,
			CreatedAt:       upstream.CreatedAt,
		}
		if err := tx.Create(&row).Error; err != nil {
			return 0, fmt.Errorf("create upstream: %w", err)
		}
		return row.ID, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get upstream: %w", err)
	}

	updates := map[string]any{
		"resource_version": upstream.ResourceVersion,
		"created_at":       upstream.CreatedAt,
	}
	if err := tx.Model(&row).Updates(updates).Error; err != nil {
		return 0, fmt.Errorf("update upstream: %w", err)
	}
	return row.ID, nil
}
