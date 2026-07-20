# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Replaced the complete production backend with Go and consolidated it under `backend/`.
- Kept the existing Svelte frontend contracts while moving authentication, chat, providers, files, knowledge, administration, backups, Terminal and HaloClaw to Go.
- Replaced the runtime image with a stripped Go binary and static frontend in distroless; server-side Python and its packaging/workflows were removed.
- Added 256 MB host resource limits, image-content checks, health/memory gates and Go-only Docker publishing to `iwvw/halowebui`.
- Updated installation, operations, security, contribution and refactor documentation for the Go-only architecture.

## [0.0.1] - 2026-03-22

### Highlights

- **多模型统一接入**: 支持 OpenAI、Gemini、Anthropic、Grok、Ollama 以及兼容 OpenAI 协议的第三方服务，在一个界面中统一管理模型、密钥与连接。
- **灵活的联网与知识能力**: 内置 HaloWebUI 搜索、模型原生联网、网页加载、文件解析和知识库检索，适合日常问答、资料整理与复杂研究。
- **可控的工具调用体系**: 提供兼容、原生、关闭等工具调用模式，并支持 MCP、内置工具、技能和函数，让用户按场景决定是否启用自动工具能力。
- **本地优先的数据体验**: 用户、配置、聊天历史、文件和向量数据默认保存在本地托管的服务中，便于私有化部署、迁移和备份。
- **轻量部署与扩展**: 兼顾本地开发、Docker 镜像和服务器部署，保留简洁默认体验，同时为高级用户提供更完整的管理与扩展入口。

### Experience

- **面向中文用户优化**: 默认文案、设置项和关键提示更贴近中文使用习惯，减少理解成本。
- **清晰的管理边界**: 管理员配置、用户个人连接、聊天侧开关和运行时工具能力分层呈现，避免误操作。
- **更适合长期使用**: 强调稳定、可迁移、可验证的产品能力，而不是把内部开发记录直接暴露给普通用户。
