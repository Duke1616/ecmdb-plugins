package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Duke1616/ecmdb-plugins/plugins/ssh/ioc"
	"github.com/fsnotify/fsnotify"
	"github.com/gotomicro/ego"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

func main() {
	rootCmd := &cobra.Command{
		Use:   "ssh-plugin",
		Short: "启动 ECMDB SSH 插件服务",
		RunE:  run,
	}

	// 默认读取子插件下的 config.yaml，保持高内聚自包含
	dir, _ := os.Getwd()
	defaultCfg := dir + "/plugins/ssh/config/config.yaml"
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", defaultCfg, "配置文件路径")

	// 绑定环境变量
	_ = viper.BindEnv("ssh.addr", "SSH_PLUGIN_ADDR")
	_ = viper.BindEnv("ssh.upstream", "SSH_PLUGIN_UPSTREAM")
	_ = viper.BindEnv("ecmdb.grpc_addr", "ECMDB_GRPC_ADDR")
	_ = viper.BindEnv("ssh.timeout_seconds", "SSH_PLUGIN_TIMEOUT_SECONDS")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	cobra.OnInitialize(initViper)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(_ *cobra.Command, _ []string) error {
	app, err := ioc.InitApp()
	if err != nil {
		return fmt.Errorf("init ssh plugin app: %w", err)
	}

	// 在 Web 启动运行后，异步调用生命周期方法向主站发起注册
	go func() {
		if err = app.Register(context.Background()); err != nil {
			elog.Error("自动向主站注册插件元数据失败", elog.FieldErr(err))
		}
	}()

	if err = ego.New(ego.WithDisableBanner(true)).
		Serve(func() server.Server {
			return app.Web
		}()).
		Run(); err != nil {
		elog.Panic("run ssh plugin app", elog.FieldErr(err))
	}

	return nil
}

// initViper 开启动态监听，支持配置热重载
func initViper() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}

	// 开启文件监控
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Warning: 配置文件读取失败: %v\n", err)
	} else {
		fmt.Printf("Using config file: %s\n", viper.ConfigFileUsed())
		setLogLevel() // 每次读取/重载都设置日志级别
	}

	// 监听配置变更，支持动态切换日志级别
	viper.OnConfigChange(func(in fsnotify.Event) {
		fmt.Printf("Config file changed: %s\n", in.Name)
		setLogLevel()
	})
}

// setLogLevel 根据配置文件中的 log.debug 动态调整全局日志级别
func setLogLevel() {
	if viper.GetBool("log.debug") {
		elog.DefaultLogger.SetLevel(elog.DebugLevel)
		elog.DefaultLogger.Debug("已根据配置开启 Debug 日志级别")
	} else {
		elog.DefaultLogger.SetLevel(elog.InfoLevel)
	}
}
