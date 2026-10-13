package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/types"
)

// Gateway 对应 MySQL gateways 表及其 Listener 集合。
type Gateway struct {
	ID          uint64            `gorm:"column:id;primaryKey;autoIncrement"`
	Namespace   string            `gorm:"column:namespace"`
	Name        string            `gorm:"column:name"`
	DisplayName string            `gorm:"column:display_name"`
	Version     string            `gorm:"column:version"`
	CreatedAt   time.Time         `gorm:"column:created_at;autoCreateTime:false"`
	UpdatedAt   time.Time         `gorm:"column:updated_at;autoCreateTime;autoUpdateTime"`
	Listeners   []GatewayListener `gorm:"-"`
}

// GatewayListener 对应 MySQL gateway_listeners 表中的监听入口。
type GatewayListener struct {
	ID        uint64 `gorm:"column:id;primaryKey;autoIncrement"`
	GatewayID uint64 `gorm:"column:gateway_id"`
	Name      string `gorm:"column:name"`
	Protocol  string `gorm:"column:protocol"`
	Port      uint16 `gorm:"column:port"`
	Hostname  string `gorm:"column:hostname"`
}

// GatewayPage 保存一次列表查询的结果和总数。
type GatewayPage struct {
	Items      []Gateway
	TotalCount int64
}

// TableName 返回 Gateway 使用的数据库表名。
func (Gateway) TableName() string { return "gateways" }

// TableName 返回 GatewayListener 使用的数据库表名。
func (GatewayListener) TableName() string { return "gateway_listeners" }

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
func (s *Store) DeleteGateway(ctx context.Context, namespace, name string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var gateway Gateway
		err := tx.Select("id").Where("namespace = ? AND name = ?", namespace, name).Take(&gateway).Error
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

// ListGatewayKeys 返回所有已投影对象的空间和名称，供启动对账使用。
func (s *Store) ListGatewayKeys(ctx context.Context) ([]types.NamespacedName, error) {
	var keys []types.NamespacedName
	query := s.db.WithContext(ctx).Model(&Gateway{}).
		Select("namespace", "name").Order("namespace, name")
	if err := query.Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("list Gateway keys: %w", err)
	}
	return keys, nil
}

// GetGateway 在同一个只读事务中读取 Gateway 和 Listener。
func (s *Store) GetGateway(ctx context.Context, namespace, name string) (Gateway, error) {
	var gateway Gateway
	err := s.readOnlyTransaction(ctx, func(tx *gorm.DB) error {
		err := tx.Where("namespace = ? AND name = ?", namespace, name).Take(&gateway).Error
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

// ListGateways 在指定命名空间内按名称分页读取 Gateway 与 Listener。
func (s *Store) ListGateways(ctx context.Context, params ResourceListParams) (GatewayPage, error) {
	var result GatewayPage
	err := s.readOnlyTransaction(ctx, func(tx *gorm.DB) error {
		query := filterDisplayName(
			tx.Model(&Gateway{}).Where("namespace = ?", params.Namespace),
			params.DisplayName,
		)
		if err := query.Count(&result.TotalCount).Error; err != nil {
			return fmt.Errorf("count gateways: %w", err)
		}

		gatewayQuery := page(query.Order("name"), params.PageSize, params.PageNum)
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
	err := tx.Select("id").
		Where("namespace = ? AND name = ?", gateway.Namespace, gateway.Name).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = Gateway{
			Namespace:   gateway.Namespace,
			Name:        gateway.Name,
			DisplayName: gateway.DisplayName,
			Version:     gateway.Version,
			CreatedAt:   gateway.CreatedAt,
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
		"display_name": gateway.DisplayName,
		"version":      gateway.Version,
		"created_at":   gateway.CreatedAt,
	}
	if err := tx.Model(&row).Updates(updates).Error; err != nil {
		return 0, fmt.Errorf("update gateway: %w", err)
	}
	return row.ID, nil
}
