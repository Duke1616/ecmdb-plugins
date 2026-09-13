package define

import (
	"testing"

	"github.com/Duke1616/ecmdb-plugins/pkg/bootstrap"
	"github.com/Duke1616/ecmdb/pkg/plugin/types"
)

func TestDefinition(t *testing.T) {
	def, err := NewProvider(bootstrap.PluginConfig{Upstream: "http://ssh-plugin:8080"}).Definition()
	if err != nil {
		t.Fatalf("Definition() error = %v", err)
	}

	if def.Plugin.UID != PluginUID {
		t.Fatalf("plugin uid = %s", def.Plugin.UID)
	}
	if len(def.Plugin.Actions) != 2 {
		t.Fatalf("actions = %#v", def.Plugin.Actions)
	}
	if def.Plugin.Actions[0].Runtime == nil {
		t.Fatal("terminal runtime not found")
	}
	if def.Plugin.Actions[0].Permission != PermissionConnect {
		t.Fatalf("unexpected terminal permission: %s", def.Plugin.Actions[0].Permission)
	}
	if def.Plugin.Actions[0].Runtime.Layout != "workspace" {
		t.Fatalf("unexpected terminal layout: %s", def.Plugin.Actions[0].Runtime.Layout)
	}
	if got := def.Plugin.Actions[0].Runtime.Props["connectionType"]; got != "Web Shell" {
		t.Fatalf("unexpected terminal connectionType: %v", got)
	}
	if _, ok := def.Plugin.Actions[0].Runtime.Props["autoConnect"]; ok {
		t.Fatal("terminal action should not auto connect")
	}
	if def.Plugin.Actions[1].Runtime == nil {
		t.Fatal("sftp runtime not found")
	}
	if def.Plugin.Actions[1].Permission != PermissionConnect {
		t.Fatalf("unexpected sftp permission: %s", def.Plugin.Actions[1].Permission)
	}
	if got := def.Plugin.Actions[1].Runtime.Props["connectionType"]; got != "Web Sftp" {
		t.Fatalf("unexpected sftp connectionType: %v", got)
	}
	if _, ok := def.Plugin.Actions[1].Runtime.Props["autoConnect"]; ok {
		t.Fatal("sftp action should not auto connect")
	}

	// 验证绑定复用：sftp 动作自动复用 terminal 的 binding_uid
	expectedBindingUID := "builtin.ssh.host"
	if def.Plugin.Actions[0].BindingUID != expectedBindingUID {
		t.Fatalf("terminal binding_uid = %s, want %s", def.Plugin.Actions[0].BindingUID, expectedBindingUID)
	}
	if def.Plugin.Actions[1].BindingUID != expectedBindingUID {
		t.Fatalf("sftp reuse binding_uid = %s, want %s", def.Plugin.Actions[1].BindingUID, expectedBindingUID)
	}

	runtime, ok := def.Plugin.Runtime()
	if !ok {
		t.Fatal("runtime not found")
	}
	if runtime.Mode != types.RuntimeModeExternalService || runtime.Upstream != "http://ssh-plugin:8080" {
		t.Fatalf("runtime = %#v", runtime)
	}

	// 验证无需写 Setup，CMDB 自动从 Bind 推导出父子两层模型与各自的中文名/分组
	if len(def.Schema.Models) != 2 {
		t.Fatalf("models = %#v", def.Schema.Models)
	}
	var hostModel, gatewayModel *types.ModelSpec
	for i := range def.Schema.Models {
		if def.Schema.Models[i].UID == ModelHost {
			hostModel = &def.Schema.Models[i]
		} else if def.Schema.Models[i].UID == ModelAuthGateway {
			gatewayModel = &def.Schema.Models[i]
		}
	}
	if hostModel == nil || hostModel.Name != "主机资产" || hostModel.GroupName != "计算资源" {
		t.Fatalf("host model = %#v", hostModel)
	}
	if gatewayModel == nil || gatewayModel.Name != "跳板机网关" || gatewayModel.GroupName != "安全凭据" {
		t.Fatalf("gateway model = %#v", gatewayModel)
	}

	// 验证分组自动收集
	if len(def.Schema.ModelGroups) != 2 {
		t.Fatalf("model groups count = %d, want 2", len(def.Schema.ModelGroups))
	}

	// 验证模型关联拓扑自动构建
	if len(def.Schema.ModelRelations) != 1 {
		t.Fatalf("model relations = %#v", def.Schema.ModelRelations)
	}

	// 验证 Binding 图谱正确
	if len(def.Bindings) != 1 || def.Bindings[0].Graph == nil {
		t.Fatalf("bindings = %#v", def.Bindings)
	}
}
