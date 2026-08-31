// Package kafkaclient 提供 Ingate 组件共用的 franz-go 客户端连接能力。
package kafkaclient

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"

	"github.com/lgc202/ingate/internal/pkg/tlsconfig"
)

// SASL 定义 Kafka 客户端身份认证参数。
type SASL struct {
	Mechanism string
	Username  string
	Password  string
}

// Config 定义 Kafka broker 连接的公共参数。
// Topic、Consumer Group 和生产消费语义由调用组件通过 options 明确传入。
type Config struct {
	Brokers     []string
	DialTimeout time.Duration
	SASL        SASL
	TLS         tlsconfig.ClientConfig
}

// Validate 校验 SASL 机制及凭据组合。
func (c SASL) Validate() error {
	_, err := c.mechanism()
	return err
}

// New 创建 franz-go 客户端，并在公共连接参数之后应用组件专用 options。
func New(config Config, options ...kgo.Opt) (*kgo.Client, error) {
	if len(config.Brokers) == 0 {
		return nil, errors.New("kafka brokers must not be empty")
	}
	if config.DialTimeout <= 0 {
		return nil, errors.New("kafka dial timeout must be greater than zero")
	}

	clientOptions := []kgo.Opt{
		kgo.SeedBrokers(config.Brokers...),
		kgo.DialTimeout(config.DialTimeout),
	}
	mechanism, err := config.SASL.mechanism()
	if err != nil {
		return nil, err
	}
	if mechanism != nil {
		clientOptions = append(clientOptions, kgo.SASL(mechanism))
	}
	tlsConfig, err := tlsconfig.NewClient(config.TLS)
	if err != nil {
		return nil, fmt.Errorf("configure Kafka TLS: %w", err)
	}
	if tlsConfig != nil {
		clientOptions = append(clientOptions, kgo.DialTLSConfig(tlsConfig))
	}
	client, err := kgo.NewClient(append(clientOptions, options...)...)
	if err != nil {
		return nil, fmt.Errorf("create Kafka client: %w", err)
	}
	return client, nil
}

func (c SASL) mechanism() (sasl.Mechanism, error) {
	mechanism := strings.ToUpper(c.Mechanism)
	if mechanism == "" && (c.Username != "" || c.Password != "") {
		return nil, errors.New("kafka SASL mechanism is required when credentials are configured")
	}
	if mechanism != "" && (c.Username == "" || c.Password == "") {
		return nil, errors.New("kafka SASL username and password are required")
	}
	switch mechanism {
	case "":
		return nil, nil
	case "PLAIN":
		return plain.Auth{User: c.Username, Pass: c.Password}.AsMechanism(), nil
	case "SCRAM-SHA-256":
		return scram.Auth{User: c.Username, Pass: c.Password}.AsSha256Mechanism(), nil
	case "SCRAM-SHA-512":
		return scram.Auth{User: c.Username, Pass: c.Password}.AsSha512Mechanism(), nil
	default:
		return nil, fmt.Errorf("unsupported Kafka SASL mechanism %q", c.Mechanism)
	}
}
