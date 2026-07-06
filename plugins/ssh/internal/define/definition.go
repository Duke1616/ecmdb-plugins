package define

import (
	"sort"

	"github.com/Duke1616/ecmdb-plugins/plugins/ssh/internal/config"
	"github.com/Duke1616/ecmdb/pkg/plugin"
	"github.com/Duke1616/ecmdb/pkg/term"
)

const (
	PluginUID      = "builtin.ssh"
	ActionTerminal = "terminal"
	ActionSFTP     = "sftp"

	inputEndpoint = "endpoint"
)

type Provider struct {
	cfg config.Config
}

func NewProvider(cfg config.Config) Provider {
	return Provider{cfg: cfg}
}

func (p Provider) Definition() (plugin.Definition, error) {
	return plugin.NewRegistry(
		PluginUID,
		"SSH",
		plugin.Type("builtin"),
		plugin.Version("1.0.0"),
		plugin.Description("基于 CMDB 主机和登录网关关系提供 SSH 终端与 SFTP 文件管理能力。"),
		plugin.ExternalServiceRuntime(p.cfg.Upstream, plugin.RuntimeHealthPath("/healthz")),
	).
		Action(
			ActionTerminal,
			"SSH 终端",
			plugin.Icon("terminal"),
			plugin.UI(plugin.UIBuiltinTerminal),
		).
		Action(
			ActionSFTP,
			"文件管理",
			plugin.Icon("folder"),
			plugin.UI(plugin.UIBuiltinSFTP),
		).
		Setup(
			plugin.ModelGroup("主机模型"),
			plugin.ModelGroup("网关模型"),
			plugin.RelationTypes(plugin.BasicRelationTypes()...),
			hostModel(),
			gatewayModel(),
			plugin.Relation("AuthGateway", plugin.RelationTypeDefault, "host").ManyToMany(),
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

func DecodeTarget(actionCtx plugin.ActionContext) (ConnectionTarget, error) {
	return plugin.InputRootOne[ConnectionTarget](actionCtx)
}

func ResolveRequest(action string, resourceID int64) plugin.ResolveRequest {
	return plugin.ResolveRequest{
		PluginID:   PluginUID,
		Action:     action,
		ResourceID: resourceID,
	}
}

func ResolveGatewayChain(actionCtx plugin.ActionContext) (term.GatewayChain, error) {
	target, err := DecodeTarget(actionCtx)
	if err != nil {
		return nil, err
	}
	return target.ToGatewayChain(), nil
}

func (t ConnectionTarget) ToGatewayChain() term.GatewayChain {
	sort.SliceStable(t.Gateways, func(i, j int) bool {
		return t.Gateways[i].Sort < t.Gateways[j].Sort
	})

	chain := make(term.GatewayChain, 0, len(t.Gateways)+1)
	for _, gateway := range t.Gateways {
		chain = append(chain, gateway.ToEndpoint())
	}

	target := t.Endpoint.ToEndpoint()
	target.Sort = len(chain) + 1
	chain = append(chain, target)
	return chain
}

func (e Endpoint) ToEndpoint() term.Endpoint {
	return toEndpoint(e.Host, e.Port, e.Username, e.Password, e.PrivateKey, e.AuthType, e.Sort)
}

func (g Gateway) ToEndpoint() term.Endpoint {
	return toEndpoint(g.Host, g.Port, g.Username, g.Password, g.PrivateKey, g.AuthType, g.Sort)
}

func toEndpoint(host string, port int, username, password, privateKey, authType string, sort int) term.Endpoint {
	if authType == "" {
		authType = "passwd"
	}

	return term.Endpoint{
		Host:       host,
		Port:       port,
		Username:   username,
		Password:   password,
		PrivateKey: privateKey,
		AuthType:   authType,
		Passphrase: password,
		Sort:       sort,
	}
}

func hostModel() plugin.ModelSpec {
	return plugin.Model(
		"host",
		"主机",
		plugin.ModelIcon("monitor-host"),
		plugin.ModelGroupName("主机模型"),
	).
		AttrGroup("基础属性", 0,
			plugin.String("name", "名称").Required().Display().Index(0),
			plugin.String("ip", "IP地址").Required().Display().Index(1),
			plugin.String("port", "端口").Display().Index(2),
			plugin.String("username", "用户名").Display().Index(3),
			plugin.List("auth_type", "认证类型", authOptions()).Required().Display().Index(6),
		).
		AttrGroup("加密属性", 2,
			plugin.String("password", "密码").Secure().Index(1),
			plugin.Multiline("private_key", "私钥").Secure().Index(2),
		).
		Build()
}

func gatewayModel() plugin.ModelSpec {
	return plugin.Model(
		"AuthGateway",
		"登陆网关",
		plugin.ModelIcon("ops-oneterm-login"),
		plugin.ModelGroupName("网关模型"),
	).
		AttrGroup("基础属性", 0,
			plugin.String("name", "名称").Required().Display().Index(0),
			plugin.String("host", "地址").Required().Display().Index(1),
			plugin.String("port", "端口").Display().Index(2),
			plugin.String("username", "用户名").Display().Index(3),
		).
		AttrGroup("分类属性", 1,
			plugin.List("auth_type", "认证类型", authOptions()).Display().Index(1),
			plugin.String("sort", "排序").Display().Index(2),
		).
		AttrGroup("加密属性", 2,
			plugin.String("password", "密码").Secure().Index(1),
			plugin.Multiline("private_key", "私钥").Secure().Index(2),
		).
		Build()
}

func authOptions() []string {
	return []string{"passwd", "publickey", "passphrase"}
}
