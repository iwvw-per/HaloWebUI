# HaloWebUI 安装指南

## Docker（推荐）

```bash
docker volume create halowebui-data
docker run -d --name halowebui \
  --restart unless-stopped \
  -p 3000:8080 \
  -v halowebui-data:/app/data \
  -e WEBUI_SECRET_KEY="replace-with-a-long-random-secret" \
  iwvw/halowebui:latest
```

访问 <http://localhost:3000>。首次注册用户成为管理员。升级前备份 `/app/data`，然后拉取新镜像并用相同 volume 重建容器。

## Docker Compose

```bash
docker compose up -d
docker compose logs -f open-webui
```

可选覆盖：

- `docker-compose.ollama.yaml`：增加 Ollama 服务。
- `docker-compose.api.yaml`：把 Ollama API 暴露到宿主机。
- `docker-compose.data.yaml`：把 Ollama 模型映射到指定本地目录。
- `docker-compose.gpu.yaml`：为 Ollama 请求 NVIDIA GPU。
- `docker-compose.amdgpu.yaml`：为 Ollama 使用 ROCm 设备。

示例：

```bash
docker compose \
  -f docker-compose.yaml \
  -f docker-compose.ollama.yaml \
  -f docker-compose.gpu.yaml \
  up -d
```

## Fly.io 小主机

适用目标是 256 MB 内存、1 GB 持久盘。应用只应运行一个 Go 进程，并使用远程模型服务。

1. 创建应用和至少 1 GB volume，并挂载到 `/app/data`。
2. 设置 `WEBUI_SECRET_KEY`，不要依赖每次启动自动生成的密钥。
3. 设置 `HALO_GO_MEMORY_LIMIT_MIB=48`。
4. 将内部端口设为 `8080`，健康检查路径设为 `/health`。
5. 首次上线后检查进程 RSS、volume 剩余空间和 SQLite WAL 增长。

不要在这类主机内运行 Ollama、本地 embedding、OCR、Whisper、浏览器自动化或大型 MCP 子进程。它们应部署到独立机器并通过网络连接。

## Kubernetes

仓库只维护 Kustomize manifest，不提供 Helm Chart：

```bash
kubectl apply -k kubernetes/manifest/base
```

NVIDIA GPU 只应用于同组部署的 Ollama：

```bash
kubectl apply -k kubernetes/manifest/gpu
```

部署前应修改 Ingress host、存储类、资源限制和 `WEBUI_SECRET_KEY`。默认 WebUI PVC 是 1 GiB。

## 从源码运行

要求 Go 1.25 和 Node.js 22：

```bash
npm ci
npm run build
cd backend
go run ./cmd/halowebui
```

本地运行时设置 `FRONTEND_DIR=./build` 和 `DATA_DIR=./.data`。Windows PowerShell 示例：

```powershell
$env:FRONTEND_DIR = (Resolve-Path ./build)
$env:DATA_DIR = "$PWD/.data"
Set-Location backend
go run ./cmd/halowebui
```

全部环境变量见 [`docs/backend.md`](docs/backend.md)。
