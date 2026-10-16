// Package mysql 保存管理员账户和声明式资源的查询模型。
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"gorm.io/gorm"
)

// ErrNotFound 表示查询模型中不存在指定资源。
var ErrNotFound = errors.New("resource not found")

// Store 使用注入的 GORM 客户端访问管理员账户和资源查询模型。
// 客户端的初始化与连接池关闭由组件运行入口负责。
type Store struct {
	db *gorm.DB
}

// ResourceListParams 限定一个命名空间内的分页与展示名称筛选。
type ResourceListParams struct {
	Namespace   string
	PageSize    int32
	PageNum     int32
	DisplayName string
}

// New 使用已构造的 GORM 客户端创建业务 Store，不取得连接池的关闭责任。
func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// readOnlyTransaction 让相关查询读取同一个可重复读视图。
func (s *Store) readOnlyTransaction(ctx context.Context, query func(*gorm.DB) error) error {
	return s.db.WithContext(ctx).Transaction(query, &sql.TxOptions{
		ReadOnly:  true,
		Isolation: sql.LevelRepeatableRead,
	})
}

func page(db *gorm.DB, size, number int32) *gorm.DB {
	return db.Offset(int(int64(number-1) * int64(size))).Limit(int(size))
}

func filterDisplayName(db *gorm.DB, name string) *gorm.DB {
	if name == "" {
		return db
	}

	// 显式使用 ! 转义，避免依赖 MySQL 的反斜杠 SQL 模式。
	escaped := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(name)

	return db.Where("display_name LIKE ? ESCAPE '!'", "%"+escaped+"%")
}
