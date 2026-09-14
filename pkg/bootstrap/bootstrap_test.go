package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSkipWebSocket(t *testing.T) {
	gin.SetMode(gin.TestMode)

	calledHandler := false
	dummyHandler := func(ctx *gin.Context) {
		calledHandler = true
		ctx.String(http.StatusOK, "auth_passed")
	}

	tests := []struct {
		name          string
		allowedPaths  []string
		reqPath       string
		upgradeHeader string
		expectBypass  bool // true 表示跳过鉴权直接执行下游，false 表示被拦截调用了 dummyHandler 鉴权
	}{
		{
			name:          "未声明任何白名单时，即使带 Upgrade 也必须走鉴权",
			allowedPaths:  nil,
			reqPath:       "/api/otp",
			upgradeHeader: "websocket",
			expectBypass:  false,
		},
		{
			name:          "请求路径在白名单内且带 Upgrade，成功放行跳过鉴权",
			allowedPaths:  []string{"/terminal/ws"},
			reqPath:       "/terminal/ws",
			upgradeHeader: "websocket",
			expectBypass:  true,
		},
		{
			name:          "请求路径在白名单内但前缀带网关路径且带 Upgrade，成功放行",
			allowedPaths:  []string{"/terminal/ws"},
			reqPath:       "/api/plugin-runtime/builtin.ssh/terminal/ws",
			upgradeHeader: "websocket",
			expectBypass:  true,
		},
		{
			name:          "普通 API 恶意伪造 Upgrade 请求头，严厉拦截走鉴权",
			allowedPaths:  []string{"/terminal/ws"},
			reqPath:       "/api/otp",
			upgradeHeader: "websocket",
			expectBypass:  false,
		},
		{
			name:          "白名单路径普通 HTTP 请求（无 Upgrade 头），正常走鉴权",
			allowedPaths:  []string{"/terminal/ws"},
			reqPath:       "/terminal/ws",
			upgradeHeader: "",
			expectBypass:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calledHandler = false
			router := gin.New()

			// 挂载被测试的中间件
			router.Use(skipWebSocket(dummyHandler, tt.allowedPaths))
			router.Any("/*path", func(ctx *gin.Context) {
				ctx.String(http.StatusOK, "downstream")
			})

			req := httptest.NewRequest(http.MethodGet, tt.reqPath, nil)
			if tt.upgradeHeader != "" {
				req.Header.Set("Upgrade", tt.upgradeHeader)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			if tt.expectBypass {
				assert.False(t, calledHandler, "期望直接放行不触发鉴权中间件")
			} else {
				assert.True(t, calledHandler, "期望触发鉴权中间件执行检查")
			}
		})
	}
}
