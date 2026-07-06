package grpc

import (
	"context"
	"encoding/json"

	pluginv1 "github.com/Duke1616/ecmdb-plugins/api/proto/gen/ecmdb/plugin/v1"
	"github.com/Duke1616/ecmdb/pkg/plugin"
)

type Resolver struct {
	client pluginv1.PluginRuntimeServiceClient
}

func NewResolver(client pluginv1.PluginRuntimeServiceClient) *Resolver {
	return &Resolver{client: client}
}

// ResolveActionContext 通过 gRPC 客户端发起请求到主站，解析具体的 action 运行上下文（全插件共享公共客户端）
func (r *Resolver) ResolveActionContext(ctx context.Context, req plugin.ResolveRequest) (plugin.ActionContext, error) {
	resp, err := r.client.ResolveActionContext(ctx, &pluginv1.ResolveActionContextRequest{
		PluginId:   req.PluginID,
		Action:     req.Action,
		ResourceId: req.ResourceID,
	})
	if err != nil {
		return plugin.ActionContext{}, err
	}

	var actionCtx plugin.ActionContext
	if err := json.Unmarshal(resp.ActionContextJson, &actionCtx); err != nil {
		return plugin.ActionContext{}, err
	}

	return actionCtx, nil
}

// RegisterPlugin 发起 gRPC 客户端握手，向主站注册插件及 upstream 访问地址
func (r *Resolver) RegisterPlugin(ctx context.Context, pluginID, upstream string) error {
	_, err := r.client.RegisterPlugin(ctx, &pluginv1.RegisterPluginRequest{
		PluginId: pluginID,
		Upstream: upstream,
	})
	return err
}
