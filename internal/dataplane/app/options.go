package app

import (
	"github.com/lgc202/ingate/pkg/xlog"
	"github.com/spf13/pflag"
)

const (
	defaultListenAddress = "127.0.0.1:18081"
	defaultLogFormat     = xlog.FormatText
	defaultLogLevel      = xlog.LevelInfo
)

// Options 表示 ingate-dataplane 启动参数
type Options struct {
	ListenAddress string
	LogFormat     xlog.Format
	LogLevel      xlog.Level
	LogFile       xlog.FileOptions
	LogStdout     bool
}

// NewOptions 创建 ingate-dataplane 默认启动参数
func NewOptions() *Options {
	return &Options{
		ListenAddress: defaultListenAddress,
		LogFormat:     defaultLogFormat,
		LogLevel:      defaultLogLevel,
		LogStdout:     true,
	}
}

// AddFlags 注册 ingate-dataplane 命令行参数
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.ListenAddress, "listen-address", o.ListenAddress, "数据面服务 HTTP 监听地址")
	flags.Var((*xlog.FormatValue)(&o.LogFormat), "log-format", "日志输出格式：text 或 json")
	flags.Var((*xlog.LevelValue)(&o.LogLevel), "log-level", "日志级别：debug、info、warn 或 error")
	flags.BoolVar(&o.LogStdout, "log-stdout", o.LogStdout, "是否输出日志到 stdout")
	flags.StringVar(&o.LogFile.Path, "log-file", o.LogFile.Path, "日志文件路径，留空时只输出到 stdout")
	flags.IntVar(&o.LogFile.MaxSizeMB, "log-max-size-mb", o.LogFile.MaxSizeMB, "单个日志文件最大大小 MB")
	flags.IntVar(&o.LogFile.MaxBackups, "log-max-backups", o.LogFile.MaxBackups, "最多保留的旧日志文件数")
	flags.IntVar(&o.LogFile.MaxAgeDays, "log-max-age-days", o.LogFile.MaxAgeDays, "旧日志最多保留天数")
	flags.BoolVar(&o.LogFile.Compress, "log-compress", o.LogFile.Compress, "是否压缩轮转后的日志文件")
}
