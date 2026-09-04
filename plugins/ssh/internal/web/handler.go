package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	vuefinderginx "github.com/Duke1616/vuefinder-go/pkg/ginx"
	vuefinderweb "github.com/Duke1616/vuefinder-go/pkg/web"

	"github.com/Duke1616/ecmdb-plugins/pkg/bootstrap"
	"github.com/Duke1616/ecmdb-plugins/pkg/contract/permission"
	"github.com/Duke1616/ecmdb-plugins/plugins/ssh/internal/define"
	_ "github.com/Duke1616/ecmdb-plugins/plugins/ssh/internal/ssh"
	"github.com/Duke1616/ecmdb/pkg/plugin"
	"github.com/Duke1616/ecmdb/pkg/plugin/types"
	"github.com/Duke1616/ecmdb/pkg/term"
	"github.com/Duke1616/ecmdb/pkg/term/sshx"
	"github.com/Duke1616/eiam/pkg/web/capability"
	"github.com/ecodeclub/ginx"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type missingResolver struct{}

func (missingResolver) ResolveActionContext(context.Context, types.ResolveRequest) (types.ActionContext, error) {
	return types.ActionContext{}, fmt.Errorf("ecmdb context resolver is not configured")
}

type Handler struct {
	provider define.Provider
	resolver plugin.ContextResolver
	sessions *runtimeSessionStore
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
		sessions:  newRuntimeSessionStore(),
		timeout:   timeout,
		finder:    newFinderRuntime(),
		IRegistry: capability.NewRegistry("cmdb", "ssh", "插件中心/SSH 插件"),
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
	// ==========================================
	// 1. Web 终端会话路由
	// ==========================================
	terminal := router.Group("/terminal")

	// 终端连接
	terminal.POST("/connect", h.Define("终端连接", "connect").
		Needs(permission.Ssh.SshSession, permission.Ssh.SftpFiles).
		Bind(ginx.B[ConnectReq](h.Connect)),
	)

	// 终端长连接会话通道
	terminal.GET("/ws", h.Define("终端会话", "ssh_session").
		NoSync().
		Bind(h.SshSessionTunnel),
	)

	// ==========================================
	// 2. SFTP 文件管理路由 (全局由 withFinder 守卫)
	// ==========================================
	sftp := router.Group("/sftp")
	sftp.Use(h.withFinder)

	// 查看文件列表
	sftp.GET("/files", h.Define("查看文件", "sftp_files").
		NoSync().
		Bind(vuefinderginx.Wrap(h.finder.Index)),
	)

	// 下载文件
	sftp.GET("/download", h.Define("下载文件", "sftp_download").
		Bind(h.finder.DownloadStream),
	)

	// 搜索文件
	sftp.GET("/search", h.Define("搜索文件", "sftp_search").
		Bind(vuefinderginx.Wrap(h.finder.Search)),
	)

	// 预览文件
	sftp.GET("/preview", h.Define("预览文件", "sftp_preview").
		Bind(vuefinderginx.WrapBuff(h.finder.Preview)),
	)

	// 创建目录
	sftp.POST("/new_folder", h.Define("创建目录", "sftp_new_folder").
		Bind(vuefinderginx.WrapBody(h.finder.NewFolder)),
	)

	// 创建文件
	sftp.POST("/new_file", h.Define("创建文件", "sftp_new_file").
		Bind(vuefinderginx.WrapBody(h.finder.NewFile)),
	)

	// 重命名文件
	sftp.POST("/rename", h.Define("重命名文件", "sftp_rename").
		Bind(vuefinderginx.WrapBody(h.finder.Rename)),
	)

	// 移动文件
	sftp.POST("/move", h.Define("移动文件", "sftp_move").
		Bind(vuefinderginx.WrapBody(h.finder.Move)),
	)

	// 压缩文件
	sftp.POST("/archive", h.Define("压缩文件", "sftp_archive").
		Bind(vuefinderginx.WrapBody(h.finder.Archive)),
	)

	// 解压文件
	sftp.POST("/unarchive", h.Define("解压文件", "sftp_unarchive").
		Bind(vuefinderginx.WrapBody(h.finder.Unarchive)),
	)

	// 保存文件内容
	sftp.POST("/save", h.Define("保存文件内容", "sftp_save").
		Bind(vuefinderginx.WrapBuffBody(h.finder.Save)),
	)

	// 删除文件
	sftp.POST("/delete", h.Define("删除文件", "sftp_delete").
		Bind(vuefinderginx.WrapBody(h.finder.Delete)),
	)

	// 上传文件
	sftp.POST("/upload", h.Define("上传文件", "sftp_upload").
		Bind(vuefinderginx.WrapUpload(h.finder.Upload)),
	)

	// WebSocket 上传通道
	sftp.GET("/upload/ws", h.Define("上传文件", "sftp_upload_ws").
		Bind(func(ctx *gin.Context) {
			vuefinderweb.UploadHandler(h.finder.Handler)(ctx.Writer, ctx.Request)
		}),
	)
}

