# Go 后端运行手册

## 目录布局

```text
backend/
  cmd/halowebui/       进程入口和健康检查命令
  internal/auth/       密码、JWT、API Key 和会话
  internal/server/     HTTP 路由、Provider 适配和 SSE
  internal/store/      SQLite schema、迁移和持久化
src/                   Svelte 前端源码
static/                前端静态资源
```

Go 服务同时提供 `/api`、Provider 代理、SSE、文件、知识库、HaloClaw、Terminal 和前端静态文件。单实例 SQLite 是默认部署模式；扩展到多实例前必须提供共享数据库和共享文件存储，并验证锁与会话语义。

## 配置参考

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `HOST` / `PORT` | `0.0.0.0:8080` | 监听地址 |
| `DATA_DIR` | `/app/data` | SQLite、上传、备份和运行时文件 |
| `FRONTEND_DIR` | `/app/build` | 前端构建目录 |
| `HALO_GO_MEMORY_LIMIT_MIB` | `48` | Go 软内存上限，允许范围 16-160 MiB |
| `FILE_MAX_SIZE_MB` | `25` | 单文件上传上限，允许范围 1-250 MiB |
| `WEBUI_SECRET_KEY` | 自动生成 | 生产环境必须显式设置并持久化 |
| `JWT_EXPIRES_IN` | `-1` | JWT 有效期，可用 Go duration、`d` 或 `w` |
| `OPENAI_API_BASE_URL` / `OPENAI_API_KEY` | OpenAI 默认地址 | 默认 OpenAI 兼容连接 |
| `OLLAMA_BASE_URL` / `OLLAMA_API_KEY` | `http://127.0.0.1:11434` | Ollama 地址 |
| `ENABLE_SIGNUP` | `true` | 是否允许注册 |
| `ENABLE_LOGIN_FORM` | `true` | 是否显示登录表单 |
| `ENABLE_API_KEY` | `true` | 是否允许用户 API Key |
| `ENABLE_TERMINAL` | `true` | 是否启用受限文件 Terminal API |
| `ENABLE_WEBSOCKET_SUPPORT` | `false` | 可选 WebSocket 兼容层 |
| `WEBUI_AUTH_COOKIE_SECURE` | `false` | HTTPS 部署时设为 `true` |
| `WEBUI_AUTH_COOKIE_SAME_SITE` | `lax` | Cookie SameSite 策略 |
| `ENABLE_SCIM` / `SCIM_AUTH_BEARER_TOKEN` | `false` / 空 | SCIM 管理接口 |
| `ENABLE_ADMIN_EXPORT` | `false` | 管理员导出接口 |

管理界面中保存的 Provider、用户连接和模型覆盖写入 SQLite，不应把密钥提交到 `.env` 或仓库。

## 资源预算

Go 进程默认使用 48 MiB 软内存上限，CI 对最终镜像执行以下门禁：镜像小于 100 MiB、镜像文件树不含 Python 运行时路径、启动后 RSS 小于 100 MiB。Fly.io 256 MB 主机还需要为内核、文件缓存、网络缓冲和 SQLite 写入预留空间。

2026-07-20 本地发行构建证据：Linux amd64 stripped 二进制 11.79 MiB，前端构建树 75.54 MiB，二者合计 87.32 MiB；最新 Go 进程空闲 RSS 11.94 MiB。最终容器大小和 RSS 仍以 GitHub Actions 的 Docker 门禁为准。

默认镜像不包含本地 ML 推理、Python、Node、Git、uv 或 MCP stdio 运行时。浏览器 Pyodide 只在前端按需下载，开发时可用 `npm run dev:full`，生产 Docker 构建明确设置 `ENABLE_PYODIDE=false`。

## 开发与验证

```bash
cd backend
go test ./...
go vet ./...
cd ..
npm run test:frontend -- --run
npm run build
```

交叉编译发行二进制：

```bash
cd backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags='-s -w' -o ../.tmp/halowebui-linux-amd64 ./cmd/halowebui
```

## 发布与回滚

`.github/workflows/go-docker-publish.yml` 在 `codex/backend-refactor` 和 `main` 上运行 Go 测试、Docker 构建、镜像内容检查、健康检查和内存门禁，并推送到 `iwvw/halowebui`。重构分支产生 `go-edge` 与 `git-<sha>` 标签，`main` 产生 `latest`、`slim` 与 `git-<sha>` 标签。

回滚只切换到上一份 Go 镜像并保留 `/app/data`；不要恢复已删除的 Python 服务，也不要在回滚时删除 SQLite、上传或备份文件。
