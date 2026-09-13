# ECMDB 插件微服务开发与代码实战指南 (Plugin Guide)

本文档提供完整的代码模版与开发实现细节。开发者可直接复制以下代码骨架，根据业务需求进行修改。

---

## 目录
- [一、单资产插件实战模板（以 Redis 为例）](#一单资产插件实战模板以-redis-为例)
  - [1. 领域与元数据定义 (definition.go)](#1-领域与元数据定义-definitiongo)
  - [2. 业务处理与路由挂载 (handler.go)](#2-业务处理与路由挂载-handlergo)
  - [3. 容器装配与服务启动 (ioc/web.go)](#3-容器装配与服务启动-iocwebgo)
- [二、复合拓扑插件实战模板（以 SSH 为例）](#二复合拓扑插件实战模板以-ssh-为例)
- [三、纯动作插件模板（以网络工具为例）](#三纯动作插件模板以网络工具为例)
- [四、微前端微组件模板 (Vue3 + Vite UMD)](#四微前端微组件模板-vue3--vite-umd)
  - [1. 页面组件 (Index.vue)](#1-页面组件-indexvue)
  - [2. 打包配置 (vite.config.ts)](#2-打包配置-viteconfigts)
- [五、插件 Tag 语法速查字典](#五插件-tag-语法速查字典)

---

## 一、单资产插件实战模板（以 Redis 为例）

适用场景：管理某类独立资产模型（如 Redis、MySQL、Nginx 实例、域名等），并在 CMDB 资产详情页挂载运维工作台。

### 1. 领域与元数据定义 (`internal/define/definition.go`)
```go
package define

import (
	"context"
	"fmt"

	"github.com/Duke1616/ecmdb-plugins/pkg/model"
	"github.com/Duke1616/ecmdb/pkg/plugin"
)

const (
	PluginUID     = "builtin.redis"
	ActionConsole = "console"
	ModelRedis    = "redis_instance"
)

// 1. 声明数据资产模型：字段即 CMDB 映射，包含 password/secret/token 等关键字的字段由主站后台异步自动加密
type RedisTarget struct {
	model.BaseResource // 内嵌基础字段 (Name/ID)
	Host     string `plugin:"host,label=主机地址,field=ip,required"`
	Port     int    `plugin:"port,label=连接端口,default=6379"`
	Password string `plugin:"password,label=连接密码"` // 敏感加密字段
	DB       int    `plugin:"db,label=默认库号,default=0"`
}

type Provider struct {
	upstream string
}

func NewProvider(upstream string) Provider {
	return Provider{upstream: upstream}
}

// 2. 导出自描述元数据：Target 绑定资产 -> Model 声明名称分组 -> Workspace 挂载工作台
func (p Provider) Definition() (plugin.Definition, error) {
	reg := plugin.NewRegistry(
		PluginUID,
		"Redis 管理器",
		plugin.Type("builtin"),
		plugin.Version("1.0.0"),
		plugin.Description("提供 Redis 在线命令行交互与实时监控能力"),
		plugin.ExternalServiceRuntime(p.upstream, plugin.RuntimeHealthPath("/healthz")),
	)

	return plugin.Target[RedisTarget](reg, ModelRedis).
		Model("Redis实例", "缓存服务"). // 👈 一行声明模型展示名与所属分组
		Workspace(
			ActionConsole,
			"Redis 控制台",
			plugin.Icon("Terminal"),
			plugin.Permission("cmdb:redis:console"),
			plugin.CardFields("name", "ip", "port"), // 侧边栏展示的关键字段
			plugin.Prop("connectionType", "redis-cli"),
		).
		Definition()
}

// 3. 消费动作上下文：一键向主站拉取内存态已解密的完整凭据
func ResolveRedisTarget(ctx context.Context, resolver plugin.ContextResolver, resourceID int64) (RedisTarget, error) {
	// 一行代码完成：gRPC 通信 + 内存解密 + 强类型映射赋值
	return plugin.ResolveActionRoot[RedisTarget](ctx, resolver, PluginUID, ActionConsole, resourceID)
}
```

### 2. 业务处理与路由挂载 (`internal/web/handler.go`)
```go
package web

import (
	"fmt"
	"net/http"

	"github.com/Duke1616/ecmdb-plugins/plugins/redis/internal/define"
	"github.com/Duke1616/ecmdb/pkg/plugin"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	resolver plugin.ContextResolver
	provider define.Provider
}

func NewHandler(resolver plugin.ContextResolver, provider define.Provider) *Handler {
	return &Handler{resolver: resolver, provider: provider}
}

// 实现 IPlugin 通用契约
func (h *Handler) ID() string                        { return define.PluginUID }
func (h *Handler) Name() string                      { return "Redis 管理器" }
func (h *Handler) Definition() (plugin.Definition, error) { return h.provider.Definition() }

// 挂载插件私有 API (主站反代前缀 /api/cmdb/plugin-runtime/:plugin_id 会被网关自动剥离)
func (h *Handler) RegisterPrivateRoutes(server *gin.Engine) {
	g := server.Group("/api/redis")
	{
		g.POST("/ping", h.Ping)
	}
}

type PingReq struct {
	ResourceID int64 `json:"resource_id" binding:"required"`
}

func (h *Handler) Ping(c *gin.Context) {
	var req PingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 消费上下文：此时 target.Password 已经是解密后的明文密码
	target, err := define.ResolveRedisTarget(c.Request.Context(), h.resolver, req.ResourceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 利用解密密码与物理基础设施握手
	c.JSON(http.StatusOK, gin.H{
		"msg":     "连接成功",
		"address": fmt.Sprintf("%s:%d", target.Host, target.Port),
		"db":      target.DB,
	})
}
```

### 3. 容器装配与服务启动 (`ioc/web.go`)
```go
package ioc

import (
	"net"

	"github.com/Duke1616/ecmdb-plugins/pkg/bootstrap"
	common_grpc "github.com/Duke1616/ecmdb-plugins/pkg/grpc"
	redisweb "github.com/Duke1616/ecmdb-plugins/plugins/redis/internal/web"
)

func InitWebServer(
	cfg bootstrap.Config,
	hdl *redisweb.Handler,
	listener net.Listener,
	resolver *common_grpc.Resolver,
) *bootstrap.PluginApp {
	// 一键拉起微服务容器：自动托管自描述端点、静态 UMD 资源托管、健康检查与 gRPC 自动服务注册
	return bootstrap.NewPluginApp(bootstrap.BootstrapOptions{
		Plugin:     hdl,
		Resolver:   resolver,
		Upstream:   cfg.Upstream,
		StaticDist: "./plugins/redis/frontend/dist",
		Listener:   listener,
	})
}
```

---

## 二、复合拓扑插件实战模板（以 SSH 为例）

适用场景：资产间存在依赖、多跳网关等复杂关系。通过结构体组合与切片，**自动推导资产模型与拓扑关系图谱**。

```go
package define

import (
	"github.com/Duke1616/ecmdb-plugins/pkg/model"
	"github.com/Duke1616/ecmdb/pkg/plugin"
)

// 网关资产结构
type Gateway struct {
	model.BaseResource
	Host       string `plugin:"host,label=网关地址,required"`
	Port       int    `plugin:"port,label=端口,default=22"`
	Username   string `plugin:"username,label=账号,required"`
	Password   string `plugin:"password,label=密码"`
	PrivateKey string `plugin:"private_key,label=私钥"`
	Sort       int    `plugin:"sort,label=排序权重"`
}

// 主机基础资产
type Endpoint struct {
	model.BaseResource
	Host       string `plugin:"host,label=主机地址,field=ip,required"`
	Port       int    `plugin:"port,label=SSH端口,default=22"`
	Username   string `plugin:"username,label=登录账号,required"`
	Password   string `plugin:"password,label=登录密码"`
	PrivateKey string `plugin:"private_key,label=私钥凭证"`
}

// 目标复合资产：主机 + 跳板机网关链路
type ConnectionTarget struct {
	// 1. 匿名内嵌作为根资产
	Endpoint `plugin:",label=主机资产,group=计算资源"`

	// 2. 切片关联：自动推导与 AuthGateway 模型的出向关联关系 (out=default)
	Gateways []Gateway `plugin:"gateways,model=AuthGateway,name=跳板机网关,group=安全凭据,out=default"`
}

func (p Provider) Definition() (plugin.Definition, error) {
	reg := plugin.NewRegistry("builtin.ssh", "SSH 插件")

	// 自动生成 host 与 AuthGateway 的模型与拓扑图谱，无需手动编写建表或映射
	return plugin.Target[ConnectionTarget](reg, "host").
		Workspace("terminal", "Web Shell", plugin.Icon("terminal")).
		Workspace("sftp", "Web Sftp", plugin.Icon("folder")).
		Definition()
}
```

---

## 三、纯动作插件模板（以网络工具为例）

适用场景：不绑定任何 CMDB 资产模型，作为系统级通用工具或全局批量扫描能力。

```go
package define

import "github.com/Duke1616/ecmdb/pkg/plugin"

func (p Provider) Definition() (plugin.Definition, error) {
	reg := plugin.NewRegistry(
		"builtin.network_tools",
		"网络诊断工具箱",
		plugin.Type("builtin"),
		plugin.Version("1.0.0"),
		plugin.ExternalServiceRuntime(p.upstream, plugin.RuntimeHealthPath("/healthz")),
	)

	// 直接注册全局动作，无需 Target 资产绑定
	return reg.
		Action("ping", "全局 Ping 检测", plugin.Permission("tools:net:ping")).
		Action("traceroute", "路由跟踪", plugin.Permission("tools:net:trace")).
		Definition()
}
```

---

## 四、微前端微组件模板 (Vue3 + Vite UMD)

### 1. 页面组件 (`frontend/src/Index.vue`)
```vue
<template>
  <div class="plugin-container">
    <h3>{{ title }}</h3>
    <el-button type="primary" :loading="loading" @click="onExecute">
      开始执行
    </el-button>
    <div v-if="result" class="result-box">
      <pre>{{ result }}</pre>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import axios from 'axios'

// 关键契约：主站基座加载微组件时会自动注入当前资产 ID 与代理网关前缀
interface Props {
  resourceId?: number // 当前资产在 CMDB 中的主键 ID (纯动作插件为空)
  apiBase: string     // 主站反代前缀，如 "/api/cmdb/plugin-runtime/builtin.redis"
}

const props = defineProps<Props>()
const title = ref('插件控制面板')
const loading = ref(false)
const result = ref('')

const onExecute = async () => {
  loading.value = true
  try {
    // 必须使用 props.apiBase 拼装请求，主站网关会自动反代至插件后端服务
    const res = await axios.post(`${props.apiBase}/api/redis/ping`, {
      resource_id: props.resourceId,
    })
    result.value = JSON.stringify(res.data, null, 2)
  } catch (err: any) {
    result.value = err.message
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.plugin-container { padding: 16px; height: 100%; }
.result-box { margin-top: 16px; background: #f5f7fa; padding: 12px; border-radius: 4px; }
</style>
```

### 2. 打包配置 (`frontend/vite.config.ts`)
```ts
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

export default defineConfig({
  plugins: [vue()],
  build: {
    lib: {
      entry: path.resolve(__dirname, 'src/index.ts'),
      name: 'EcmdbPluginBuiltinRedis', // 与主站由 plugin_id 推导出的 global_name 严格保持一致
      fileName: () => 'index.umd.js',
      formats: ['umd'],
    },
    rollupOptions: {
      // 外部化公共依赖，由主站基座统一注入，大幅缩减微组件包体积
      external: ['vue', 'element-plus', 'pinia', 'axios'],
      output: {
        assetFileNames: 'index.[ext]', // 确保样式文件输出为 index.css
        globals: {
          vue: 'Vue',
          'element-plus': 'ElementPlus',
          pinia: 'Pinia',
          axios: 'axios',
        },
      },
    },
  },
})
```

---

## 五、插件 Tag 语法速查字典

| 选项语法 | 适用场景 | 说明 | 示例 |
| :--- | :--- | :--- | :--- |
| `plugin:"<key>"` | 基础字段 | 字段在绑定图谱中的别名 key | `plugin:"host"` |
| `label=<text>` 或 `name=<text>` | 所有字段 | 模型中文名或属性展示名称 | `label=连接密码` |
| `group=<text>` | 嵌入或关联字段 | 模型所属的 CMDB 模型分组 | `group=计算资源` |
| `field=<cmdb_field>` | 基础字段 | 映射至 CMDB 资产的底层字段 UID | `field=ip` |
| `default=<val>` | 基础字段 | CMDB 对应值为空时的默认回退值 | `default=22` |
| `required` | 基础字段 | 声明必填字段，缺失时解析直接阻断报错 | `plugin:"host,required"` |
| `model=<model_uid>` | 关联对象/切片 | 显式指定关联的目标 CMDB 模型 UID | `model=AuthGateway` |
| `out=<rel_type>` | 关联对象/切片 | 声明从当前模型出发的关联关系 | `out=default` |
| `in=<rel_type>` | 关联对象/切片 | 声明指向当前模型的入向关联关系 | `in=run` |
| `plugin:"-"` | 所有字段 | 忽略该字段，不参与内省与模型映射 | `plugin:"-"` |

> [!TIP]
> **自动加密规则**：字段名或标签包含 `password`、`private_key`、`secret`、`token` 时，系统会自动标记为加密敏感属性，无需额外手工配置。
