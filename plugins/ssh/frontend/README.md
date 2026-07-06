# SSH Frontend

这里预留 SSH 插件前端。第一阶段可以继续使用 ECMDB 主前端内置的 `builtin:terminal` 和 `builtin:sftp` UI。

前端调用插件接口时只使用 ECMDB 下发的 `api_base`：

```ts
await request.post(`${apiBase}/terminal/connect`, { resource_id: resourceId })
```

插件服务真实地址由 ECMDB 插件网关处理，前端不需要知道。
