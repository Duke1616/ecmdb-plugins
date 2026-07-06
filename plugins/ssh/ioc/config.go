package ioc

import (
	"fmt"
	"net"
	"strings"

	sshconfig "github.com/Duke1616/ecmdb-plugins/plugins/ssh/internal/config"
	"github.com/Duke1616/ecmdb/pkg/plugin"
	"github.com/spf13/viper"
)

type Config struct {
	Addr           string
	Upstream       string
	ECMDBGRPCAddr  string
	TimeoutSeconds int
	Resolver       plugin.ContextResolver
}

func InitConfig() Config {
	addr := viper.GetString("ssh.addr")
	if addr == "" {
		addr = ":18080"
	}

	upstream := viper.GetString("ssh.upstream")
	if upstream == "" {
		upstream = defaultUpstream(addr)
	}

	timeoutSeconds := viper.GetInt("ssh.timeout_seconds")
	if timeoutSeconds == 0 {
		timeoutSeconds = 5
	}

	return Config{
		Addr:           addr,
		Upstream:       upstream,
		ECMDBGRPCAddr:  viper.GetString("ecmdb.grpc_addr"),
		TimeoutSeconds: timeoutSeconds,
	}
}

func (cfg Config) SSHConfig() sshconfig.Config {
	return sshconfig.Config{
		Upstream:       cfg.Upstream,
		Resolver:       cfg.Resolver,
		TimeoutSeconds: cfg.TimeoutSeconds,
	}
}

func InitListener(cfg Config) net.Listener {
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		panic(err)
	}
	return listener
}

func defaultUpstream(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://127.0.0.1:18080"
	}
	if host == "" || host == "::" || host == "0.0.0.0" || host == "[::]" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%s", strings.Trim(host, "[]"), port)
}
