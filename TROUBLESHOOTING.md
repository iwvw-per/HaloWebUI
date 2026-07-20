# HaloWebUI 排障指南

## 服务无法启动

先检查容器日志和健康检查：

```bash
docker logs halowebui
curl -fsS http://127.0.0.1:3000/health
```

常见原因：`/app/data` 不可写、端口被占用、`HALO_GO_MEMORY_LIMIT_MIB` 不在 16-160 范围、`FILE_MAX_SIZE_MB` 不在 1-250 范围。distroless 镜像没有 shell，诊断应通过日志、健康接口或挂载 volume 完成。

## 登录后失效或重启后全部退出

生产环境必须设置稳定的 `WEBUI_SECRET_KEY`，并复用同一个 `/app/data`。如果密钥和自动生成的 secret 文件同时丢失，旧 JWT 无法继续验证。

HTTPS 反向代理后应设置：

```text
WEBUI_AUTH_COOKIE_SECURE=true
WEBUI_AUTH_COOKIE_SAME_SITE=lax
```

并正确转发 `Host`、`X-Forwarded-For` 和 `X-Forwarded-Proto`。

## Provider 模型发现失败

在“设置 > 连接”中填写的是模型 Provider 的完整基础地址，不是 HaloWebUI 自己的地址。OpenAI 兼容端点通常以 `/v1` 结尾，例如 `https://provider.example/v1`。检查：

- URL 是否包含正确协议和版本路径。
- API Key 是否属于该 Provider。
- 容器是否能访问公网或目标内网。
- Provider 的 `/models` 是否兼容 OpenAI 格式。
- 密钥是否只保存在管理界面或 secret 中，而不是日志和仓库。

## Ollama 连接失败

容器中的 `127.0.0.1` 指向容器本身。宿主机 Ollama 通常应使用：

```text
OLLAMA_BASE_URL=http://host.docker.internal:11434
```

Linux 需要在运行参数中加入 `--add-host=host.docker.internal:host-gateway`。使用仓库 Compose 的 Ollama 覆盖时地址会自动设为 `http://ollama:11434`。

## 聊天卡住或没有流式输出

检查浏览器网络面板中聊天请求是否持续接收 SSE 数据，再检查反向代理是否缓冲响应。Nginx 对流式路径应关闭缓冲：

```nginx
proxy_buffering off;
proxy_read_timeout 600s;
```

如果启用了 WebSocket，还必须转发 `Upgrade` 和 `Connection` 头。默认 Go 部署使用 HTTP/SSE，不要求 WebSocket。

## 内存或磁盘不足

- 保持 `HALO_GO_MEMORY_LIMIT_MIB=48`，不要把它设置为主机全部可用内存。
- 降低 `FILE_MAX_SIZE_MB`，限制并发大文件和长上下文请求。
- 清理 `/app/data` 中不再需要的上传和旧备份，但不要直接删除正在使用的 SQLite 文件或 WAL。
- 将 Ollama、OCR、语音识别、embedding 和浏览器自动化移到其他主机。
- 用容器 RSS 或 cgroup 指标判断内存，不要用虚拟内存值。

## 数据恢复

优先通过管理界面的备份/恢复功能处理 `.hwbk`。恢复前另存当前 `/app/data`，并确保磁盘有足够空间容纳数据库、上传文件和临时解压内容。服务会拒绝路径穿越、symlink 和超过 512 MiB 的备份内容。

## 服务器端 Python

当前服务端完全由 Go 实现，镜像中没有 Python、pip、FastAPI 或 uvicorn。聊天代码块中的 Python 执行是浏览器按需下载的 Pyodide；它不会修复或影响后端问题，也不应被当作服务端扩展机制。
