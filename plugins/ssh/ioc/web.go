package ioc

import (
	"context"
	"net"
	"time"

	sshweb "github.com/Duke1616/ecmdb-plugins/plugins/ssh/internal/web"
	common_grpc "github.com/Duke1616/ecmdb-plugins/pkg/grpc"
	pluginv1 "github.com/Duke1616/ecmdb-plugins/api/proto/gen/ecmdb/plugin/v1"
	"github.com/Duke1616/ecmdb/pkg/plugin"
	"github.com/Duke1616/eiam/pkg/web/capability"
	"github.com/Duke1616/eiam/pkg/web/middleware"
	"github.com/Duke1616/eiam/pkg/web/sdk"
	grpcpkg "github.com/Duke1616/etask/pkg/grpc"
	"github.com/Duke1616/etask/pkg/grpc/registry"
	"github.com/gin-gonic/gin"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/server/egin"
	"github.com/spf13/viper"
)

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

func InitSSHHandler(cfg Config, resolver plugin.ContextResolver) *sshweb.Handler {
	sshCfg := cfg.SSHConfig()
	sshCfg.Resolver = resolver
	return sshweb.NewHandler(sshCfg)
}

func InitWebServer(
	mdls []gin.HandlerFunc,
	policySDK *sdk.SDK,
	syncer capability.Syncer,
	providers []capability.PermissionProvider,
	sshHdl *sshweb.Handler,
	listener net.Listener,
) *egin.Component {
	server := egin.DefaultContainer().Build(egin.WithListener(listener))
	server.Engine.ContextWithFallback = true
	server.Use(mdls...)

	sshHdl.PublicRoutes(server.Engine)

	server.Use(policySDK.CheckLogin())
	server.Use(policySDK.CheckPolicy())

	sshHdl.PrivateRoutes(server.Engine)

	go func() {
		time.Sleep(time.Second)
		if err := syncer.WithOption(
			capability.WithPermissions(providers...),
			capability.WithRouter(server.Engine),
		).Sync(context.Background()); err != nil {
			elog.Error("EIAM 资产注册控制器启动失败", elog.FieldErr(err))
		}
	}()

	return server
}

func InitGinMiddlewares() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.AccessLogger(),
		middleware.NewCorsBuilder().Build(),
	}
}
