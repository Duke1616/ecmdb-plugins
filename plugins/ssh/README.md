# SSH 插件

SSH 插件用于把 CMDB 中的主机、账号和登录网关关系转换为可执行的远程访问能力。当前已实现插件自描述后端骨架，终端和 SFTP 具体运行时接口仍待从 ECMDB 主仓库迁移。

## 资源动作

- `terminal`：SSH 在线终端。
- `sftp`：文件管理。

## 资源模型

| 模型 | 用途 |
| --- | --- |
| `host` | 被连接的目标主机 |
| `AuthGateway` | 登录网关或跳板机 |

`AuthGateway` 和 `host` 是 many-to-many 关系。一个主机可以通过多个网关访问，一个网关也可以服务多个主机。

## 输入解析

插件执行前，ECMDB 需要根据资源 ID 解析出连接链路：

```text
AuthGateway[] -> host
```

连接字段包括：

- `host` / `ip`
- `port`
- `username`
- `password`
- `private_key`
- `auth_type`
- `sort`

密码和私钥属于敏感字段，不能下发到浏览器。

## 本地启动

```bash
go run ./cmd/ssh-plugin
```

配置来源支持 cobra flag、viper 配置文件和环境变量：

```bash
go run ./cmd/ssh-plugin --addr :18080 --upstream http://127.0.0.1:18080
go run ./cmd/ssh-plugin --ecmdb-grpc-addr 127.0.0.1:9000
go run ./cmd/ssh-plugin --config ./plugins/ssh/config/config.yaml
```

```text
SSH_PLUGIN_ADDR=:18080
SSH_PLUGIN_UPSTREAM=http://127.0.0.1:18080
SSH_PLUGIN_TIMEOUT_SECONDS=5
ECMDB_GRPC_ADDR=127.0.0.1:9000
```

访问：

```bash
curl http://127.0.0.1:18080/.well-known/ecmdb-plugin
curl http://127.0.0.1:18080/healthz
```

## 后端接口预留

```text
POST /terminal/connect
GET  /terminal/ws
GET  /sftp/files
GET  /sftp/download
GET  /sftp/preview
GET  /sftp/search
GET  /sftp/upload/ws
POST /sftp/new_folder
POST /sftp/new_file
POST /sftp/rename
POST /sftp/move
POST /sftp/archive
POST /sftp/unarchive
POST /sftp/save
POST /sftp/delete
```

外部访问时由 ECMDB 插件网关代理，例如：

```text
/api/plugin-runtime/builtin.ssh/terminal/connect
/api/plugin-runtime/builtin.ssh/sftp/files
```

## 迁移来源

后续可以从 ECMDB 主仓库中的这些位置逐步抽离：

- `internal/plugin/ssh`
- `internal/web/terminal`
- `pkg/term`
