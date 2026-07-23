# Go 后端与前端功能差分

更新时间：2026-07-23

## 结论

当前 Svelte 前端实际使用的具体 API 已有自动契约测试：
`backend/internal/server/server_test.go` 中的
`TestEveryConcreteFrontendAPIEndpointHasGoOwner` 会扫描 `src/lib/apis` 的
`fetch` 调用，并请求 Go `ServeMux`。只要响应带有
`X-Halo-Compatibility-Fallback: true`，测试就失败。当前 `go test ./...` 通过，
因此没有发现“前端调用但落入 404 compatibility fallback”的已确认缺口。

这不等于 Go 与上游 Python 的能力完全相同。下面区分了前端契约、上游扩展契约和
部署 profile 限制，避免把聚合 handler 或动态路由误判为缺失。

## 已对应

| 范围 | 前端入口 | Go 归属 | 状态 |
| --- | --- | --- | --- |
| 认证、用户、API key、SCIM | `src/lib/apis/auths`, `users` | `auth_handlers.go`, `scim_handlers.go` | 已对应并有 Go 测试 |
| 聊天、分享、分支、标签、SSE | `src/lib/apis/chats` | `chat_handlers.go`, `stream_handlers.go` | 已对应 |
| 模型、连接、Provider 配置 | `src/lib/apis/models`, `connections` | `model_handlers.go`, `provider_handlers.go`, `config_handlers.go` | 已对应；Provider 通过聚合 handler 分派 |
| 文件、知识库、检索 | `src/lib/apis/files`, `knowledge`, `retrieval` | `file_handlers.go`, `knowledge_handlers.go`, `retrieval_handlers.go` | 已对应；本地实现是 slim 版 |
| prompts、notes、tools、tasks、utils | 对应 `src/lib/apis/*` | `resource_handlers.go`, `task_handlers.go`, `utils_handlers.go` | 已对应；多种资源共用 typed resource handler |
| 频道和消息 reaction | `src/lib/apis/channels` | `channel_handlers.go`, `store/channels.go` | 已对应；上游 chat reaction 当前也被注释禁用 |
| 音频、HaloClaw、备份恢复、外部 API | 对应设置和 API 模块 | `audio_handlers.go`, `haloclaw_handlers.go`, `utils_handlers.go`, `external_api_handlers.go` | 已对应 |

## 已修复的前端能力开关

`/api/config` 曾把频道、联网搜索、图像生成和直连能力硬编码为 `false`，导致已有 Go
handler 被前端隐藏。现在 `server.go` 从持久化 retrieval、web search、image 和
connections 配置计算这些 feature，测试
`TestConfigAdvertisesPersistedFrontendCapabilities` 覆盖该行为。

## 明确的 profile 限制

| 能力 | 上游参考 | 当前 Go 行为 | 用户影响 | 优先级 |
| --- | --- | --- | --- | --- |
| Python code format | 上游 Python formatter | `POST /api/v1/utils/code/format` 只做确定性的换行规范化，并返回 `formatter_unavailable` | 不等价于 Black/autopep8 | P1 |
| 本地 Python code execute | 上游本地 Python runtime | Go slim 只转发到远程 Jupyter；未配置 URL 返回 503 | 需要远程 Jupyter，不能在控制进程执行任意 Python | P1 |
| stdio MCP | 上游 MCP child process | 明确返回 400；只支持 HTTP MCP | 现有 stdio server 配置必须改为 HTTP | P1 |
| Playwright / Firecrawl loader | 上游可选 loader | retrieval capability 标记不可用并返回说明 | 复杂网页抓取需外部服务或内置 loader 的有限子集 | P1 |
| 本地 embedding / reranking / Whisper / OCR | 上游本地模型栈 | slim 使用 lexical/remote adapter；不加载 Python、Torch、Whisper 或 OCR runtime | 需配置远程 capability worker，模型质量/语言覆盖可能不同 | P1 |
| STT/TTS provider | 上游支持更多本地和云 provider | Go slim 规范化为远程 OpenAI-compatible STT/TTS，TTS 另支持浏览器 TTS | Deepgram、Azure、本地 Whisper 配置不会按原语义运行 | P1 |
| Web search provider | 上游 provider 集合 | 当前实现支持 Tavily 和 SearXNG；不支持的 engine 明确 400 | Brave、其他未适配 provider 不能直接迁移 | P2 |
| Socket.IO | 上游 Socket.IO 事件协议 | 前端默认关闭 WebSocket；Go 提供原生 WS/SSE 所需能力但不是完整 Socket.IO server | 依赖 Socket.IO 事件的外部客户端不能假定兼容 | P2 |
| OAuth token exchange | 上游 `routers/auths.py:/token/exchange` | 当前 Go 未提供该外部 OAuth/JWKS 交换端点；前端没有调用 | 仅影响外部统一登录集成，不影响登录表单 | P2 |

## 上游存在但当前前端未使用

上游 knowledge 的 `/{id}/files/batch/add` 和 memories 的 `/ef` 在当前 `src/lib/apis`
中没有调用；Go 也没有为这两个上游专用端点建立同名兼容路由。它们不属于当前 UI 的
已确认缺口，但如果要保证第三方 API 兼容，应作为独立 contract slice 增加测试和实现。

上游 chat reactions 在参考仓库中被 `[REACTION_FEATURE]` 注释禁用；当前实现的 reaction
是频道消息 reaction，不应把两者混为一谈。

## 继续对齐建议

1. P1：将 formatter、远程 worker、MCP transport 和 loader capability 统一放入 capability registry，返回结构化 `unsupported` 状态而不是让设置页猜测。
2. P1：为 embedding/reranking/audio/document adapter 定义相同的 worker interface，并补充远程服务的超时、取消、重试和幂等测试。
3. P2：如果需要第三方上游兼容，新增 token exchange、knowledge batch add、memory `/ef` 的 contract fixtures；不要仅注册空 fallback。
4. P2：把 `/api/config` 的 feature 计算迁移到 capability registry，避免新 handler 与 feature flag 再次分离。

## 验证命令

```text
cd backend
go test -count=1 ./...
```

前端完整类型检查目前仍被仓库既有的 i18n/隐式 any/模型类型错误阻塞；本次新增的
`LazySettingsPanel` 类型错误已清除。生产构建 `npm run build` 已通过。
