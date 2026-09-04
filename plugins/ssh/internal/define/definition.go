package define

import (
	"slices"

	"github.com/samber/lo"

	"github.com/Duke1616/ecmdb-plugins/pkg/bootstrap"
	"github.com/Duke1616/ecmdb/pkg/plugin"
	"github.com/Duke1616/ecmdb/pkg/plugin/codec"
	"github.com/Duke1616/ecmdb/pkg/plugin/types"
	"github.com/Duke1616/ecmdb/pkg/term"
)

const (
	PluginUID      = "builtin.ssh"
	ActionTerminal = "terminal"
	ActionSFTP     = "sftp"

	PermissionConnect = "cmdb:ssh:connect"

	inputEndpoint = "endpoint"
)

type Provider struct {
	cfg bootstrap.PluginConfig
}

func NewProvider(cfg bootstrap.PluginConfig) Provider {
	return Provider{cfg: cfg}
}

func (p Provider) Definition() (plugin.Definition, error) {
	hostBindingUID := plugin.CenterBindingUID(PluginUID, "host")
	return plugin.NewRegistry(
		PluginUID,
		"SSH",
		plugin.Type("builtin"),
		plugin.Version("1.0.1"),
		plugin.Description("基于 CMDB 主机和登录网关关系提供 SSH 终端与 SFTP 文件管理能力。"),
		plugin.ExternalServiceRuntime(p.cfg.Upstream, plugin.RuntimeHealthPath("/healthz")),
	).
		Action(
			ActionTerminal,
			"SSH 终端",
			plugin.Icon("terminal"),
			plugin.Permission(PermissionConnect),
			plugin.Workspace("Web Shell", "host",
				plugin.CardFields("name", "ip"),
				plugin.Prop("connectionType", "Web Shell"),
			),
			plugin.UseBinding(hostBindingUID),
		).
		Action(
			ActionSFTP,
			"文件管理",
			plugin.Icon("folder"),
			plugin.Permission(PermissionConnect),
			plugin.Workspace("Web Sftp", "host",
				plugin.CardFields("name", "ip"),
				plugin.Prop("connectionType", "Web Sftp"),
			),
			plugin.UseBinding(hostBindingUID),
		).
		Setup(
			plugin.Derive[ConnectionTarget]("host"),
		).
		Bind(plugin.CenterNamed[ConnectionTarget](inputEndpoint, "host")).
		Definition()
}

type Endpoint struct {
	Host       string `plugin:"host,field=ip,required"`
	Port       int    `plugin:"port,default=22"`
	Username   string `plugin:"username,required"`
	Password   string `plugin:"password"`
	PrivateKey string `plugin:"private_key"`
	AuthType   string `plugin:"auth_type"`
	Sort       int    `plugin:"sort"`
}

type Gateway struct {
	Host       string `plugin:"host,field=host,required"`
	Port       int    `plugin:"port,default=22"`
	Username   string `plugin:"username,required"`
	Password   string `plugin:"password"`
	PrivateKey string `plugin:"private_key"`
	AuthType   string `plugin:"auth_type"`
	Sort       int    `plugin:"sort"`
}

type ConnectionTarget struct {
	Endpoint
	Gateways []Gateway `plugin:"gateways,model=AuthGateway,in=default"`
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
