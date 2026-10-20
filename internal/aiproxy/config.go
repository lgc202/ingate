package aiproxy

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/encoding/protojson"
	"sigs.k8s.io/yaml"

	"github.com/lgc202/ingate/internal/aiproxy/conf"
	logconfig "github.com/lgc202/ingate/internal/pkg/log"
)

// loadConfig 只读取、解码和校验启动配置，不创建客户端、监听 socket 或启动 Watch。
func loadConfig(configFile string) (*conf.Bootstrap, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", configFile, err)
	}

	jsonData, err := yaml.YAMLToJSONStrict(data)
	if err != nil {
		return nil, fmt.Errorf("decode config file %q: %w", configFile, err)
	}

	config := new(conf.Bootstrap)
	if err := protojson.Unmarshal(jsonData, config); err != nil {
		return nil, fmt.Errorf("unmarshal config file %q: %w", configFile, err)
	}

	if err := protovalidate.Validate(config); err != nil {
		return nil, fmt.Errorf("validate config file %q: %w", configFile, err)
	}

	if err := validateStartupConfig(config); err != nil {
		return nil, fmt.Errorf("validate config file %q: %w", configFile, err)
	}

	return config, nil
}

// validateStartupConfig 校验发现地址与路径语义；不重复 proto 的必填及数值规则。
// 路径存在性、Unix 长度限制及目录权限交给后续实际监听阶段，不在这里产生文件副作用。
func validateStartupConfig(config *conf.Bootstrap) error {
	host, port, err := net.SplitHostPort(config.GetDiscovery().GetAddress())
	if err != nil {
		return fmt.Errorf("parse AI discovery address: %w", err)
	}

	if host == "" || port == "" {
		return errors.New("AI discovery address requires a host and port")
	}

	if value, err := strconv.ParseUint(port, 10, 16); err != nil || value == 0 {
		return errors.New("AI discovery port must be between 1 and 65535")
	}

	ingress := config.GetServer().GetSocket()
	egress := config.GetEnvoy().GetEgressSocket()
	if !filepath.IsAbs(ingress) || strings.ContainsRune(ingress, 0) {
		return errors.New("AI ingress socket must be an absolute path without NUL")
	}

	if !filepath.IsAbs(egress) || strings.ContainsRune(egress, 0) {
		return errors.New("AI egress socket must be an absolute path without NUL")
	}

	if filepath.Clean(ingress) == filepath.Clean(egress) {
		return errors.New("AI ingress and egress sockets must differ")
	}

	return nil
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
