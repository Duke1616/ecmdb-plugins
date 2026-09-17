package define

import (
	"testing"

	"github.com/Duke1616/ecmdb-plugins/pkg/bootstrap"
	"github.com/Duke1616/ecmdb-plugins/pkg/term"
	"github.com/Duke1616/ecmdb/pkg/plugin/types"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefinition(t *testing.T) {
	def, err := NewProvider(bootstrap.PluginConfig{Upstream: "http://ssh-plugin:8080"}).Definition()
	require.NoError(t, err)

	t.Run("插件基础信息与运行时", func(t *testing.T) {
		assert.Equal(t, PluginUID, def.Plugin.UID)
		assert.Equal(t, "SSH", def.Plugin.Name)

		runtime, ok := def.Plugin.Runtime()
		require.True(t, ok)
		assert.Equal(t, types.RuntimeModeExternalService, runtime.Mode)
		assert.Equal(t, "http://ssh-plugin:8080", runtime.Upstream)
	})

	t.Run("动作与工作区配置", func(t *testing.T) {
		require.Len(t, def.Plugin.Actions, 2)

		termAction := def.Plugin.Actions[0]
		assert.Equal(t, ActionTerminal, termAction.Action)
		assert.Equal(t, PermissionConnect, termAction.Permission)
		assert.Equal(t, "workspace", termAction.Runtime.Layout)
		assert.Equal(t, "Web Shell", termAction.Runtime.Props["connectionType"])
		assert.Equal(t, "builtin.ssh.host", termAction.BindingUID)

		sftpAction := def.Plugin.Actions[1]
		assert.Equal(t, ActionSFTP, sftpAction.Action)
		assert.Equal(t, PermissionConnect, sftpAction.Permission)
		assert.Equal(t, "Web Sftp", sftpAction.Runtime.Props["connectionType"])
		assert.Equal(t, "builtin.ssh.host", sftpAction.BindingUID) // 复用终端绑定 UID
	})

	t.Run("CMDB 资产模型推导", func(t *testing.T) {
		require.Len(t, def.Schema.Models, 2)

		hostModel, ok := lo.Find(def.Schema.Models, func(m types.ModelSpec) bool { return m.UID == ModelHost })
		require.True(t, ok)
		assert.Equal(t, "主机资产", hostModel.Name)
		assert.Equal(t, "计算资源", hostModel.GroupName)

		gatewayModel, ok := lo.Find(def.Schema.Models, func(m types.ModelSpec) bool { return m.UID == ModelAuthGateway })
		require.True(t, ok)
		assert.Equal(t, "跳板机网关", gatewayModel.Name)
		assert.Equal(t, "安全凭据", gatewayModel.GroupName)

		// 验证网关模型属性推导
		gwFields := gatewayModel.AttributeGroups[0].Fields

		// 1. private_key 自动推导为 multiline 且安全加密
		pkField, ok := lo.Find(gwFields, func(a types.Attribute) bool { return a.UID == "private_key" })
		require.True(t, ok)
		assert.Equal(t, "multiline", pkField.Type)
		assert.True(t, pkField.Secure)
		assert.False(t, pkField.Display)

		// 2. auth_type 通过 IEnum 自动推导为 list 且包含所有枚举选项
		authField, ok := lo.Find(gwFields, func(a types.Attribute) bool { return a.UID == "auth_type" })
		require.True(t, ok)
		assert.Equal(t, "list", authField.Type)
		assert.Equal(t, []string{"passwd", "publickey", "passphrase"}, authField.Option)
	})

	t.Run("拓扑关联与绑定图谱", func(t *testing.T) {
		assert.Len(t, def.Schema.ModelGroups, 2)
		require.Len(t, def.Schema.ModelRelations, 1)

		rel := def.Schema.ModelRelations[0]
		assert.Equal(t, ModelAuthGateway, rel.SourceModelUID)
		assert.Equal(t, ModelHost, rel.TargetModelUID)
		assert.Equal(t, types.RelationTypeDefault, rel.RelationTypeUID)

		require.Len(t, def.Bindings, 1)
		assert.NotNil(t, def.Bindings[0].Graph)
	})
}

func TestDecodeTarget(t *testing.T) {
	actionCtx := types.ActionContext{
		Inputs: map[string]types.ResolvedInput{
			"target": {
				Name:        "target",
				Cardinality: types.CardinalityOne,
				Resources: []types.ResolvedResource{
					{
						Fields: map[string]any{
							"host":        "192.168.1.100",
							"port":        22,
							"username":    "root",
							"password":    "secret_password",
							"private_key": "target_private_key",
							"auth_type":   "publickey",
						},
						Children: map[string]types.ResolvedInput{
							"gateways": {
								Name:        "gateways",
								Cardinality: types.CardinalityMany,
								Resources: []types.ResolvedResource{
									{
										Fields: map[string]any{
											"host":        "1.1.1.1",
											"port":        2222,
											"username":    "jump",
											"password":    "jump_password",
											"private_key": "jump_private_key",
											"auth_type":   "passphrase",
											"sort":        1,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	target, err := DecodeTarget(actionCtx)
	require.NoError(t, err)

	t.Run("解码为强类型目标结构体", func(t *testing.T) {
		assert.Equal(t, "192.168.1.100", target.Host)
		assert.Equal(t, 22, target.Port)
		assert.Equal(t, "root", target.Username)
		assert.Equal(t, "secret_password", target.Password)
		assert.Equal(t, []byte("target_private_key"), target.PrivateKey)
		assert.Equal(t, AuthTypePublicKey, target.AuthType)

		require.Len(t, target.Gateways, 1)
		gw := target.Gateways[0]
		assert.Equal(t, "1.1.1.1", gw.Host)
		assert.Equal(t, 2222, gw.Port)
		assert.Equal(t, "jump", gw.Username)
		assert.Equal(t, []byte("jump_private_key"), gw.PrivateKey)
		assert.Equal(t, AuthTypePassphrase, gw.AuthType)
		assert.Equal(t, 1, gw.Sort)
	})

	t.Run("转换为多级网关跳转链路", func(t *testing.T) {
		chain := target.ToGatewayChain()
		require.Len(t, chain, 2)

		expectedChain := term.GatewayChain{
			{
				Host:       "1.1.1.1",
				Port:       2222,
				Username:   "jump",
				Password:   "jump_password",
				PrivateKey: []byte("jump_private_key"),
				AuthType:   "passphrase",
				Passphrase: "jump_password",
				Sort:       1,
			},
			{
				Host:       "192.168.1.100",
				Port:       22,
				Username:   "root",
				Password:   "secret_password",
				PrivateKey: []byte("target_private_key"),
				AuthType:   "publickey",
				Passphrase: "secret_password",
				Sort:       2,
			},
		}

		assert.Equal(t, expectedChain, chain)
	})
}
