package bootstrap

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	common_grpc "github.com/Duke1616/ecmdb-plugins/pkg/grpc"
	"github.com/Duke1616/ecmdb/pkg/plugin"
	"github.com/Duke1616/eiam/pkg/web/capability"
	"github.com/Duke1616/eiam/pkg/web/middleware"
	"github.com/Duke1616/eiam/pkg/web/sdk"
	"github.com/ecodeclub/ekit/retry"
	"github.com/gin-gonic/gin"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/server/egin"
)

// IPlugin 定义通用插件契约接口
type IPlugin interface {
	ID() string                                    // 插件唯一标识符，例如 builtin.ssh
	Name() string                                  // 插件名称，例如 ssh
	Definition() (plugin.Definition, error)        // 插件对应的自描述信息
	RegisterPrivateRoutes(router *gin.RouterGroup) // 挂载业务私有路由
}

type BootstrapOptions struct {
	Plugin              IPlugin
	Resolver            *common_grpc.Resolver
	Upstream            string // 插件对外注册的反代物理地址，如 http://127.0.0.1:18080
	StaticDist          string // 静态文件目录物理路径，例如 "./plugins/ssh/frontend/dist"
	Listener            net.Listener
	PermissionProviders []capability.PermissionProvider
}

// PluginApp 统一包装插件生命周期的公共 App 容器
type PluginApp struct {
	Web      *egin.Component
	Resolver *common_grpc.Resolver
	pluginID string
	upstream string
}

// NewPluginApp 一键实例化并自动装配插件 Web 容器及依赖
func NewPluginApp(opt BootstrapOptions) *PluginApp {
	server := egin.DefaultContainer().Build(egin.WithListener(opt.Listener))
	server.Engine.ContextWithFallback = true

	// 1. 注入通用的 AccessLogger 与 CORS 中间件
	server.Use(middleware.AccessLogger(), middleware.NewCorsBuilder().Build())

	// 2. 自动注册公共声明端点 (自描述与其健康检查)
	server.Engine.GET(plugin.WellKnownPath, gin.WrapH(plugin.DefinitionHandler(opt.Plugin)))
	server.Engine.GET("/healthz", func(ctx *gin.Context) {
		ctx.PureJSON(http.StatusOK, gin.H{
			"status":    "ok",
			"plugin_id": opt.Plugin.ID(),
		})
	})

	// 3. 自动托管静态资源目录
	if opt.StaticDist != "" {
		server.Engine.Static("/static", opt.StaticDist)
	}

	// 4. 配置 EIAM 权限模块前缀并启用鉴权中间件
	policySDK := sdk.NewSDK()
	policySDK.WithPathPrefix("/api/plugin-runtime/" + opt.Plugin.ID())
	server.Use(policySDK.CheckLogin())
	server.Use(policySDK.CheckPolicy())

	// 5. 注册具体的私有业务逻辑路由
	opt.Plugin.RegisterPrivateRoutes(server.Engine.Group("/"))

	// 6. 异步向 EIAM 动态同步资产策略权限定义
	syncer := capability.NewSyncer(capability.NewHttpReporter())
	go func() {
		time.Sleep(time.Second)
		if err := syncer.WithOption(
			capability.WithSource(opt.Plugin.ID()),
			capability.WithAPIPathPrefix("/api/plugin-runtime/"+opt.Plugin.ID()),
			capability.WithPermissions(opt.PermissionProviders...),
			capability.WithRouter(server.Engine),
		).Sync(context.Background()); err != nil {
			elog.Error("EIAM 资产注册同步失败", elog.FieldErr(err))
		}
	}()

	resolver := opt.Resolver

	return &PluginApp{
		Web:      server,
		Resolver: resolver,
		pluginID: opt.Plugin.ID(),
		upstream: opt.Upstream,
	}
}

// Register 执行统一的主动动态握手注册（利用 ekit 重试策略，重试 3 次）
func (a *PluginApp) Register(ctx context.Context) error {
	if a.upstream == "" {
		elog.Warn("upstream 未配置，跳过动态握手注册，请检查配置文件")
		return nil
	}

	strategy, err := retry.NewFixedIntervalRetryStrategy(time.Second, 3)
	if err != nil {
		return fmt.Errorf("创建重试策略失败: %w", err)
	}

	err = retry.Retry(ctx, strategy, func() error {
		regErr := a.Resolver.RegisterPlugin(ctx, a.pluginID, a.upstream)
		if regErr != nil {
			elog.Warn("动态注册插件失败，准备重试", elog.FieldErr(regErr))
			return regErr
		}
		elog.Info("动态注册插件元数据成功", elog.String("plugin_id", a.pluginID), elog.String("upstream", a.upstream))
		return nil
	})

	if err != nil {
		return fmt.Errorf("动态注册插件在重试后依然失败: %w", err)
	}

	return nil
}

