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
		if err := h.ensureSFTPFinder(ctx); err != nil {
			ctx.PureJSON(http.StatusInternalServerError, ginx.Result{
				Code: 0,
				Msg:  err.Error(),
			})
			return
		}
		next(ctx)
	}
}

func (h *Handler) ensureSFTPFinder(ctx *gin.Context) error {
	sessionID, err := parseFinderSessionID(ctx)
	if err != nil {
		return err
	}

	if h.finder.isReady(sessionID) {
		return nil
	}

	sess, err := h.sessions.Get(sessionID)
	if err != nil {
		return err
	}

	return h.finder.attach(sessionID, sess)
}

func parseFinderSessionID(ctx *gin.Context) (int64, error) {
	finderID := strings.TrimSpace(ctx.GetHeader("x-finder-id"))
	if finderID == "" {
		finderID = strings.TrimSpace(ctx.Query("id"))
	}
	if finderID == "" {
		return 0, fmt.Errorf("finder id is required")
	}

	return parsePositiveInt64(finderID, "finder id")
}
