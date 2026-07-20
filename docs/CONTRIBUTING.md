# 贡献指南

HaloWebUI 当前由 Svelte/TypeScript 前端和 Go 后端组成。服务端 Python 已从仓库删除，新的后端功能必须使用 Go 实现。

## 开发环境

- Node.js 22、npm
- Go 1.25
- Docker（需要验证镜像时）

安装依赖并启动：

```bash
npm ci
npm run dev
cd backend
go run ./cmd/halowebui
```

前端开发服务器默认使用 5173，Go API 默认使用 8080。不要把 `.env`、`.data/`、上传文件、Provider 密钥或本地测试数据库提交到 Git。

## 提交前检查

```bash
cd backend
go test ./...
go vet ./...
cd ..
npm run test:frontend -- --run
npm run build
git diff --check
```

涉及 HTTP、SSE、认证、SQLite schema、文件恢复或 Provider 适配时，必须增加对应回归测试，并在 PR 中记录资源和安全影响。保持单实例 SQLite 约束，不要引入隐式全局状态或无界缓冲。

## 目录约定

- `backend/cmd/halowebui`：进程入口。
- `backend/internal/server`：路由和外部协议。
- `backend/internal/store`：持久化和迁移。
- `backend/internal/auth`：认证与凭证。
- `src/`：Svelte 页面、组件和 API 客户端。
- `static/`：不会随请求生成的前端资源。

## Pull Request

PR 描述应包含：变更范围、兼容性、迁移或回滚方法、测试命令、内存/磁盘证据、配置变化和已知限制。不要提交构建产物、依赖缓存或临时截图。涉及用户行为时，更新根目录文档或 `docs/` 中对应章节。

Issue 请附复现步骤、浏览器/容器版本、`/health` 结果和脱敏日志。不要在 Issue、PR 或日志中粘贴 API Key、JWT、Cookie 或完整备份文件。
