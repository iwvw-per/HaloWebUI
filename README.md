<div align="center">
  <img src="./static/favicon.png" alt="HaloWebUI" width="96" height="96" />
  <h1>HaloWebUI</h1>
  <p><strong>轻量、自托管的多模型 AI 工作台</strong></p>
  <p>Svelte 4 + TypeScript 前端，Go + SQLite 后端，面向 256 MB 级别的小型主机优化。</p>
</div>

## 当前架构

- **前端**：SvelteKit、Svelte 4、TypeScript、Tailwind CSS，构建产物由 Go 服务提供。
- **后端**：Go 1.25，标准 `net/http`，SQLite（`modernc.org/sqlite`），单进程运行。
- **模型连接**：OpenAI 兼容接口、Gemini、Anthropic、Grok、Ollama，以及管理界面中配置的自定义连接。
- **数据目录**：默认 `/app/data`，包含 SQLite 数据库、上传文件、备份和运行时配置。
- **实时能力**：HTTP 流式响应和可选 WebSocket；默认关闭 WebSocket 以减少空闲资源。
- **部署镜像**：`iwvw/halowebui`，最终镜像只包含 Go 二进制、前端静态文件和 distroless 运行时，不包含 Python、pip、uvicorn 或 FastAPI。

后端目录是 [`backend/`](backend/)，旧 Python 服务已删除。浏览器端的 Pyodide 是用户主动执行代码时下载的可选运行时，不参与服务端启动，也不会进入生产镜像。

更完整的接口、配置、资源预算和验证证据见 [`docs/backend.md`](docs/backend.md)。

## 快速开始

### Docker

```bash
docker run -d --name halowebui \
  --restart unless-stopped \
  -p 3000:8080 \
  -v halowebui-data:/app/data \
  -e WEBUI_SECRET_KEY="replace-with-a-stable-secret" \
  iwvw/halowebui:latest
```

打开 <http://localhost:3000>。首次注册的用户自动成为管理员。生产部署必须设置稳定的 `WEBUI_SECRET_KEY`，并持久化 `/app/data`。

镜像标签与分支一一对应：`main` 分支发布 `latest`，`dev` 分支发布 `dev`。生产环境使用 `latest`，测试环境可使用 `iwvw/halowebui:dev`。

### Docker Compose

```bash
docker compose up -d
```

需要同时运行 Ollama 时：

```bash
docker compose -f docker-compose.yaml -f docker-compose.ollama.yaml up -d
```

`docker-compose.api.yaml` 用于暴露 Ollama 端口，`docker-compose.data.yaml` 用于把 Ollama 模型保存到本地目录；GPU 覆盖文件只作用于 Ollama，不会改变 HaloWebUI 的 Go 服务。

### 本地开发

要求：Go 1.25、Node.js 22、npm。分别启动后端和前端：

在一个终端启动后端：

```bash
cd backend
go run ./cmd/halowebui
```

在仓库根目录的另一个终端启动前端：

```bash
npm install
npm run dev
```

前端开发服务器会把 API 请求转发到 `http://127.0.0.1:8080`。生产构建使用 `npm run build`，然后设置 `FRONTEND_DIR=./build` 启动 Go 服务。

## 小主机配置

针对 Fly.io 256 MB（约 200 MB 可用内存、1 GB 磁盘）的建议：

- 只运行一个实例，`HALO_GO_MEMORY_LIMIT_MIB=48` 保留系统和请求峰值空间。
- 使用远程模型和远程检索服务；不要在该实例加载本地 embedding、OCR、Whisper 或浏览器自动化运行时。
- 给 `/app/data` 挂载持久盘，保留至少 300 MB 可用空间用于升级和备份。
- 限制上传文件大小（默认 25 MB），定期清理旧备份和临时文件。
- 通过 `/health` 做健康检查，不要用首页响应代替健康检查。

## 文档

- [Go 后端架构、配置与验收证据](docs/backend.md)
- [安装与部署](INSTALLATION.md)
- [排障](TROUBLESHOOTING.md)
- [贡献指南](docs/CONTRIBUTING.md)
- [安全策略](docs/SECURITY.md)
- [移动端与 PWA](docs/mobile-access.md)
- [后端重构 PRD 与实施记录](docs/prd/backend-refactor.md)

## 许可证

本项目遵循 [BSD-3-Clause](LICENSE) 许可证。项目基于 [Open WebUI](https://github.com/open-webui/open-webui) 的前端和交互设计演进，相关版权和许可说明见 [`docs/apache.md`](docs/apache.md)。
