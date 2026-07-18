package appconfig_test

import (
	"path/filepath"
	"testing"

	kitconfig "github.com/lgc202/go-kit/config"
	"github.com/lgc202/go-kit/logx"

	adminapp "github.com/lgc202/ingate/internal/adminapi/app"
	apiserverapp "github.com/lgc202/ingate/internal/apiserver/app"
	controllerapp "github.com/lgc202/ingate/internal/controller/app"
)

func TestServiceConfigFiles(t *testing.T) {
	configDir := filepath.Join("..", "..", "..", "configs")

	t.Run("admin api", func(t *testing.T) {
		t.Setenv("INGATE_ADMIN_API_SERVER_LISTEN_ADDRESS", "127.0.0.1:19001")
		loaded, err := kitconfig.Load[adminapp.Config](
			filepath.Join(configDir, "ingate-admin-api.yaml"),
			kitconfig.WithEnv[adminapp.Config]("INGATE_ADMIN_API"),
		)
		if err != nil {
			t.Fatalf("config.Load(admin api) error = %v", err)
		}
		settings := loaded.Get()
		if err := settings.Validate(); err != nil {
			t.Fatalf("Config.Validate(admin api) error = %v", err)
		}
		if got, want := settings.Server.ListenAddress, "127.0.0.1:19001"; got != want {
			t.Errorf("admin api listen address = %q, want environment override %q", got, want)
		}
		if settings.Logging.Stdout {
			t.Error("admin api logging stdout = true, want false")
		}
		if settings.Logging.Format != logx.FormatJSON {
			t.Errorf("admin api logging format = %q, want %q", settings.Logging.Format, logx.FormatJSON)
		}
		if settings.Logging.File.Path == "" {
			t.Error("admin api logging file path is empty")
		}
	})

	t.Run("apiserver", func(t *testing.T) {
		loaded, err := kitconfig.Load[apiserverapp.Config](filepath.Join(configDir, "ingate-apiserver.yaml"))
		if err != nil {
			t.Fatalf("config.Load(apiserver) error = %v", err)
		}
		settings := loaded.Get()
		if err := settings.Validate(); err != nil {
			t.Fatalf("Config.Validate(apiserver) error = %v", err)
		}
		if settings.Logging.Stdout {
			t.Error("apiserver logging stdout = true, want false")
		}
		if settings.Logging.Format != logx.FormatJSON {
			t.Errorf("apiserver logging format = %q, want %q", settings.Logging.Format, logx.FormatJSON)
		}
		if settings.Logging.File.Path == "" {
			t.Error("apiserver logging file path is empty")
		}
	})

	t.Run("controller", func(t *testing.T) {
		loaded, err := kitconfig.Load[controllerapp.Config](filepath.Join(configDir, "ingate-controller.yaml"))
		if err != nil {
			t.Fatalf("config.Load(controller) error = %v", err)
		}
		settings := loaded.Get()
		if err := settings.Validate(); err != nil {
			t.Fatalf("Config.Validate(controller) error = %v", err)
		}
		if settings.Logging.Stdout {
			t.Error("controller logging stdout = true, want false")
		}
		if settings.Logging.Format != logx.FormatJSON {
			t.Errorf("controller logging format = %q, want %q", settings.Logging.Format, logx.FormatJSON)
		}
		if settings.Logging.File.Path == "" {
			t.Error("controller logging file path is empty")
		}
	})
}
