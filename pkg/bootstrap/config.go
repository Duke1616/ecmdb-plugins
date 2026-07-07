package bootstrap

import (
	"fmt"
	"net"
	"strings"

	"github.com/Duke1616/ecmdb/pkg/plugin"
	"github.com/spf13/viper"
)

// Config 定义通用的插件环境配置结构
type Config struct {
	Addr           string `mapstructure:"addr"`
	Upstream       string `mapstructure:"upstream"`
	ECMDBGRPCAddr  string `mapstructure:"-"`
	TimeoutSeconds int    `mapstructure:"timeout_seconds"`
}

// PluginConfig 提供给具体插件 Handler 消费的通用业务连接配置
type PluginConfig struct {
	Upstream       string
	Resolver       plugin.ContextResolver
	TimeoutSeconds int
}

// LoadConfig 从 viper 中提取公共环境配置并应用默认值，避免每个插件手写装配逻辑
func LoadConfig(name string) Config {
	cfg := Config{
		Addr:           ":18080",
		TimeoutSeconds: 5,
	}

	_ = viper.UnmarshalKey(name, &cfg)

	cfg.ECMDBGRPCAddr = viper.GetString("ecmdb.grpc_addr")

	if cfg.Upstream == "" {
		cfg.Upstream = DefaultUpstream(cfg.Addr)
	}

	return cfg
}

// InitListener 基于监听地址统一提供 TCP 服务监听器初始化
func InitListener(addr string) net.Listener {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		panic(fmt.Errorf("failed to listen on %s: %w", addr, err))
	}
	return listener
}

// DefaultUpstream 提供根据 Addr 自动推导默认物理 IP + 端口 upstream 链路的辅助算法
func DefaultUpstream(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://127.0.0.1:18080"
	}
	if host == "" || host == "::" || host == "0.0.0.0" || host == "[::]" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%s", strings.Trim(host, "[]"), port)
}
