# 插件运行时设计

这份文档描述 ECMDB 插件拆分后的推荐运行方式。目标不是一开始做复杂插件市场，而是让插件能够独立开发、独立部署，同时复用 ECMDB 的资源上下文和统一入口。

## 核心角色

### ECMDB Core

ECMDB Core 继续负责资产和模型主数据能力：

- 资产模型、属性、关系和资源实例。
- 插件注册表和插件绑定关系。
- 根据资源 ID 解析插件需要的输入上下文。
- 插件网关，例如 `/api/plugin-runtime/{plugin_id}/...`。

ECMDB 网关只做尽量薄的代理：识别 `plugin_id`，查 `upstream`，保留登录态和请求头，然后转发到插件服务。接口级权限由插件自己的 EIAM middleware 控制。

### Plugin Service

Plugin Service 是独立部署的插件后端：

- 暴露 `/.well-known/ecmdb-plugin`，返回 `plugin.Definition`。
- 声明 actions、模型、关系、默认绑定和 runtime。
- 接收 ECMDB 插件网关转发的请求。
- 自己接入 EIAM 权限控制。
- 执行业务逻辑，例如 SSH 连接、SFTP 文件操作。

### Plugin Frontend

Plugin Frontend 可以是独立构建产物，也可以先作为 ECMDB 前端内的内置组件存在。

建议插件前端只依赖 ECMDB 提供的运行时参数：

- `plugin_id`
- `action`
- `resource_id`
- `api_base`
- `tenant_id`

敏感字段不应该下发到浏览器，例如密码、私钥、跳板机凭据等。这些字段由插件后端通过 ECMDB 受控接口获取，或者后续由 ECMDB 生成短生命周期 context token。

## 请求链路

推荐链路：

```text
Browser
  -> ECMDB Web
  -> /api/plugin-runtime/{plugin_id}/{plugin_path}
  -> ECMDB Plugin Gateway
  -> Plugin Service
```

插件真实地址只保存在 ECMDB 插件注册表里，例如：

```json
{
  "plugin_id": "builtin.ssh",
  "runtime": {
    "mode": "external-service",
    "upstream": "http://ecmdb-plugin-ssh:18080",
    "health_path": "/healthz"
  }
}
```

## 自描述接口

插件服务暴露：

```text
GET /.well-known/ecmdb-plugin
```

返回 ECMDB `pkg/plugin.Definition`。Go 插件可以直接使用：

```go
plugin.MountWellKnown(mux, provider)
```

## Nginx 配置方式

Nginx 不建议为每个插件写一段代理配置。推荐只代理 ECMDB：

```nginx
location / {
    proxy_pass http://ecmdb-web;
}

location /api/ {
    proxy_pass http://ecmdb-api;
}
```

插件请求统一落到 ECMDB：

```text
/api/plugin-runtime/builtin.ssh/terminal/connect
/api/plugin-runtime/builtin.ssh/terminal/ws
/api/plugin-runtime/builtin.ssh/sftp/files
```

ECMDB 再根据 `builtin.ssh` 找到对应插件服务并转发。新增插件时只需要更新 ECMDB 插件注册表，不需要改 Nginx。

## 权限设计

当前推荐第一版：

- ECMDB 不理解插件内部接口权限。
- ECMDB 透传 `Authorization`、Cookie 和 EIAM 相关请求头。
- 插件后端继续使用自己的 EIAM middleware 鉴权。
- 插件服务只暴露在内网，避免绕过 ECMDB 统一入口。

后续如果需要资源详情页 action 级权限展示，可以在 `ActionSpec.Meta` 或单独 registration API 中补充权限元数据。

## 分阶段落地

### Phase 1: 自描述插件

- 插件实现 `plugin.Provider`。
- 插件暴露 `/.well-known/ecmdb-plugin`。
- ECMDB 导入 `Definition` 和 runtime。

### Phase 2: 后端外置

- SSH 后端接口从 ECMDB 主服务拆到独立插件服务。
- ECMDB 提供插件网关和资源上下文解析接口。
- 原 `/api/term`、`/api/finder` 迁移到插件服务。

### Phase 3: 前端外置

- SSH 前端页面或组件独立构建。
- ECMDB 前端根据 action 加载插件前端入口。
- ECMDB 主前端只保留通用插件运行容器。
