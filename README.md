# Log Viewer

本地日志目录的 Web 浏览与 HTTP API。

## 运行

```bash
chmod +x run.sh
./run.sh
```

- 后端：`http://127.0.0.1:8080`（日志目录默认使用仓库根目录下的 `logs/`，可通过环境变量 `LOG_VIEWER_LOG_DIR` 覆盖）
- 前端：`http://127.0.0.1:5173`
- OpenAPI（Swagger UI）：`http://127.0.0.1:8080/api/docs`

手动启动示例：

```bash
cd backend && go run ./cmd/server -addr :8080 -logs /path/to/logs
cd frontend && npm run start
```

## API 摘要

| 路径 | 说明 |
|------|------|
| `GET /api/health` | 健康检查 |
| `GET /api/logs` | 列出日志目录中的文件 |
| `GET /api/logs/content?name=...&tail=400` | 读取文件末尾若干行 |
| `GET /api/docs` | Swagger UI |
| `GET /openapi.yaml` | OpenAPI 规范 YAML |
