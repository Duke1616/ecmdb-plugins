package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Duke1616/ecmdb-plugins/pkg/bootstrap"
	"github.com/Duke1616/ecmdb-plugins/plugins/ssh/internal/define"
	"github.com/Duke1616/ecmdb/pkg/plugin"
	"github.com/Duke1616/ecmdb/pkg/term"
	"github.com/gin-gonic/gin"
)

func TestWellKnown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler := NewHandler(bootstrap.PluginConfig{Upstream: "http://ssh-plugin:8080"})
	engine.GET(plugin.WellKnownPath, gin.WrapH(plugin.DefinitionHandler(handler)))

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

func TestConnectTypeSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		typ         ConnectType
		wantAction  string
		wantMessage string
		wantErr     bool
	}{
		{
			name:        "web shell",
			typ:         ConnectTypeSSH,
			wantAction:  define.ActionTerminal,
			wantMessage: "SSH 连接成功",
		},
		{
			name:        "web sftp",
			typ:         ConnectTypeWebSftp,
			wantAction:  define.ActionSFTP,
			wantMessage: "SFTP 连接成功",
		},
		{
			name:    "rdp unsupported",
			typ:     ConnectTypeRDP,
			wantErr: true,
		},
		{
			name:    "unknown unsupported",
			typ:     ConnectType("unknown"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spec, err := tc.typ.spec()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("spec() error = %v", err)
			}
			if spec.action != tc.wantAction {
				t.Fatalf("action = %s, want %s", spec.action, tc.wantAction)
			}
			if spec.successMsg != tc.wantMessage {
				t.Fatalf("message = %s, want %s", spec.successMsg, tc.wantMessage)
			}
		})
	}
}

func TestReplaceSessionClosesPreviousSession(t *testing.T) {
	t.Parallel()

	h := NewHandler(bootstrap.PluginConfig{Upstream: "http://ssh-plugin:8080"})
	resourceID := int64(101)
	prev := &stubSession{}
	next := &stubSession{}

	h.session.SetSession(resourceID, prev)
	h.finder.markReady(resourceID)

	h.replaceSession(resourceID, next)

	if prev.closed != 1 {
		t.Fatalf("previous session close count = %d, want 1", prev.closed)
	}
	if h.finder.isReady(resourceID) {
		t.Fatal("finder state should be cleared when replacing session")
	}

	stored, err := h.session.GetSession(resourceID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if stored != next {
		t.Fatal("stored session was not replaced")
	}
}

func TestParseFinderResourceID(t *testing.T) {
	t.Parallel()

	t.Run("header first", func(t *testing.T) {
		t.Parallel()
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		req := httptest.NewRequest(http.MethodGet, "/sftp/files?id=2", nil)
		req.Header.Set("x-finder-id", "9")
		ctx.Request = req

		id, err := parseFinderResourceID(ctx)
		if err != nil {
			t.Fatalf("parseFinderResourceID() error = %v", err)
		}
		if id != 9 {
			t.Fatalf("id = %d, want 9", id)
		}
	})

	t.Run("query fallback", func(t *testing.T) {
		t.Parallel()
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest(http.MethodGet, "/sftp/files?id=22", nil)

		id, err := parseFinderResourceID(ctx)
		if err != nil {
			t.Fatalf("parseFinderResourceID() error = %v", err)
		}
		if id != 22 {
			t.Fatalf("id = %d, want 22", id)
		}
	})
}

type stubSession struct {
	closed int
}

func (s *stubSession) Protocol() string {
	return "ssh"
}

func (s *stubSession) Close() error {
	s.closed++
	return nil
}

func (s *stubSession) Transport() term.Transport {
	return nil
}
