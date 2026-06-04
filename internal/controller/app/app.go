// Package app 实现 ingate-controller 服务入口
package app

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/lgc202/ingate/internal/controller/controller"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
	"github.com/lgc202/ingate/pkg/logx"
)

const (
	defaultTarget    = "xds"
	defaultLogFormat = logx.FormatText
	defaultLogLevel  = logx.LevelInfo
)

const usage = `ingate-controller 负责声明式资源的状态收敛

职责：
  - 监听 ingate-apiserver 中的资源变化
  - 调用 compiler 和 pipeline 生成 RuntimeSnapshot
  - 推进资源状态并为 ingate-xds 准备运行时配置
`

// Options 表示 ingate-controller 启动参数
type Options struct {
	Master       string
	Kubeconfig   string
	Target       string
	ResyncPeriod time.Duration
	LogFormat    logx.Format
	LogLevel     logx.Level
	LogFile      logx.FileOptions
	LogStdout    bool
}

// Run 执行 ingate-controller 服务
func Run(args []string, stdout, stderr io.Writer) error {
	flags := pflag.NewFlagSet("ingate-controller", pflag.ContinueOnError)
	flags.SetOutput(stderr)

	options := NewOptions()
	options.AddFlags(flags)
	flags.Usage = func() {
		fmt.Fprint(stdout, usage)
		fmt.Fprintln(stdout)
		flags.SetOutput(stdout)
		flags.PrintDefaults()
		flags.SetOutput(stderr)
	}
	if err := flags.Parse(args); err != nil {
		if err == pflag.ErrHelp {
			return nil
		}
		return err
	}

	config, err := clientcmd.BuildConfigFromFlags(options.Master, options.Kubeconfig)
	if err != nil {
		return err
	}
	client, err := clientset.NewForConfig(config)
	if err != nil {
		return err
	}
	logger, err := logx.New(logx.Options{
		Output: logOutput(options.LogStdout, stdout),
		Format: options.LogFormat,
		Level:  options.LogLevel,
		File:   options.LogFile,
	})
	if err != nil {
		return err
	}
	defer logger.Close()

	ctrl, err := controller.New(client, options.Target, options.ResyncPeriod, logger.With("component", "ingate-controller"))
	if err != nil {
		return err
	}

	return ctrl.Run(server.SetupSignalContext())
}

// NewOptions 创建 ingate-controller 默认启动参数
func NewOptions() *Options {
	return &Options{
		Target:       defaultTarget,
		ResyncPeriod: 0,
		LogFormat:    defaultLogFormat,
		LogLevel:     defaultLogLevel,
		LogStdout:    true,
	}
}

// AddFlags 注册 ingate-controller 命令行参数
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.Master, "master", o.Master, "ingate-apiserver 地址")
	flags.StringVar(&o.Kubeconfig, "kubeconfig", o.Kubeconfig, "连接 ingate-apiserver 的 kubeconfig 路径")
	flags.StringVar(&o.Target, "target", o.Target, "编译输出的运行时 target")
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

func logOutput(enabled bool, stdout io.Writer) io.Writer {
	if !enabled {
		return nil
	}
	return stdout
}
