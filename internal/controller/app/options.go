package app

import (
	"errors"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"github.com/lgc202/ingate/internal/envoy/delivery"
	"github.com/lgc202/ingate/pkg/xlog"
)

const (
	defaultXDSListenAddress      = ":18000"
	defaultInternalListenAddress = "127.0.0.1:18080"
)

// Options 表示 ingate-controller 启动参数
type Options struct {
	Master                string
	Kubeconfig            string
	XDSListenAddress      string
	InternalListenAddress string
	CandidateACKTimeout   time.Duration
	NACKRollbackTimeout   time.Duration
	ResyncPeriod          time.Duration
	LogFormat             xlog.Format
	LogLevel              xlog.Level
	LogFile               xlog.FileOptions
	LogStdout             bool
}

// NewOptions 创建 ingate-controller 默认启动参数
func NewOptions() *Options {
	return &Options{
		XDSListenAddress:      defaultXDSListenAddress,
		InternalListenAddress: defaultInternalListenAddress,
		CandidateACKTimeout:   delivery.DefaultACKTimeout,
		NACKRollbackTimeout:   delivery.DefaultNACKRollbackTimeout,
		LogFormat:             xlog.FormatText,
		LogLevel:              xlog.LevelInfo,
		LogStdout:             true,
	}
}

// Validate 校验启动参数自身可以确定的约束
func (o *Options) Validate() error {
	if strings.TrimSpace(o.XDSListenAddress) == "" {
		return errors.New("xDS listen address must not be empty")
	}
	if strings.TrimSpace(o.InternalListenAddress) == "" {
		return errors.New("internal listen address must not be empty")
	}
	if o.CandidateACKTimeout < 0 {
		return errors.New("candidate ACK timeout must not be negative")
	}
	if o.NACKRollbackTimeout < 0 {
		return errors.New("NACK rollback timeout must not be negative")
	}
	if o.ResyncPeriod < 0 {
		return errors.New("resync period must not be negative")
	}
	return nil
}

// AddFlags 注册 ingate-controller 命令行参数
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.Master, "master", o.Master, "ingate-apiserver 地址")
	flags.StringVar(&o.Kubeconfig, "kubeconfig", o.Kubeconfig, "连接 ingate-apiserver 的 kubeconfig 路径")
	flags.StringVar(&o.XDSListenAddress, "xds-listen-address", o.XDSListenAddress, "Envoy ADS gRPC 监听地址")
	flags.StringVar(&o.InternalListenAddress, "internal-listen-address", o.InternalListenAddress, "健康检查和内部状态 HTTP 监听地址")
	flags.DurationVar(&o.CandidateACKTimeout, "candidate-ack-timeout", o.CandidateACKTimeout, "Candidate 首次发送后的 ACK 等待时间")
	flags.DurationVar(&o.NACKRollbackTimeout, "nack-rollback-timeout", o.NACKRollbackTimeout, "NACK 同步回滚的最长等待时间")
	flags.DurationVar(&o.ResyncPeriod, "resync-period", o.ResyncPeriod, "informer 全量 resync 周期，0 表示关闭")
	flags.Var((*xlog.FormatValue)(&o.LogFormat), "log-format", "日志输出格式：text 或 json")
	flags.Var((*xlog.LevelValue)(&o.LogLevel), "log-level", "日志级别：debug、info、warn 或 error")
	flags.BoolVar(&o.LogStdout, "log-stdout", o.LogStdout, "是否输出日志到 stdout")
	flags.StringVar(&o.LogFile.Path, "log-file", o.LogFile.Path, "日志文件路径，留空时只输出到 stdout")
	flags.IntVar(&o.LogFile.MaxSizeMB, "log-max-size-mb", o.LogFile.MaxSizeMB, "单个日志文件最大大小 MB")
	flags.IntVar(&o.LogFile.MaxBackups, "log-max-backups", o.LogFile.MaxBackups, "最多保留的旧日志文件数")
	flags.IntVar(&o.LogFile.MaxAgeDays, "log-max-age-days", o.LogFile.MaxAgeDays, "旧日志最多保留天数")
	flags.BoolVar(&o.LogFile.Compress, "log-compress", o.LogFile.Compress, "是否压缩轮转后的日志文件")
}
