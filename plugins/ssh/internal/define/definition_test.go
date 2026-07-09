package define

import (
	"testing"

	"github.com/Duke1616/ecmdb-plugins/pkg/bootstrap"
	"github.com/Duke1616/ecmdb/pkg/plugin"
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

	runtime, ok := def.Plugin.Runtime()
	if !ok {
		t.Fatal("runtime not found")
	}
	if runtime.Mode != plugin.RuntimeModeExternalService || runtime.Upstream != "http://ssh-plugin:8080" {
		t.Fatalf("runtime = %#v", runtime)
	}
	if len(def.Schema.Models) != 2 {
		t.Fatalf("models = %#v", def.Schema.Models)
	}
	if len(def.Bindings) != 1 || def.Bindings[0].Graph == nil {
		t.Fatalf("bindings = %#v", def.Bindings)
	}
}
