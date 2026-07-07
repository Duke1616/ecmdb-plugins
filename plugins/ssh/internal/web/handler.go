package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Duke1616/ecmdb-plugins/pkg/bootstrap"
	"github.com/Duke1616/ecmdb-plugins/plugins/ssh/internal/define"
	_ "github.com/Duke1616/ecmdb-plugins/plugins/ssh/internal/ssh"
	"github.com/Duke1616/ecmdb/pkg/ginx"
	"github.com/Duke1616/ecmdb/pkg/plugin"
	"github.com/Duke1616/ecmdb/pkg/term"
	"github.com/Duke1616/ecmdb/pkg/term/sshx"
	"github.com/Duke1616/eiam/pkg/web/capability"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type missingResolver struct{}

func (missingResolver) ResolveActionContext(context.Context, plugin.ResolveRequest) (plugin.ActionContext, error) {
	return plugin.ActionContext{}, fmt.Errorf("ecmdb context resolver is not configured")
}

type Handler struct {
	provider define.Provider
	resolver plugin.ContextResolver
	session  *term.SessionPool
	timeout  time.Duration
	finder   *finderRuntime
	capability.IRegistry
}

func NewHandler(cfg bootstrap.PluginConfig) *Handler {
	resolver := cfg.Resolver
	if resolver == nil {
		resolver = missingResolver{}
	}

	timeout := 5 * time.Second
	if cfg.TimeoutSeconds > 0 {
		timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}

	return &Handler{
		provider:  define.NewProvider(cfg),
		resolver:  resolver,
		session:   term.NewSessionPool(),
		timeout:   timeout,
		finder:    newFinderRuntime(),
		IRegistry: bootstrap.NewRegistry("ssh", "资产仓库/SSH 插件"),
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	Subprotocols: []string{"guacamole"},
}

func (h *Handler) ID() string {
	return define.PluginUID
}

func (h *Handler) Name() string {
	return "ssh"
}

func (h *Handler) Definition() (plugin.Definition, error) {
	return h.provider.Definition()
}

func (h *Handler) RegisterPrivateRoutes(router *gin.RouterGroup) {
	terminal := router.Group("/terminal")
	terminal.POST("/connect", h.Capability("终端连接验证", "connect").
		Handle(ginx.WrapBody(h.Connect)),
	)
	terminal.GET("/ws", h.Capability("终端会话", "ssh_session").
		Handle(ginx.Ws(h.SshSessionTunnel)),
	)

	sftpGroup := router.Group("/sftp")
	registerSFTPRoutes(sftpGroup, h)
}

func (h *Handler) Connect(ctx *gin.Context, req ConnectReq) (ginx.Result, error) {
	spec, err := req.Type.spec()
	if err != nil {
		return ginx.Result{Msg: err.Error()}, err
	}

	if err := h.openAndStoreSession(ctx, req.ResourceId, spec.action); err != nil {
		return ginx.Result{}, err
	}

	return ginx.Result{Msg: spec.successMsg}, nil
}

func (h *Handler) openAndStoreSession(ctx context.Context, resourceID int64, action string) error {
	sess, err := h.openSSHSession(ctx, resourceID, action)
	if err != nil {
		return err
	}

	h.replaceSession(resourceID, sess)
	return nil
}

func (h *Handler) replaceSession(resourceID int64, next term.Session) {
	if current, err := h.session.GetSession(resourceID); err == nil && current != nil {
		_ = current.Close()
	}

	h.finder.clear(resourceID)
	h.session.SetSession(resourceID, next)
}

func (h *Handler) openSSHSession(ctx context.Context, resourceID int64, action string) (term.Session, error) {
	actionCtx, err := h.resolver.ResolveActionContext(ctx, define.ResolveRequest(action, resourceID))
	if err != nil {
		return nil, fmt.Errorf("获取 SSH 插件输入失败: %w", err)
	}

	chain, err := define.ResolveGatewayChain(actionCtx)
	if err != nil {
		return nil, fmt.Errorf("解析 SSH 插件输入失败: %w", err)
	}

	connector, ok := term.GetConnector("ssh")
	if !ok {
		return nil, fmt.Errorf("ssh connector not registered")
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	sess, err := connector.Connect(ctxWithTimeout, chain, nil)
	if err != nil {
		return nil, fmt.Errorf("ssh connector fail: %w", err)
	}

	return sess, nil
}

func (h *Handler) SshSessionTunnel(ctx *gin.Context) error {
	resourceIDInt, err := parseRequiredInt64Query(ctx, "resource_id")
	if err != nil {
		return err
	}

	colsInt, err := parseRequiredIntQuery(ctx, "cols")
	if err != nil {
		return err
	}

	rowsInt, err := parseRequiredIntQuery(ctx, "rows")
	if err != nil {
		return err
	}

	return h.wsSSHSession(ctx, resourceIDInt, colsInt, rowsInt)
}

func (h *Handler) wsSSHSession(ctx *gin.Context, resourceID int64, cols, rows int) error {
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	sess, err := h.session.GetSession(resourceID)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
		return err
	}

	shellCapable, ok := sess.(term.ShellCapable)
	if !ok {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("session not support shell"))
		return fmt.Errorf("session does not implement ShellCapable")
	}

	terminalSession, err := shellCapable.NewTerminal(conn, rows, cols)
	if err != nil {
		return err
	}

	terminalSession.Start()
	defer terminalSession.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			_, message, err1 := conn.ReadMessage()
			if err1 == io.EOF {
				return nil
			}
			if err1 != nil {
				return err1
			}

			msg, err2 := sshx.ParseTerminalMessage(message)
			if err2 != nil {
				continue
			}

			switch msg.Operation {
			case "resize":
				if err = terminalSession.Resize(msg.Rows, msg.Cols); err != nil {
					return err
				}
			case "stdin":
				if err = terminalSession.Write([]byte(msg.Data)); err != nil {
					return err
				}
			case "ping":
				if err = terminalSession.Ping(); err != nil {
					return err
				}
			}
		}
	}
}

func parseRequiredInt64Query(ctx *gin.Context, key string) (int64, error) {
	value := ctx.Query(key)
	if value == "" {
		return 0, fmt.Errorf("%s is required", key)
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func parseRequiredIntQuery(ctx *gin.Context, key string) (int, error) {
	value := ctx.Query(key)
	if value == "" {
		return 0, fmt.Errorf("%s is required", key)
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}
