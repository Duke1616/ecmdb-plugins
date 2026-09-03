package web

import (
	"fmt"
	"net/http"
	"strings"

	vuefinderginx "github.com/Duke1616/vuefinder-go/pkg/ginx"
	"github.com/gin-gonic/gin"
)

func (h *Handler) withFinder(ctx *gin.Context) {
	token, err := parseFinderSessionToken(ctx)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, vuefinderginx.Result{
			Code:    0,
			Message: err.Error(),
		})
		return
	}

	runtimeSess, err := h.sessions.Get(token)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, vuefinderginx.Result{
			Code:    0,
			Message: err.Error(),
		})
		return
	}

	if !h.finder.isReady(runtimeSess.finderID) {
		if err = h.finder.attach(runtimeSess.finderID, runtimeSess.session); err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, vuefinderginx.Result{
				Code:    0,
				Message: err.Error(),
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

	ctx.Next()
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
