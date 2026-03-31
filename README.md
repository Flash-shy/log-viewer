# Log Viewer

本地日志目录的 Web 浏览与 HTTP API。

## 产品功能

- **数据来源**：从本机配置的目录读取日志文件（默认仓库内 `logs/`，可通过环境变量 `LOG_VIEWER_LOG_DIR` 指定）。当前示例包含模拟的 Nginx 访问/错误、MySQL、Docker 容器等文本日志。
- **Web 界面**：列出目录内日志文件；选择文件后按「末尾行数」拉取并展示（等宽字体、行号）；支持刷新。
- **HTTP API**：健康检查、文件列表、按行读取内容（支持 `tail` 或 `offset`/`limit`）；单文件大小与单次返回行数有上限，避免误读超大文件。
- **OpenAPI**：`/api/docs` 提供 Swagger UI，`/openapi.yaml` 为 OpenAPI 3 规范，便于对接与调试。

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
