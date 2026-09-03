# ECMDB-PLUGINS 全局权限大盘与元数据字典

> 本文档由 `permgen` 基于全仓 AST 静态分析自动生成。请勿手动修改。
>
> 💡 **联动包含机制**：当为角色分配某项操作权限时，系统将**自动附带拥有**其“联动包含”中的权限，无需管理员手动重复勾选（例如：勾选“修改用户”会自动附带拥有“用户详情”权限）。

- **受控业务模块数**: 1
- **受控权限点总数**: 16


## 模块: 插件中心/SSH 插件 (`ssh`)

- **所属服务**: `cmdb`
- **定义源码**: `plugins/ssh/internal/web/handler.go`

| 操作名称 | 完整权限码 | 作用域 | 归属类型 | 暴露状态 | 联动包含权限 | 宿主源码位置 |
|:---|:---|:---|:---|:---|:---|:---|
| 终端连接 | `cmdb:ssh:connect` | 租户级 | 本级 | 正常 | 终端会话 · `cmdb:ssh:ssh_session`<br>查看文件 · `cmdb:ssh:sftp_files` | `plugins/ssh/internal/web/handler.go` 行 91 |
| 压缩文件 | `cmdb:ssh:sftp_archive` | 租户级 | 本级 | 正常 | - | `plugins/ssh/internal/web/handler.go` 行 150 |
| 删除文件 | `cmdb:ssh:sftp_delete` | 租户级 | 本级 | 正常 | - | `plugins/ssh/internal/web/handler.go` 行 165 |
| 下载文件 | `cmdb:ssh:sftp_download` | 租户级 | 本级 | 正常 | - | `plugins/ssh/internal/web/handler.go` 行 115 |
| 查看文件 | `cmdb:ssh:sftp_files` | 租户级 | 本级 | 静默 (不暴露) | - | `plugins/ssh/internal/web/handler.go` 行 109 |
| 移动文件 | `cmdb:ssh:sftp_move` | 租户级 | 本级 | 正常 | - | `plugins/ssh/internal/web/handler.go` 行 145 |
| 创建文件 | `cmdb:ssh:sftp_new_file` | 租户级 | 本级 | 正常 | - | `plugins/ssh/internal/web/handler.go` 行 135 |
| 创建目录 | `cmdb:ssh:sftp_new_folder` | 租户级 | 本级 | 正常 | - | `plugins/ssh/internal/web/handler.go` 行 130 |
| 预览文件 | `cmdb:ssh:sftp_preview` | 租户级 | 本级 | 正常 | - | `plugins/ssh/internal/web/handler.go` 行 125 |
| 重命名文件 | `cmdb:ssh:sftp_rename` | 租户级 | 本级 | 正常 | - | `plugins/ssh/internal/web/handler.go` 行 140 |
| 保存文件内容 | `cmdb:ssh:sftp_save` | 租户级 | 本级 | 正常 | - | `plugins/ssh/internal/web/handler.go` 行 160 |
| 搜索文件 | `cmdb:ssh:sftp_search` | 租户级 | 本级 | 正常 | - | `plugins/ssh/internal/web/handler.go` 行 120 |
| 解压文件 | `cmdb:ssh:sftp_unarchive` | 租户级 | 本级 | 正常 | - | `plugins/ssh/internal/web/handler.go` 行 155 |
| 上传文件 | `cmdb:ssh:sftp_upload` | 租户级 | 本级 | 正常 | - | `plugins/ssh/internal/web/handler.go` 行 170 |
| 上传文件 | `cmdb:ssh:sftp_upload_ws` | 租户级 | 本级 | 正常 | - | `plugins/ssh/internal/web/handler.go` 行 175 |
| 终端会话 | `cmdb:ssh:ssh_session` | 租户级 | 本级 | 静默 (不暴露) | - | `plugins/ssh/internal/web/handler.go` 行 97 |

---


