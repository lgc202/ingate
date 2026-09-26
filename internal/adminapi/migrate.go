package adminapi

import (
	"context"
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/migration"
)

// Migrate 将 Admin API 的 MySQL Schema 迁移到当前版本。
func Migrate(ctx context.Context, configFile string) error {
	config, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("load Admin API config: %w", err)
	}

	if err := migration.Up(ctx, config.GetMysql().GetDsn()); err != nil {
		return fmt.Errorf("migrate Admin API database: %w", err)
	}

	return nil
}
