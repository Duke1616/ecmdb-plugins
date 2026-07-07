# SSH 插件 (`plugins/ssh`)

SSH 插件是基于主站 CMDB 资产拓扑模型构建的内置微服务插件，提供在线终端（Web Shell）与文件管理（SFTP）的业务处理能力。

---

## 业务能力与资源动作

插件在主站中注册两个关联的资源动作（Action）：
* `terminal` (SSH 终端)：基于 Web Socket 传输的在线终端。
* `sftp` (文件管理)：提供文件预览、上传、下载、重命名、归档等文件管理功能。

---

## CMDB 模型与拓扑绑定

插件的元数据声明（[definition.go](file:///Users/luankz/go-code/ecmdb-plugins/plugins/ssh/internal/define/definition.go)）向主站同步注册了以下拓扑模型与关联关系：
1. **模型定义**：
   - `host`（主机）：包含 IP、端口、登录账号及加密存储的密码、私钥等基础信息。
   - `AuthGateway`（登录网关/跳板机）：包含网关地址、网关登录账号、加密密码和私钥等。
2. **拓扑关系**：
   - `AuthGateway` 和 `host` 之间建立多对多（`ManyToMany`）的拓扑连接关系（一个主机可通过多个跳板机网关代理访问，一个跳板机网关可以服务多台主机）。

---

## 凭证解密与连接链路组装

在执行 SSH/SFTP 连接前，插件后端本身不存储连接密码与私钥，也不执行复杂的拓扑解析。
具体流程如下：
1. 插件后端接收到前端请求时，调用主站 gRPC 的 `ResolveActionContext` 接口。
2. 主站控制面在内存中对 `host` 及关联的 `AuthGateway` 密码和私钥进行安全解密。
3. 按照 `Bind` 声明将解密后的字段拼装为 `ConnectionTarget`（内含网关排序链与最终主机信息）以 JSON 格式投递回插件服务。
4. 插件服务通过 `define.ResolveGatewayChain` 解析数据，构建 `term.GatewayChain` 链路，调用 `ssh` connector 建立真正的物理连接。

---

## 前后端协作与集成

### 1. 前端 UMD 静态托管
* 插件的前端组件（`plugins/ssh/frontend`）被独立构建为 UMD 格式包（将基座中的 `vue` 等声明为 externals）。
* 前端打包的全局挂载名为 `window.EcmdbPluginBuiltinSsh`，入口组件为 `Index`。
* 静态托管在 IoC 装配时由统一脚手架 `bootstrap.NewPluginApp` 直接挂载：
  ```go
  StaticDist: "./plugins/ssh/frontend/dist"
  ```
  主站网关会自动将 `/api/cmdb/plugin-runtime/builtin.ssh/static/index.umd.js` 的请求透明代理转发至本服务的 `/static/index.umd.js` 路径，从而完成前端动态渲染。

### 2. 接口反向代理
前端调用的业务 API 统一通过主站网关代理，去掉网关前缀并转发给插件服务：
- 外部请求地址：`/api/plugin-runtime/builtin.ssh/terminal/connect`
- 网关转发至插件服务路径：`/terminal/connect`

---

## 本地启动与开发

### 1. 运行服务
在根目录下：
```bash
task run:ssh
```
或者在 `plugins/ssh/` 目录下直接运行：
```bash
go run ./cmd/main.go server --addr :18080 --upstream http://127.0.0.1:18080 --ecmdb-grpc-addr 127.0.0.1:9000
```

### 2. 环境变量配置
支持以下环境变量，优先级高于配置文件：
```bash
SSH_PLUGIN_ADDR=:18080                     # 插件监听的 HTTP/WS 端口
SSH_PLUGIN_UPSTREAM=http://127.0.0.1:18080 # 注册给主站网关的反代物理地址
SSH_PLUGIN_TIMEOUT_SECONDS=5               # SSH 拨号超时时间
ECMDB_GRPC_ADDR=127.0.0.1:9000             # 主站的 gRPC 注册中心端口
```

### 3. 配置文件启动
```bash
go run ./cmd/main.go server --config ./config/config.yaml
```

---

## 后端接口列表

插件后端服务内部实现的核心路由包括：

### Terminal (SSH 在线终端)
* `POST /terminal/connect`：创建连接会话。通过主站 gRPC 解析资产凭证，发起 SSH 物理连接并注入会话池中。
* `GET /terminal/ws`：WebSocket 会话隧道，与前端基于 Web Shell 协议（基于 Guacamole 原理）传输终端字符流。

### SFTP (文件管理)
* `GET /sftp/files`：读取文件列表。
* `GET /sftp/download`：下载文件。
* `GET /sftp/preview`：预览文件。
* `GET /sftp/search`：搜索文件。
* `GET /sftp/upload/ws`：基于 Web Socket 的文件上传。
* `POST /sftp/new_folder`：新建目录。
* `POST /sftp/new_file`：新建文件。
* `POST /sftp/rename`：文件/目录重命名。
* `POST /sftp/move`：文件/目录移动。
* `POST /sftp/archive`：文件压缩归档。
* `POST /sftp/unarchive`：解压文件。
* `POST /sftp/save`：文件编辑保存。
* `POST /sftp/delete`：删除文件。
