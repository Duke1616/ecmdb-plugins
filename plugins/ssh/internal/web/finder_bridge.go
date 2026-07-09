package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Duke1616/ecmdb/pkg/ginx"
	vuefinderginx "github.com/Duke1616/vuefinder-go/pkg/ginx"
	vuefinderweb "github.com/Duke1616/vuefinder-go/pkg/web"
	"github.com/gin-gonic/gin"
)

func registerSFTPRoutes(group *gin.RouterGroup, h *Handler) {
	wrap := h.withFinder

	group.GET("/files", h.Capability("查看文件", "sftp_files").
		NoSync().
		Handle(wrap(vuefinderginx.Wrap(h.finder.Index))),
	)
	group.GET("/download", h.Capability("下载文件", "sftp_download").
		Handle(wrap(h.finder.DownloadStream)),
	)
	group.GET("/search", h.Capability("搜索文件", "sftp_search").
		Handle(wrap(vuefinderginx.Wrap(h.finder.Search))),
	)
	group.GET("/preview", h.Capability("预览文件", "sftp_preview").
		Handle(wrap(vuefinderginx.WrapBuff(h.finder.Preview))),
	)
	group.POST("/new_folder", h.Capability("创建目录", "sftp_new_folder").Handle(wrap(vuefinderginx.WrapBody(h.finder.NewFolder))))
	group.POST("/new_file", h.Capability("创建文件", "sftp_new_file").Handle(wrap(vuefinderginx.WrapBody(h.finder.NewFile))))
	group.POST("/rename", h.Capability("重命名文件", "sftp_rename").Handle(wrap(vuefinderginx.WrapBody(h.finder.Rename))))
	group.POST("/move", h.Capability("移动文件", "sftp_move").Handle(wrap(vuefinderginx.WrapBody(h.finder.Move))))
	group.POST("/archive", h.Capability("压缩文件", "sftp_archive").Handle(wrap(vuefinderginx.WrapBody(h.finder.Archive))))
	group.POST("/unarchive", h.Capability("解压文件", "sftp_unarchive").Handle(wrap(vuefinderginx.WrapBody(h.finder.Unarchive))))
	group.POST("/save", h.Capability("保存文件内容", "sftp_save").Handle(wrap(vuefinderginx.WrapBuffBody(h.finder.Save))))
	group.POST("/delete", h.Capability("删除文件", "sftp_delete").Handle(wrap(vuefinderginx.WrapBody(h.finder.Delete))))
	group.POST("/upload", h.Capability("上传文件", "sftp_upload").
		Handle(wrap(vuefinderginx.WrapUpload(h.finder.Upload))),
	)
	group.GET("/upload/ws", h.Capability("上传文件", "sftp_upload_ws").
		Handle(wrap(func(ctx *gin.Context) {
			vuefinderweb.UploadHandler(h.finder.Handler)(ctx.Writer, ctx.Request)
		})),
	)
}

func (h *Handler) withFinder(next gin.HandlerFunc) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token, err := parseFinderSessionToken(ctx)
		if err != nil {
			ctx.PureJSON(http.StatusInternalServerError, ginx.Result{
				Code: 0,
				Msg:  err.Error(),
			})
			return
		}

		runtimeSess, err := h.sessions.Get(token)
		if err != nil {
			ctx.PureJSON(http.StatusInternalServerError, ginx.Result{
				Code: 0,
				Msg:  err.Error(),
			})
			return
		}

		if !h.finder.isReady(runtimeSess.finderID) {
			if err = h.finder.attach(runtimeSess.finderID, runtimeSess.session); err != nil {
				ctx.PureJSON(http.StatusInternalServerError, ginx.Result{
					Code: 0,
					Msg:  err.Error(),
				})
				return
			}
		}

		// NOTE: 核心桥接适配设计。
		// 对外，我们在 WebSocket 与 HTTP 会话中使用全局唯一的随机 string 类型的 SessionToken，以支持多终端并发。
		// 对内，由于依赖的 vuefinder-go 第三方库底层深度依赖整型 (int64) 的存储插槽分配，
		// 我们在此处将请求的 x-finder-id 头部与 id 查询参数透明重写为内部映射的 int64 finderID。
		// 这既根治了连接覆盖问题，又以零侵入方式完美兼顾了下游库的类型约束。
		finderIDStr := fmt.Sprintf("%d", runtimeSess.finderID)
		ctx.Request.Header.Set("x-finder-id", finderIDStr)
		if ctx.Query("id") != "" {
			q := ctx.Request.URL.Query()
			q.Set("id", finderIDStr)
			ctx.Request.URL.RawQuery = q.Encode()
		}

		next(ctx)
	}
}

func parseFinderSessionToken(ctx *gin.Context) (string, error) {
	token := strings.TrimSpace(ctx.GetHeader("x-finder-id"))
	if token == "" {
		token = strings.TrimSpace(ctx.Query("id"))
	}
	if token == "" {
		return "", fmt.Errorf("session token is required")
	}
	return token, nil
}
