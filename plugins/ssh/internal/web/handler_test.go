package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Duke1616/ecmdb-plugins/plugins/ssh/internal/config"
	"github.com/Duke1616/ecmdb-plugins/plugins/ssh/internal/define"
	"github.com/Duke1616/ecmdb/pkg/plugin"
	"github.com/gin-gonic/gin"
)

func TestWellKnown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler := NewHandler(config.Config{Upstream: "http://ssh-plugin:8080"})
	handler.PublicRoutes(engine)

	req := httptest.NewRequest(http.MethodGet, plugin.WellKnownPath, nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var def plugin.Definition
	if err := json.Unmarshal(rec.Body.Bytes(), &def); err != nil {
		t.Fatalf("decode definition: %v", err)
	}
	if def.Plugin.UID != define.PluginUID {
		t.Fatalf("plugin uid = %s", def.Plugin.UID)
	}
}
