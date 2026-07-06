package ioc

import (
	"context"
	"fmt"
	"time"

	common_grpc "github.com/Duke1616/ecmdb-plugins/pkg/grpc"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/server/egin"
	"github.com/spf13/viper"
)

type App struct {
	Web      *egin.Component
	Resolver *common_grpc.Resolver
}

// Register 负责在服务启动后自主向主站注册自身元数据，内置重试以应对 Web 端口尚未就绪的问题
func (a *App) Register(ctx context.Context) error {
	upstream := viper.GetString("ssh.upstream")
	if upstream == "" {
		elog.Warn("ssh.upstream 没配置，跳过向主站自动发起注册，请在配置文件中指定")
		return nil
	}

	// 插件唯一标识符
	const pluginID = "builtin.ssh"

	// 动态重试逻辑，以应对 Web 服务端口就绪存在毫秒级延迟的情况
	var err error
	for i := 0; i < 3; i++ {
		time.Sleep(1 * time.Second)
		err = a.Resolver.RegisterPlugin(ctx, pluginID, upstream)
		if err == nil {
			elog.Info("自动向主站发送 RPC 动态注册元数据请求成功", elog.String("upstream", upstream))
			return nil
		}
		elog.Warn("自动向主站注册插件元数据尝试失败，正在准备重试", elog.FieldErr(err), elog.Int("attempt", i+1))
	}

	return fmt.Errorf("在重试后依然注册失败: %w", err)
}
