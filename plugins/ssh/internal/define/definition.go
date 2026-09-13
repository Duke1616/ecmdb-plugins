package define

import (
	"slices"

	"github.com/Duke1616/ecmdb-plugins/pkg/bootstrap"
	"github.com/Duke1616/ecmdb-plugins/pkg/model"
	"github.com/Duke1616/ecmdb-plugins/pkg/term"
	"github.com/Duke1616/ecmdb/pkg/plugin"
	"github.com/Duke1616/ecmdb/pkg/plugin/codec"
	"github.com/Duke1616/ecmdb/pkg/plugin/types"
	"github.com/samber/lo"
)

const (
	PluginUID      = "builtin.ssh"
	ActionTerminal = "terminal"
	ActionSFTP     = "sftp"

	PermissionConnect = "cmdb:ssh:connect"

	// 核心模型唯一标识符
	ModelHost        = "host"
	ModelAuthGateway = "AuthGateway"
)

type Provider struct {
	cfg bootstrap.PluginConfig
}

func NewProvider(cfg bootstrap.PluginConfig) Provider {
	return Provider{cfg: cfg}
}

func (p Provider) Definition() (plugin.Definition, error) {
	reg := plugin.NewRegistry(
		PluginUID,
		"SSH",
		plugin.Type("builtin"),
		plugin.Version("1.0.1"),
		plugin.Description("基于 CMDB 主机和登录网关关系提供 SSH 终端与 SFTP 文件管理能力。"),
		plugin.ExternalServiceRuntime(p.cfg.Upstream, plugin.RuntimeHealthPath("/healthz")),
	)

	return plugin.Target[ConnectionTarget](reg, ModelHost).
		Workspace(
			ActionTerminal,
			"Web Shell",
			plugin.Icon("terminal"),
			plugin.Permission(PermissionConnect),
			plugin.CardFields("name", "ip"),
			plugin.Prop("connectionType", "Web Shell"),
		).
		Workspace(
			ActionSFTP,
			"Web Sftp",
			plugin.Icon("folder"),
			plugin.Permission(PermissionConnect),
			plugin.CardFields("name", "ip"),
			plugin.Prop("connectionType", "Web Sftp"),
		).
		Definition()
}

type Endpoint struct {
	model.BaseResource
	Host       string `plugin:"host,label=主机地址,field=ip,required"`
	Port       int    `plugin:"port,label=SSH端口,default=22"`
	Username   string `plugin:"username,label=登录账号,required"`
	Password   string `plugin:"password,label=登录密码"`
	PrivateKey string `plugin:"private_key,label=私钥凭证"`
	AuthType   string `plugin:"auth_type,label=认证方式"`
	Sort       int    `plugin:"sort,label=排序权重"`
}

type Gateway struct {
	model.BaseResource
	Host       string `plugin:"host,label=网关地址,field=host,required"`
	Port       int    `plugin:"port,label=网关端口,default=22"`
	Username   string `plugin:"username,label=网关账号,required"`
	Password   string `plugin:"password,label=网关密码"`
	PrivateKey string `plugin:"private_key,label=网关私钥"`
	AuthType   string `plugin:"auth_type,label=认证方式"`
	Sort       int    `plugin:"sort,label=排序权重"`
}

type ConnectionTarget struct {
	Endpoint `plugin:",label=主机资产,group=计算资源"`
	Gateways []Gateway `plugin:"gateways,model=AuthGateway,name=跳板机网关,group=安全凭据,in=default"`
}

func DecodeTarget(actionCtx types.ActionContext) (ConnectionTarget, error) {
	return codec.InputRootOne[ConnectionTarget](actionCtx)
}

func ResolveRequest(action string, resourceID int64) types.ResolveRequest {
	return types.ResolveRequest{
		PluginID:   PluginUID,
		Action:     action,
		ResourceID: resourceID,
	}
}

func ResolveGatewayChain(actionCtx types.ActionContext) (term.GatewayChain, error) {
	target, err := DecodeTarget(actionCtx)
	if err != nil {
		return nil, err
	}
	return target.ToGatewayChain(), nil
}

func (t ConnectionTarget) ToGatewayChain() term.GatewayChain {
	gateways := append([]Gateway(nil), t.Gateways...)
	// NOTE: 使用 slices.SortFunc 进行稳定排序，并使用 lo.Map 投影转换为 term.Endpoint 列表
	slices.SortFunc(gateways, func(a, b Gateway) int {
		return a.Sort - b.Sort
	})

	chain := lo.Map(gateways, func(g Gateway, _ int) term.Endpoint {
		return g.ToEndpoint()
	})

	target := t.Endpoint.ToEndpoint()
	target.Sort = len(chain) + 1
	return append(chain, target)
}

func (e Endpoint) ToEndpoint() term.Endpoint {
	return toEndpoint(e.Host, e.Port, e.Username, e.Password, e.PrivateKey, e.AuthType, e.Sort)
}

func (g Gateway) ToEndpoint() term.Endpoint {
	return toEndpoint(g.Host, g.Port, g.Username, g.Password, g.PrivateKey, g.AuthType, g.Sort)
}

func toEndpoint(host string, port int, username, password, privateKey, authType string, sort int) term.Endpoint {
	return term.Endpoint{
		Host:       host,
		Port:       port,
		Username:   username,
		Password:   password,
		PrivateKey: privateKey,
		AuthType:   lo.CoalesceOrEmpty(authType, "passwd"),
		Passphrase: password,
		Sort:       sort,
	}
}
