# ECMDB Plugins (`ecmdb-plugins`)

`ecmdb-plugins` 是 ECMDB 的微服务插件仓库。项目采用 **Mono-repo** 架构，将各业务插件以独立微服务形式进行开发与解耦。

系统基于 **控制面与数据面分离** 的架构进行设计：
* **主站控制面 (ECMDB Core)**：统一托管资产主数据拓扑与关系、敏感凭证存储、并在内存中提供凭证解密服务；提供反向代理网关。
* **插件服务数据面 (Plugin Service)**：承载实际的物理连接和具体业务逻辑（如在线终端会话、SFTP 读写、容器日志拉取等），并托管配套的前端 UMD 微组件。

---

## 核心交互机制

### 1. 微前端热插拔渲染 (UMD)
主站前端提供通用的插件加载容器（基座），通过运行时视图接口（`GET /api/plugin/runtime/view`）获取插件对应的 `index.umd.js` 与 `index.css` 加载路径。基座动态向 Document 插入标签拉取 UMD 资源，并注入 Vue、Pinia、ElementPlus 等全局共享依赖，最终动态渲染挂载插件的前端组件。

### 2. 网关代理与路由重写
前端微组件发送的接口请求均以 `apiBase`（即 `/api/cmdb/plugin-runtime/:plugin_id`）为前缀。主站插件网关拦截此类请求，自动剥离前缀，根据注册的 `upstream` 代理转发。
* 例如：向主站发起的静态资源请求 `/api/cmdb/plugin-runtime/builtin.ssh/static/index.umd.js` 将被代理转发至 SSH 插件后端的 `/static/index.umd.js`。

### 3. 安全凭证隔离与解密投递 (gRPC)
主机密码、私钥等敏感凭据绝不流向前端浏览器。当插件后端收到连接请求时，通过内部 gRPC 接口向主站控制面发起 `ResolveActionContext` 请求。主站控制面在内存中解密凭据，拼装成完整的连接上下文（如 `ConnectionTarget`，含主机和跳板机网关链）后返回给插件服务。插件服务据此发起真正的 SSH/SFTP 物理连接。

---

## 插件开发步骤

开发并运行一个全新的插件，通常包含以下五个核心步骤：

### 1. 定义自描述元数据与拓扑模型
在插件的 `internal/define/definition.go` 中实现 `plugin.Definition` 的元数据与拓扑定义：
- **Action 关联动作**：定义操作菜单（如 `terminal`、`sftp`）及其在主站前端的呈现参数（如 workspace 布局）。
- **Setup 关联模型**：定义插件运行需要的 CMDB 资源模型（如主机模型 `host`，网关模型 `AuthGateway`）和它们的关系拓扑（如多对多关系）。
- **Bind 数据绑定**：定义一个 Go 结构体（如 `ConnectionTarget`），通过结构体 Tag 声明当 Action 触发时，主站应该将 CMDB 实例中的哪些属性字段（包含密码、私钥等）映射拼装好投递给插件后端。

### 2. 实现业务逻辑并实现插件契约
在插件中编写具体的业务处理器（如 `Handler`），并使其实现通用的 `bootstrap.IPlugin` 契约接口：
- 实现 `ID()` 返回插件唯一标识，实现 `Name()` 返回名称。
- 实现 `Definition()` 直接导出第一步中定义的元数据。
- 实现 `RegisterPrivateRoutes(router *gin.RouterGroup)`，只将插件特有的私有核心 API 挂载至传入的路由组。

### 3. 使用 `bootstrap` 一键引导服务
在插件的依赖注入（IoC）层，通过调用通用脚手架一键拉起与装配整个微服务容器。例如在 `ioc/web.go` 中：
```go
func InitWebServer(
	cfg bootstrap.Config,
	sshHdl *sshweb.Handler,
	listener net.Listener,
	resolver *common_grpc.Resolver,
) *bootstrap.PluginApp {
	return bootstrap.NewPluginApp(bootstrap.BootstrapOptions{
		Plugin:              sshHdl,
		Resolver:            resolver,
		Upstream:            cfg.Upstream,
		StaticDist:          "./plugins/<plugin_name>/frontend/dist",
		Listener:            listener,
		PermissionProviders: nil, // 若无特殊权限控制点可留空
	})
}
```
框架内部会自动托管自描述 Well-known 路由、健康检查、日志与 CORS 中间件、前端 UMD 静态托管、EIAM 权限路由同步及主站 gRPC 自动注册（含 `ekit/retry` 优雅指数重避重试机制）。

