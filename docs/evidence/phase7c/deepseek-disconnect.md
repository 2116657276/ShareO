# Phase 7C 真实 DeepSeek provider 断开复核

## 执行信息

- 日期：2026-07-26
- 环境：现有 `shareo` Compose 项目，仅重建 `ai-service`，不修改 MySQL、Redis、MinIO、Qdrant 或 Demo 数据。
- 方法：通过一次性环境变量将 DeepSeek provider 地址覆盖为不可达端口；未修改 `.env`，未输出或归档 API Key。
- 恢复：测试结束后使用未覆盖的 Compose 配置重建 `ai-service`，重新预热 RAG readiness。

## 结果

| 检查项 | 实际结果 |
|---|---:|
| AI `/healthz` | HTTP 200 |
| AI `/readyz/rag` | HTTP 503 |
| 直接 RAG 请求 | HTTP 503 |
| 普通私聊发送 | HTTP 200 |
| Bot 固定兜底 | 成功 |
| 恢复后 AI `/healthz` | HTTP 200 |
| 恢复后 RAG readiness | HTTP 200 |

该复核证明 provider 不可用不会阻塞普通私聊，Bot 会进入固定兜底；没有把外部 provider 的失败伪装成成功回答。
