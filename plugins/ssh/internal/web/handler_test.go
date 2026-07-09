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

func TestStoreSessionKeepsConcurrentSessions(t *testing.T) {
	t.Parallel()

	h := NewHandler(bootstrap.PluginConfig{Upstream: "http://ssh-plugin:8080"})
	first := &stubSession{}
	second := &stubSession{}

	firstSess, err := h.sessions.Put(first)
	if err != nil {
		t.Fatal(err)
	}
	secondSess, err := h.sessions.Put(second)
	if err != nil {
		t.Fatal(err)
	}

	firstID := firstSess.token
	secondID := secondSess.token

	if firstID == secondID {
		t.Fatalf("session ids should be unique, got %s", firstID)
	}
	if first.closed != 0 {
		t.Fatalf("first session should not be closed, close count = %d", first.closed)
	}

	storedFirst, err := h.sessions.Get(firstID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if storedFirst.session != first {
		t.Fatal("first session was not stored")
	}

	storedSecond, err := h.sessions.Get(secondID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if storedSecond.session != second {
		t.Fatal("second session was not stored")
	}
}

func TestCloseSessionCleansRuntimeState(t *testing.T) {
	t.Parallel()

	h := NewHandler(bootstrap.PluginConfig{Upstream: "http://ssh-plugin:8080"})
	session := &stubSession{}
	sessInfo, err := h.sessions.Put(session)
	if err != nil {
		t.Fatal(err)
	}
	h.finder.markReady(sessInfo.finderID)

	h.closeSession(sessInfo.token)

	if session.closed != 1 {
		t.Fatalf("session close count = %d, want 1", session.closed)
	}
	if h.finder.isReady(sessInfo.finderID) {
		t.Fatal("finder state should be cleared")
	}
	if _, err := h.sessions.Get(sessInfo.token); err == nil {
		t.Fatal("session should be deleted")
	}
}

func TestParseFinderSessionToken(t *testing.T) {
	t.Parallel()

	t.Run("header first", func(t *testing.T) {
		t.Parallel()
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		req := httptest.NewRequest(http.MethodGet, "/sftp/files?id=ssh_two", nil)
		req.Header.Set("x-finder-id", "ssh_nine")
		ctx.Request = req

		token, err := parseFinderSessionToken(ctx)
		if err != nil {
			t.Fatalf("parseFinderSessionToken() error = %v", err)
		}
		if token != "ssh_nine" {
			t.Fatalf("token = %s, want ssh_nine", token)
		}
	})

	t.Run("query fallback", func(t *testing.T) {
		t.Parallel()
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest(http.MethodGet, "/sftp/files?id=ssh_twentytwo", nil)

		token, err := parseFinderSessionToken(ctx)
		if err != nil {
			t.Fatalf("parseFinderSessionToken() error = %v", err)
		}
		if token != "ssh_twentytwo" {
			t.Fatalf("token = %s, want ssh_twentytwo", token)
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
