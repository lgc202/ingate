package aiproxy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/encoding/protojson"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/yaml"

	"github.com/lgc202/ingate/internal/aiproxy/conf"
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

// validateStartupConfig 复用原生 Namespace 与路径语义；不重复 proto 的必填及数值规则。
// 路径存在性、Unix 长度限制及目录权限交给后续实际监听阶段，不在这里产生文件副作用。
func validateStartupConfig(config *conf.Bootstrap) error {
	namespace := config.GetApiServer().GetAiConfigNamespace()
	if messages := validation.IsDNS1123Label(namespace); len(messages) != 0 {
		return fmt.Errorf("AI configuration namespace %q must be a valid DNS label", namespace)
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
