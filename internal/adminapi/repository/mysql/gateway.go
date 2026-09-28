package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Gateway 对应 MySQL gateways 表及其 Listener 集合。
type Gateway struct {
	ID              uint64            `gorm:"column:id;primaryKey;autoIncrement"`
	Name            string            `gorm:"column:name"`
	DisplayName     string            `gorm:"column:display_name"`
	ResourceVersion string            `gorm:"column:resource_version"`
	CreatedAt       time.Time         `gorm:"column:created_at;autoCreateTime:false"`
	UpdatedAt       time.Time         `gorm:"column:updated_at;autoCreateTime;autoUpdateTime"`
	Listeners       []GatewayListener `gorm:"-"`
}

// TableName 返回 Gateway 使用的数据库表名。
func (Gateway) TableName() string { return "gateways" }

// GatewayListener 对应 MySQL gateway_listeners 表中的监听入口。
type GatewayListener struct {
	ID        uint64 `gorm:"column:id;primaryKey;autoIncrement"`
	GatewayID uint64 `gorm:"column:gateway_id"`
	Name      string `gorm:"column:name"`
	Protocol  string `gorm:"column:protocol"`
	Port      uint16 `gorm:"column:port"`
	Hostname  string `gorm:"column:hostname"`
}

// TableName 返回 GatewayListener 使用的数据库表名。
func (GatewayListener) TableName() string { return "gateway_listeners" }

// GatewayPage 保存一次列表查询的结果和总数。
type GatewayPage struct {
	Items      []Gateway
	TotalCount int64
}

// UpsertGateway 在一个事务内更新 Gateway 及其 Listener 集合。
func (s *Store) UpsertGateway(ctx context.Context, gateway Gateway) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		id, err := upsertGatewayRow(tx, gateway)
		if err != nil {
			return err
		}

		if err := tx.Where("gateway_id = ?", id).Delete(&GatewayListener{}).Error; err != nil {
			return fmt.Errorf("delete gateway listeners: %w", err)
		}

		listeners := make([]GatewayListener, len(gateway.Listeners))
		for i, listener := range gateway.Listeners {
			listeners[i] = GatewayListener{
				GatewayID: id,
				Name:      listener.Name,
				Protocol:  listener.Protocol,
				Port:      listener.Port,
				Hostname:  listener.Hostname,
			}
		}

		if len(listeners) == 0 {
			return nil
		}

		if err := tx.Create(&listeners).Error; err != nil {
			return fmt.Errorf("insert gateway listeners: %w", err)
		}
		return nil
	})
}

// DeleteGateway 删除 Gateway 及其 Listener；不存在时不报错。
func (s *Store) DeleteGateway(ctx context.Context, name string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var gateway Gateway
		err := tx.Select("id").Where("name = ?", name).Take(&gateway).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("get gateway: %w", err)
		}

		if err := tx.Where("gateway_id = ?", gateway.ID).Delete(&GatewayListener{}).Error; err != nil {
			return fmt.Errorf("delete gateway listeners: %w", err)
		}

		if err := tx.Delete(&gateway).Error; err != nil {
			return fmt.Errorf("delete gateway: %w", err)
		}
		return nil
	})
}

// ListGatewayNames 返回所有已投影 Gateway 的名称，供启动对账使用。
func (s *Store) ListGatewayNames(ctx context.Context) ([]string, error) {
	var names []string
	query := s.db.WithContext(ctx).Model(&Gateway{}).Order("name")
	if err := query.Pluck("name", &names).Error; err != nil {
		return nil, fmt.Errorf("list gateway names: %w", err)
	}
	return names, nil
}

// GetGateway 在同一个只读事务中读取 Gateway 和 Listener。
func (s *Store) GetGateway(ctx context.Context, name string) (Gateway, error) {
	var gateway Gateway
	err := s.readOnlyTransaction(ctx, func(tx *gorm.DB) error {
		err := tx.Where("name = ?", name).Take(&gateway).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: %w", ErrNotFound, err)
		}
		if err != nil {
			return fmt.Errorf("get gateway: %w", err)
		}

		listenerQuery := tx.Where("gateway_id = ?", gateway.ID).Order("name")
		if err := listenerQuery.Find(&gateway.Listeners).Error; err != nil {
			return fmt.Errorf("get gateway listeners: %w", err)
		}

		return nil
	})
	if err != nil {
		return Gateway{}, err
	}

	return gateway, nil
}

// ListGateways 按名称分页读取 Gateway 与 Listener。
func (s *Store) ListGateways(ctx context.Context, size, number int32) (GatewayPage, error) {
	var result GatewayPage
	err := s.readOnlyTransaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Model(&Gateway{}).Count(&result.TotalCount).Error; err != nil {
			return fmt.Errorf("count gateways: %w", err)
		}

		gatewayQuery := page(tx.Model(&Gateway{}).Order("name"), size, number)
		if err := gatewayQuery.Find(&result.Items).Error; err != nil {
			return fmt.Errorf("list gateways: %w", err)
		}
		if len(result.Items) == 0 {
			return nil
		}

		ids := make([]uint64, len(result.Items))
		indexes := make(map[uint64]int, len(result.Items))
		for i, gateway := range result.Items {
			ids[i] = gateway.ID
			indexes[gateway.ID] = i
		}

		var listeners []GatewayListener
		listenerQuery := tx.Where("gateway_id IN ?", ids).Order("gateway_id, name")
		if err := listenerQuery.Find(&listeners).Error; err != nil {
			return fmt.Errorf("list gateway listeners: %w", err)
		}

		for _, listener := range listeners {
			i := indexes[listener.GatewayID]
			result.Items[i].Listeners = append(result.Items[i].Listeners, listener)
		}

		return nil
	})
	if err != nil {
		return GatewayPage{}, err
	}

	return result, nil
}

func upsertGatewayRow(tx *gorm.DB, gateway Gateway) (uint64, error) {
	var row Gateway
	err := tx.Select("id").Where("name = ?", gateway.Name).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = Gateway{
			Name:            gateway.Name,
			DisplayName:     gateway.DisplayName,
			ResourceVersion: gateway.ResourceVersion,
			CreatedAt:       gateway.CreatedAt,
		}
		if err := tx.Create(&row).Error; err != nil {
			return 0, fmt.Errorf("create gateway: %w", err)
		}
		return row.ID, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get gateway: %w", err)
	}

	updates := map[string]any{
		"display_name":     gateway.DisplayName,
		"resource_version": gateway.ResourceVersion,
		"created_at":       gateway.CreatedAt,
	}
	if err := tx.Model(&row).Updates(updates).Error; err != nil {
		return 0, fmt.Errorf("update gateway: %w", err)
	}
	return row.ID, nil
}
