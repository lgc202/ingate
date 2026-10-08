package adminapi

import (
	"fmt"
	"net"
	"os"

	"buf.build/go/protovalidate"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	logconfig "github.com/lgc202/ingate/internal/pkg/log"
	redisclient "github.com/lgc202/ingate/internal/pkg/redis"
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
	if err := validateListenAddresses(config.GetServer()); err != nil {
		return nil, fmt.Errorf("validate config file %q: %w", path, err)
	}
	if err := redisConfig(config.GetRedis()).Validate(); err != nil {
		return nil, fmt.Errorf("validate Redis config file %q: %w", path, err)
	}

	return config, nil
}

func logConfig(config *conf.Log) logconfig.Config {
	var outputs []logconfig.Output
	for _, output := range config.GetOutputs() {
		outputs = append(outputs, logconfig.Output(output))
	}
	return logconfig.Config{
		Level:     config.GetLevel(),
		Format:    logconfig.Format(config.GetFormat()),
		Outputs:   outputs,
		AddSource: config.GetAddSource(),
		File: logconfig.FileConfig{
			Path:       config.GetFile().GetPath(),
			MaxSizeMB:  int(config.GetFile().GetMaxSizeMb()),
			MaxBackups: int(config.GetFile().GetMaxBackups()),
			MaxAgeDays: int(config.GetFile().GetMaxAgeDays()),
			Compress:   config.GetFile().GetCompress(),
		},
	}
}

func redisConfig(config *conf.Redis) redisclient.Config {
	return redisclient.Config{
		Addrs:            config.GetAddrs(),
		Username:         config.GetUsername(),
		Password:         config.GetPassword(),
		SentinelUsername: config.GetSentinelUsername(),
		SentinelPassword: config.GetSentinelPassword(),
		DB:               int(config.GetDb()),
		MasterName:       config.GetMasterName(),
		IsClusterMode:    config.GetIsClusterMode(),
		PoolSize:         int(config.GetPoolSize()),
		MinIdleConns:     int(config.GetMinIdleConns()),
		DialTimeout:      config.GetDialTimeout().AsDuration(),
		ReadTimeout:      config.GetReadTimeout().AsDuration(),
		WriteTimeout:     config.GetWriteTimeout().AsDuration(),
		PoolTimeout:      config.GetPoolTimeout().AsDuration(),
		TLS:              config.GetTls(),
		TLSServerName:    config.GetTlsServerName(),
	}
}

// TLS 和登录限流接入前，管理接口只允许监听本机回环地址。
func validateListenAddresses(server *conf.Server) error {
	if err := validateLoopbackAddress("HTTP", server.GetHttp().GetAddr()); err != nil {
		return err
	}
	if err := validateLoopbackAddress("gRPC", server.GetGrpc().GetAddr()); err != nil {
		return err
	}

	return nil
}

func validateLoopbackAddress(name, address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%s listen address %q: %w", name, address, err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("%s listen address %q must use a loopback IP", name, address)
	}

	return nil
}
