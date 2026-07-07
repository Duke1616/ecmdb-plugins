package ioc

import (
	"net"

	pluginv1 "github.com/Duke1616/ecmdb-plugins/api/proto/gen/ecmdb/plugin/v1"
	"github.com/Duke1616/ecmdb-plugins/pkg/bootstrap"
	common_grpc "github.com/Duke1616/ecmdb-plugins/pkg/grpc"
	sshweb "github.com/Duke1616/ecmdb-plugins/plugins/ssh/internal/web"
	"github.com/Duke1616/ecmdb/pkg/plugin"
	grpcpkg "github.com/Duke1616/etask/pkg/grpc"
	"github.com/Duke1616/etask/pkg/grpc/registry"
	"github.com/spf13/viper"
)

func InitConfig() bootstrap.Config {
	return bootstrap.LoadConfig("ssh")
}

func InitListener(cfg bootstrap.Config) net.Listener {
	return bootstrap.InitListener(cfg.Addr)
}

func InitResolverClient(reg registry.Registry) *common_grpc.Resolver {
	var cfg grpcpkg.ClientConfig
	if err := viper.UnmarshalKey("grpc.client.ecmdb", &cfg); err != nil {
		panic(err)
	}

	cc, err := grpcpkg.NewClientConn(
		reg,
		grpcpkg.WithServiceName(cfg.Name),
		grpcpkg.WithClientJWTAuth(cfg.AuthToken),
	)
	if err != nil {
		panic(err)
	}

	client := pluginv1.NewPluginRuntimeServiceClient(cc)
	return common_grpc.NewResolver(client)
}

func InitContextResolver(resolver *common_grpc.Resolver) plugin.ContextResolver {
	return resolver
}

func InitSSHHandler(cfg bootstrap.Config, resolver plugin.ContextResolver) *sshweb.Handler {
	return sshweb.NewHandler(bootstrap.PluginConfig{
		Upstream:       cfg.Upstream,
		Resolver:       resolver,
		TimeoutSeconds: cfg.TimeoutSeconds,
	})
}

// InitWebServer 通过通用的 bootstrap 框架一键装配 SSH 插件服务
func InitWebServer(
	cfg bootstrap.Config,
	sshHdl *sshweb.Handler,
	listener net.Listener,
	resolver *common_grpc.Resolver,
) *bootstrap.PluginApp {
	return bootstrap.NewPluginApp(bootstrap.BootstrapOptions{
		Plugin:              sshHdl,
		Resolver:            resolver,
		Upstream:            cfg.Upstream,
		StaticDist:          "./plugins/ssh/frontend/dist",
		Listener:            listener,
		PermissionProviders: nil,
	})
}