func (h *Handler) Connect(ctx *ginx.Context, req ConnectReq) (ginx.Result, error) {
	spec, err := req.Type.spec()
	if err != nil {
		return ginx.Result{Msg: err.Error()}, err
	}

	sessionToken, err := h.openAndStoreSession(ctx, req.ResourceId, spec.action)
	if err != nil {
		return ginx.Result{}, err
	}

	return ginx.Result{Msg: spec.successMsg, Data: ConnectResp{SessionID: sessionToken}}, nil
}

func (h *Handler) openAndStoreSession(ctx context.Context, resourceID int64, action string) (string, error) {
	sess, err := h.openSSHSession(ctx, resourceID, action)
	if err != nil {
		return "", err
	}

	runtimeSess, err := h.sessions.Put(sess)
	if err != nil {
		return "", err
	}
	return runtimeSess.token, nil
}

func (h *Handler) closeSession(token string) {
	if runtimeSess, err := h.sessions.Get(token); err == nil {
		h.finder.clear(runtimeSess.finderID)
	}
	h.sessions.Close(token)
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

func (h *Handler) SshSessionTunnel(ctx *gin.Context) {
	token, err := parseSessionTokenQuery(ctx)
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	colsInt, err := parseRequiredIntQuery(ctx, "cols")
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	rowsInt, err := parseRequiredIntQuery(ctx, "rows")
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	if err = h.wsSSHSession(ctx, token, colsInt, rowsInt); err != nil {
		_ = ctx.Error(err)
	}
}

const (
	wsPongWait = 90 * time.Second
)

func (h *Handler) wsSSHSession(ctx *gin.Context, token string, cols, rows int) error {
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer h.closeSession(token)

	runtimeSess, err := h.sessions.Get(token)
	if err != nil {
		_ = writeTerminalError(conn, err.Error())
		return err
	}

	sess := runtimeSess.session
	shellCapable, ok := sess.(term.ShellCapable)
	if !ok {
		_ = writeTerminalError(conn, "session not support shell")
		return fmt.Errorf("session does not implement ShellCapable")
	}

	terminalSession, err := shellCapable.NewTerminal(conn, rows, cols)
	if err != nil {
		return err
	}

	terminalSession.Start()
	defer terminalSession.Stop()

	_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

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

			// 关键保活加固。无论客户端发送的是业务心跳还是按键输入，只要有数据到达，
			// 即证明链路存活，立即顺延读超时，防止因云网关/中间代理丢弃 Pong 帧而误判超时断开。
			_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))

			msg, err2 := sshx.ParseTerminalMessage(message)
			if err2 != nil {
				continue
			}

			if strategy, ok := terminalOpStrategies[msg.Operation]; ok {
				if err = strategy(terminalSession, msg); err != nil {
					return err
				}
			}
		}
	}
}

// terminalOpStrategy 终端消息操作策略接口
type terminalOpStrategy func(session term.TerminalSession, msg sshx.TerminalMessage) error

// NOTE: 采用策略模式解耦各终端操作指令的具体执行逻辑，遵循开闭原则
var terminalOpStrategies = map[string]terminalOpStrategy{
	"resize": func(s term.TerminalSession, m sshx.TerminalMessage) error {
		return s.Resize(m.Rows, m.Cols)
	},
	"stdin": func(s term.TerminalSession, m sshx.TerminalMessage) error {
		return s.Write([]byte(m.Data))
	},
	"ping": func(s term.TerminalSession, _ sshx.TerminalMessage) error {
		return s.Ping()
	},
}

func writeTerminalError(conn *websocket.Conn, message string) error {
	return conn.WriteJSON(sshx.NewMessage("stderr", message, 0, 0))
}

func parseSessionTokenQuery(ctx *gin.Context) (string, error) {
	token := ctx.Query("session_id")
	if token == "" {
		return "", fmt.Errorf("session_id is required")
	}
	return token, nil
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
