package define

import (
	"testing"

	"github.com/Duke1616/ecmdb-plugins/plugins/ssh/internal/config"
	"github.com/Duke1616/ecmdb/pkg/plugin"
)

func TestDefinition(t *testing.T) {
	def, err := NewProvider(config.Config{Upstream: "http://ssh-plugin:8080"}).Definition()
	if err != nil {
		t.Fatalf("Definition() error = %v", err)
	}

	if def.Plugin.UID != PluginUID {
		t.Fatalf("plugin uid = %s", def.Plugin.UID)
	}
	if len(def.Plugin.Actions) != 2 {
		t.Fatalf("actions = %#v", def.Plugin.Actions)
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
