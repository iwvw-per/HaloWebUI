# 安全策略

HaloWebUI 是自托管应用。用户数据、聊天、Provider 密钥、上传文件和备份默认保存在部署者控制的 `/app/data` 中。

## 支持版本

| 版本 | 状态 |
| --- | --- |
| `main` | 支持 |
| `codex/backend-refactor` | 重构验证分支，支持测试和反馈 |
| 其他旧版本 | 不保证 |

## 报告漏洞

请通过 GitHub 仓库的 Security Advisories 或私密漏洞报告渠道提交，不要公开发布可直接利用的细节。报告至少应包含受影响版本、部署方式、复现步骤、影响范围和修复建议；请先删除所有真实密钥、Cookie、JWT、用户数据和备份内容。

我们不会要求漏洞报告者把敏感材料发送到第三方聊天平台。普通功能问题请使用 Issue 模板，不要把普通报错标记为安全漏洞。

## 部署安全基线

- 生产环境设置高熵且稳定的 `WEBUI_SECRET_KEY`。
- 使用 HTTPS，并设置 `WEBUI_AUTH_COOKIE_SECURE=true`。
- 仅向受信网络暴露管理、SCIM、Terminal 和 HaloClaw 接口。
- 定期备份 `/app/data`，限制备份下载权限和保留时间。
- 不要把 Provider 密钥写入 Dockerfile、Compose 文件、Issue、日志或前端 bundle。
- 默认关闭 WebSocket 和本地重型运行时；只在明确需要时启用。

最后更新：2026-07-20
