# ECMDB Plugins

ECMDB 插件仓库用于沉淀独立插件实现。插件后端可以单独部署，ECMDB Core 只负责插件目录、模型绑定、资源上下文解析和统一代理入口。

当前仓库先提供 SSH 插件骨架，用来验证最小插件边界：

- 插件实现 `plugin.Provider`，通过 `/.well-known/ecmdb-plugin` 暴露 `plugin.Definition`。
- ECMDB 同步插件定义后，按 `plugin_id -> upstream` 做无脑反向代理。
- 插件自己的 HTTP 接口、WebSocket、SFTP 文件接口和 EIAM 权限控制都留在插件后端。
- 浏览器不直接访问插件服务真实地址，只访问 ECMDB 插件网关。

## 目录结构

```text
.
├── cmd/
│   └── ssh-plugin/
├── docs/
│   └── runtime-design.md
└── plugins/
    └── ssh/
        ├── ioc/
        ├── internal/config/
        ├── internal/define/
        ├── internal/ssh/
        ├── internal/web/
        ├── README.md
        ├── backend/README.md
        └── frontend/README.md
```

## 运行 SSH 插件

```bash
go run ./cmd/ssh-plugin
```

可选参数：

```bash
go run ./cmd/ssh-plugin --addr :18080 --upstream http://127.0.0.1:18080
go run ./cmd/ssh-plugin --ecmdb-grpc-addr 127.0.0.1:9000
```

可选环境变量，优先级高于配置文件：

```text
SSH_PLUGIN_ADDR=:18080
SSH_PLUGIN_UPSTREAM=http://127.0.0.1:18080
SSH_PLUGIN_TIMEOUT_SECONDS=5
ECMDB_GRPC_ADDR=127.0.0.1:9000
```

配置文件示例：

```yaml
ssh:
  addr: ":18080"
  upstream: "http://127.0.0.1:18080"
  timeout_seconds: 5
ecmdb:
  grpc_addr: "127.0.0.1:9000"
```

通过 `--config` 指定配置文件：

```bash
go run ./cmd/ssh-plugin --config ./config.yaml
```

插件自描述接口：

```bash
curl http://127.0.0.1:18080/.well-known/ecmdb-plugin
```

健康检查：

```bash
curl http://127.0.0.1:18080/healthz
```

## 插件接入思路

推荐流程：

1. 插件服务启动并暴露 `/.well-known/ecmdb-plugin`。
2. ECMDB 拉取该接口，导入 `plugin.Definition`。
3. ECMDB 保存插件 runtime，例如 `upstream` 和 `health_path`。
4. 前端在资源详情页读取 ECMDB 返回的可用 action。
5. 前端调用 ECMDB 插件网关，由 ECMDB 转发到插件后端。

## 当前插件

| 插件 | 说明 | 状态 |
| --- | --- | --- |
| SSH | 基于 CMDB 主机和网关关系提供在线终端、SFTP 文件管理能力 | 后端自描述骨架 |

## 设计文档

- [插件运行时设计](docs/runtime-design.md)
- [SSH 插件说明](plugins/ssh/README.md)
