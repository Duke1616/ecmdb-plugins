# SSH Frontend

SSH 插件前端以 UMD 方式构建，由 ECMDB 主站运行时视图动态加载 `Index` 入口组件。

前端调用插件接口时只使用 ECMDB 下发的 `api_base`：

```ts
await request.post(`${apiBase}/terminal/connect`, { resource_id: resourceId })
```

插件服务真实地址由 ECMDB 插件网关处理，前端不需要知道。
