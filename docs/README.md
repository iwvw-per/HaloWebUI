# HaloWebUI 文档

本目录只维护与当前 Go-only 架构一致的文档：

- [`backend.md`](backend.md)：Go 后端目录、配置、资源预算、验证和发布。
- [`prd/backend-refactor.md`](prd/backend-refactor.md)：完整重构 PRD、实施状态和验收记录。
- [`CONTRIBUTING.md`](CONTRIBUTING.md)：本地开发、测试和提交要求。
- [`SECURITY.md`](SECURITY.md)：支持范围和漏洞报告流程。
- [`mobile-access.md`](mobile-access.md)：PWA 和第三方移动客户端接入。
- [`apache.md`](apache.md)：Apache 反向代理示例。

安装入口见根目录 [`INSTALLATION.md`](../INSTALLATION.md)，运行故障见 [`TROUBLESHOOTING.md`](../TROUBLESHOOTING.md)。部署工件位于根目录 Docker/Compose 文件和 `kubernetes/manifest/`，仓库不维护 Helm Chart。
