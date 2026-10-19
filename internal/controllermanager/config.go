package controllermanager

import (
	"fmt"
	"os"

	"buf.build/go/protovalidate"
	"github.com/lgc202/ingate/internal/controllermanager/conf"
	mysqlclient "github.com/lgc202/ingate/internal/pkg/mysql"
	"google.golang.org/protobuf/encoding/protojson"
	"sigs.k8s.io/yaml"
)

func loadConfig(path string) (*conf.Bootstrap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}

	jsonData, err := yaml.YAMLToJSONStrict(data)
	if err != nil {
		return nil, fmt.Errorf("decode config file %q: %w", path, err)
	}

	config := new(conf.Bootstrap)
	if err := protojson.Unmarshal(jsonData, config); err != nil {
		return nil, fmt.Errorf("unmarshal config file %q: %w", path, err)
	}

	if err := protovalidate.Validate(config); err != nil {
		return nil, fmt.Errorf("validate config file %q: %w", path, err)
	}

	return config, nil
}

func mysqlConfig(config *conf.MySQL) mysqlclient.Config {
	return mysqlclient.Config{
		DSN:                   config.GetDsn(),
		MaxOpenConnections:    int(config.GetMaxOpenConnections()),
		MaxIdleConnections:    int(config.GetMaxIdleConnections()),
		ConnectionMaxLifetime: config.GetConnectionMaxLifetime().AsDuration(),
		ConnectionMaxIdleTime: config.GetConnectionMaxIdleTime().AsDuration(),
	}
}
