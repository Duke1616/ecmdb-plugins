package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Duke1616/ecmdb-plugins/plugins/ssh/internal/config"
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
	capability.IRegistry
}

func NewHandler(cfg config.Config) *Handler {
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
		IRegistry: capability.NewRegistry("cmdb", "ssh", "资产仓库/SSH 插件"),
	}
}

var upGrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	Subprotocols: []string{"guacamole"},
}

func (h *Handler) PublicRoutes(server *gin.Engine) {
	server.GET(plugin.WellKnownPath, gin.WrapH(plugin.DefinitionHandler(h.provider)))
	server.GET("/healthz", h.healthz)

	// 托管独立编译打包出的前端静态文件目录
	server.Static("/static", "./plugins/ssh/frontend/dist")
}

func (h *Handler) PrivateRoutes(server *gin.Engine) {
	terminal := server.Group("/terminal")
	terminal.POST("/connect", h.Capability("终端连接验证", "connect").
		Handle(ginx.WrapBody(h.Connect)),
	)
	terminal.GET("/ws", h.Capability("终端会话", "ssh_session").
		Handle(ginx.Ws(h.SshSessionTunnel)),
	)

	sftpGroup := server.Group("/sftp")
	sftpGroup.GET("/files", h.Capability("查看文件", "sftp_files").
		Handle(h.sftpNotImplemented("files")),
	)
	sftpGroup.GET("/download", h.Capability("下载文件", "sftp_download").
		Handle(h.sftpNotImplemented("download")),
	)
	sftpGroup.GET("/search", h.Capability("搜索文件", "sftp_search").
		Handle(h.sftpNotImplemented("search")),
	)
	sftpGroup.GET("/preview", h.Capability("预览文件", "sftp_preview").
		Handle(h.sftpNotImplemented("preview")),
	)
	sftpGroup.POST("/new_folder", h.Capability("创建目录", "sftp_new_folder").
		Handle(h.sftpNotImplemented("new_folder")),
	)
	sftpGroup.POST("/new_file", h.Capability("创建文件", "sftp_new_file").
		Handle(h.sftpNotImplemented("new_file")),
	)
	sftpGroup.POST("/rename", h.Capability("重命名文件", "sftp_rename").
		Handle(h.sftpNotImplemented("rename")),
	)
	sftpGroup.POST("/move", h.Capability("移动文件", "sftp_move").
		Handle(h.sftpNotImplemented("move")),
	)
	sftpGroup.POST("/archive", h.Capability("压缩文件", "sftp_archive").
		Handle(h.sftpNotImplemented("archive")),
	)
	sftpGroup.POST("/unarchive", h.Capability("解压文件", "sftp_unarchive").
		Handle(h.sftpNotImplemented("unarchive")),
	)
	sftpGroup.POST("/save", h.Capability("保存文件内容", "sftp_save").
		Handle(h.sftpNotImplemented("save")),
	)
	sftpGroup.POST("/delete", h.Capability("删除文件", "sftp_delete").
		Handle(h.sftpNotImplemented("delete")),
	)
	sftpGroup.GET("/upload/ws", h.Capability("上传文件", "sftp_upload_ws").
		Handle(h.sftpNotImplemented("upload_ws")),
	)
}

func (h *Handler) healthz(ctx *gin.Context) {
	ctx.PureJSON(http.StatusOK, gin.H{
		"status":    "ok",
		"plugin_id": define.PluginUID,
	})
}

func (h *Handler) sftpNotImplemented(endpoint string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.PureJSON(http.StatusNotImplemented, ginx.Result{
			Code: http.StatusNotImplemented,
			Msg:  "SFTP endpoint is not implemented yet",
			Data: gin.H{"endpoint": endpoint},
		})
	}
}

func (h *Handler) Connect(ctx *gin.Context, req ConnectReq) (ginx.Result, error) {
	switch req.Type {
	case ConnectTypeRDP:
		return ginx.Result{Msg: "不支持RDP协议"}, fmt.Errorf("暂不支持 RDP 协议")
	case ConnectTypeVNC:
		return ginx.Result{Msg: "不支持VNC协议"}, fmt.Errorf("暂不支持 VNC 协议")
	case ConnectTypeSSH:
		_, err := h.connectSSH(ctx, req.ResourceId, define.ActionTerminal)
		if err != nil {
			return ginx.Result{}, err
		}
	case ConnectTypeWebSftp:
		_, err := h.connectSSH(ctx, req.ResourceId, define.ActionSFTP)
		if err != nil {
			return ginx.Result{}, err
		}
	default:
		return ginx.Result{Msg: fmt.Sprintf("不支持的连接类型: %s", req.Type)}, fmt.Errorf("unsupported connect type: %s", req.Type)
	}

	return ginx.Result{Msg: "SSH 连接成功"}, nil
}

func (h *Handler) connectSSH(ctx context.Context, resourceID int64, action string) (term.Session, error) {
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

	h.session.SetSession(resourceID, sess)
	return sess, nil
}

func (h *Handler) SshSessionTunnel(ctx *gin.Context) error {
	resourceID := ctx.Query("resource_id")
	resourceIDInt, err := strconv.ParseInt(resourceID, 10, 64)
	if err != nil {
		return err
	}

	cols := ctx.Query("cols")
	colsInt, err := strconv.Atoi(cols)
	if err != nil {
		return err
	}

	rows := ctx.Query("rows")
	rowsInt, err := strconv.Atoi(rows)
	if err != nil {
		return err
	}

	return h.wsSSHSession(ctx, resourceIDInt, colsInt, rowsInt)
}

func (h *Handler) wsSSHSession(ctx *gin.Context, resourceID int64, cols, rows int) error {
	conn, err := upGrader.Upgrade(ctx.Writer, ctx.Request, nil)
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
			_, message, err := conn.ReadMessage()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}

			msg, er := sshx.ParseTerminalMessage(message)
			if er != nil {
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
