package app

import (
	"time"

	"github.com/lgc202/ingate/pkg/logx"
	"github.com/spf13/pflag"
)

const (
	defaultListenAddress = ":18000"
	defaultTarget        = "xds"
	defaultLogFormat     = logx.FormatText
	defaultLogLevel      = logx.LevelInfo
)

// Options 表示 ingate-xds 启动参数
type Options struct {
	ListenAddress string
	Master        string
	Kubeconfig    string
	Target        string
	ResyncPeriod  time.Duration
	LogFormat     logx.Format
	LogLevel      logx.Level
	LogFile       logx.FileOptions
	LogStdout     bool
}

// NewOptions 创建 ingate-xds 默认启动参数
func NewOptions() *Options {
	return &Options{
		ListenAddress: defaultListenAddress,
		Target:        defaultTarget,
		ResyncPeriod:  0,
		LogFormat:     defaultLogFormat,
		LogLevel:      defaultLogLevel,
		LogStdout:     true,
	}
}

// AddFlags 注册 ingate-xds 命令行参数
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.ListenAddress, "listen-address", o.ListenAddress, "xDS gRPC 监听地址")
	flags.StringVar(&o.Master, "master", o.Master, "ingate-apiserver 地址")
	flags.StringVar(&o.Kubeconfig, "kubeconfig", o.Kubeconfig, "连接 ingate-apiserver 的 kubeconfig 路径")
	flags.StringVar(&o.Target, "target", o.Target, "消费的 RuntimeSnapshot target")
	flags.DurationVar(&o.ResyncPeriod, "resync-period", o.ResyncPeriod, "informer 全量 resync 周期")
	flags.Var((*logx.FormatValue)(&o.LogFormat), "log-format", "日志输出格式：text 或 json")
	flags.Var((*logx.LevelValue)(&o.LogLevel), "log-level", "日志级别：debug、info、warn 或 error")
	flags.BoolVar(&o.LogStdout, "log-stdout", o.LogStdout, "是否输出日志到 stdout")
	flags.StringVar(&o.LogFile.Path, "log-file", o.LogFile.Path, "日志文件路径，留空时只输出到 stdout")
	flags.IntVar(&o.LogFile.MaxSizeMB, "log-max-size-mb", o.LogFile.MaxSizeMB, "单个日志文件最大大小 MB")
	flags.IntVar(&o.LogFile.MaxBackups, "log-max-backups", o.LogFile.MaxBackups, "最多保留的旧日志文件数")
	flags.IntVar(&o.LogFile.MaxAgeDays, "log-max-age-days", o.LogFile.MaxAgeDays, "旧日志最多保留天数")
	flags.BoolVar(&o.LogFile.Compress, "log-compress", o.LogFile.Compress, "是否压缩轮转后的日志文件")
}
