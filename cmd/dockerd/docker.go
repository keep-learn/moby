package main

import (
	"fmt"
	"os"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/docker/rootless"
	"github.com/moby/buildkit/util/apicaps"
	"github.com/moby/term"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	honorXDG bool
)

// 启动后台守护进程
func newDaemonCommand() (*cobra.Command, error) {
	// 配置文件
	opts := newDaemonOptions(config.New())

	// 构建 cobra.Command 结构体
	cmd := &cobra.Command{
		Use:           "dockerd [OPTIONS]",
		Short:         "A self-sufficient runtime for containers.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.flags = cmd.Flags()
			// 主要逻辑在这里
			return runDaemon(opts)
		},
		DisableFlagsInUseLine: true,
		Version:               fmt.Sprintf("%s, build %s", dockerversion.Version, dockerversion.GitCommit),
	}
	// 注册 docker cli command
	cli.SetupRootCommand(cmd)

	flags := cmd.Flags()
	flags.BoolP("version", "v", false, "Print version information and quit")
	defaultDaemonConfigFile, err := getDefaultDaemonConfigFile()
	if err != nil {
		return nil, err
	}
	flags.StringVar(&opts.configFile, "config-file", defaultDaemonConfigFile, "Daemon configuration file")
	configureCertsDir()
	// 命令行参数获取
	opts.installFlags(flags)
	if err := installConfigFlags(opts.daemonConfig, flags); err != nil {
		return nil, err
	}
	installServiceFlags(flags)

	return cmd, nil
}

// 自动加载的
func init() {
	if dockerversion.ProductName != "" {
		apicaps.ExportedProduct = dockerversion.ProductName
	}
	// When running with RootlessKit, $XDG_RUNTIME_DIR, $XDG_DATA_HOME, and $XDG_CONFIG_HOME needs to be
	// honored as the default dirs, because we are unlikely to have permissions to access the system-wide
	// directories.
	//
	// Note that even running with --rootless, when not running with RootlessKit, honorXDG needs to be kept false,
	// because the system-wide directories in the current mount namespace are expected to be accessible.
	// ("rootful" dockerd in rootless dockerd, #38702)
	honorXDG = rootless.RunningWithRootlessKit()
}

// 这个是 docker的守护进程入口 dockerd
func main() {
	// 如果存在，直接返回
	if reexec.Init() {
		return
	}

	// 初始化日志，使用的是 logrus
	// initial log formatting; this setting is updated after the daemon configuration is loaded.
	logrus.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: jsonmessage.RFC3339NanoFixed,
		FullTimestamp:   true,
	})

	// Set terminal emulation based on platform as required.
	_, stdout, stderr := term.StdStreams()

	initLogging(stdout, stderr)

	// 定义了一个匿名函数
	onError := func(err error) {
		fmt.Fprintf(stderr, "%s\n", err)
		os.Exit(1)
	}

	// 启动后台守护进程（这个最重要）
	cmd, err := newDaemonCommand()
	if err != nil {
		onError(err)
	}
	cmd.SetOut(stdout)
	if err := cmd.Execute(); err != nil {
		onError(err)
	}
}
