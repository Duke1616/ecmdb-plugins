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

// AuthType 表示 SSH 插件自描述的认证方式强类型枚举
type AuthType string

const (
	AuthTypePasswd     AuthType = "passwd"     // 密码认证
	AuthTypePublicKey  AuthType = "publickey"  // 免密公私钥认证
	AuthTypePassphrase AuthType = "passphrase" // 带口令的私钥认证
)

// EnumOptions 实现枚举自描述方法，供 ECMDB 模型内省识别为 list 预设下拉选项
func (AuthType) EnumOptions() []string {
	return []string{
		string(AuthTypePasswd),
		string(AuthTypePublicKey),
		string(AuthTypePassphrase),
	}
}

type Endpoint struct {
	model.BaseResource
	Host       string   `plugin:"host,field=ip,label=主机地址,required"`
	Port       int      `plugin:"port,label=SSH端口,default=22"`
	Username   string   `plugin:"username,label=登录账号,required"`
	Password   string   `plugin:"password,label=登录密码"`
	PrivateKey []byte   `plugin:"private_key,label=私钥凭证"`
	AuthType   AuthType `plugin:"auth_type,label=认证方式,default=passwd"`
	Sort       int      `plugin:"sort,label=排序权重"`
}

type Gateway struct {
	model.BaseResource
	Host       string   `plugin:"host,label=网关地址,required"`
	Port       int      `plugin:"port,label=网关端口,default=22"`
	Username   string   `plugin:"username,label=网关账号,required"`
	Password   string   `plugin:"password,label=网关密码"`
	PrivateKey []byte   `plugin:"private_key,label=网关私钥"`
	AuthType   AuthType `plugin:"auth_type,label=认证方式,default=passwd"`
	Sort       int      `plugin:"sort,label=排序权重"`
}

type ConnectionTarget struct {
	Endpoint
	Gateways []Gateway `plugin:"gateways,model=AuthGateway,name=跳板机网关,group=安全凭据,in=default"`
}

func (ConnectionTarget) DescribeModel() (string, string) {
	return "主机资产", "计算资源"
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

func toEndpoint(host string, port int, username, password string, privateKey []byte, authType AuthType, sort int) term.Endpoint {
	return term.Endpoint{
		Host:       host,
		Port:       port,
		Username:   username,
		Password:   password,
		PrivateKey: privateKey,
		AuthType:   string(lo.CoalesceOrEmpty(authType, AuthTypePasswd)),
		Passphrase: password,
		Sort:       sort,
	}
}