### 4. 编译打包微组件前端 (UMD)
在 `frontend/` 下开发前端组件。为避免体积冗余，需在前端打包工具（如 `vite.config.ts`）中进行如下配置：
- 将 Vue 基座依赖外部化（`external`），运行时直接读取基座全局变量（如 `window.Vue`）。
- 打包输出格式配置为 `umd`，设置与主站推导一致的全局变量名称挂载在 window 上（例如：`name: 'EcmdbPluginBuiltinSsh'`）。
- 确保 JS 与 CSS 合并打包输出为 `index.umd.js` 和 `index.css` 并输出至 `dist/` 目录。

### 5. 消费上下文并建立物理连接
在业务逻辑处理中，利用主站提供的 gRPC 客户端：
- 向主站发送 `ResolveActionContext` 请求，传入 Resource ID。
- 将主站返回的解密后的上下文数据反序列化为第一步中声明的绑定结构体，直接提取最终的明文密码、私钥以及跳板机网关拓扑，利用通用配置 `bootstrap.PluginConfig` 中的各项指标建立底层的物理连接。

---

## 仓库目录结构

本项目是一个 Mono-repo 结构，所有插件共享同一个 Go Module (`github.com/Duke1616/ecmdb-plugins`)。

```text
.
├── api/                        # gRPC Protobuf 接口定义（包含主站与插件的通信协议）
├── buf.gen.yaml                # Buf 代码生成配置
├── docs/                       # 设计文档与运行时设计规范
├── ioc/                        # 依赖注入组件（Etcd、Registry 等）
├── pkg/                        # 共享的基础工具包（如共享的 gRPC resolver 等）
├── plugins/                    # 插件目录
│   └── ssh/                    # SSH 终端与文件管理插件
│       ├── cmd/                # 插件启动入口
│       ├── config/             # 本地配置文件模板
│       ├── frontend/           # 插件前端组件目录 (Vue3 / TS)
│       └── internal/           # 插件后端领域逻辑（Web接口、配置、定义等）
└── Taskfile.yaml               # 本地开发任务管理
```

---

## 本地开发指南

### 1. 工具链常用命令
通过根目录的 [Taskfile.yaml](file:///Users/luankz/go-code/ecmdb-plugins/Taskfile.yaml) 自动化管理流程：
* 编译更新 gRPC proto 定义：
  ```bash
  task gen
  ```
* 生成 Mock 代码：
  ```bash
  task mock
  ```
* 本地启动默认的 SSH 插件服务：
  ```bash
  task run
  ```

### 2. 运行与配置（以 SSH 插件为例）
可以直接通过命令行参数启动：
```bash
go run ./plugins/ssh/cmd/main.go server --addr :18080 --upstream http://127.0.0.1:18080 --ecmdb-grpc-addr 127.0.0.1:9000
```
或通过环境变量进行配置（优先级高于配置文件）：
```bash
SSH_PLUGIN_ADDR=:18080                     # 插件服务监听端口
SSH_PLUGIN_UPSTREAM=http://127.0.0.1:18080 # 主站网关代理转发的 upstream 物理地址
SSH_PLUGIN_TIMEOUT_SECONDS=5               # SSH 连接超时时间
ECMDB_GRPC_ADDR=127.0.0.1:9000             # ECMDB Core 主站的 gRPC 注册服务地址
```

---

## 插件矩阵列表

| 插件名称 | 插件唯一标识 (`UID`) | 业务描述 | 当前状态 | 对应目录 |
| :--- | :--- | :--- | :--- | :--- |
| **SSH 插件** | `builtin.ssh` | 基于 CMDB 主机和跳板机网关链路，提供在线终端 (Web Shell) 与 SFTP 文件管理。 | 已完成 | [plugins/ssh](file:///Users/luankz/go-code/ecmdb-plugins/plugins/ssh) |
| **K8s 插件** | `builtin.k8s` | 提供 Kubernetes 容器终端登录、日志查看与文件双向拷贝。 | 规划中 | - |
| **RDP 插件** | `builtin.rdp` | 提供 Windows 远程桌面连接管理。 | 规划中 | - |
