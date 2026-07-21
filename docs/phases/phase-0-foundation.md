# Phase 0 — 基建与可信底座

> 更新时间: 2026-07-21 | 状态: 本地完成，远端 CI 待确认

## 目标与当前基线

建立 Go/Python 双运行时、真实依赖、可靠队列、可复现迁移和统一门禁。实现提交为 `57b9615`，Homebrew 存储补充为 `7a96feb`。

## 工作包

- Go 1.25.1+ 与 Python 3.12/uv locked 工具链。
- MySQL、Redis、MinIO、Qdrant、AI API/worker 编排与 readiness。
- Redis Streams：成功才 ACK、30 秒 idle 重领、共 4 次处理、最终记录并 ACK，无 DLQ。
- Homebrew MySQL/Redis/MinIO 与独立 Docker Qdrant 的本地运行路径。
- `make check`、真实集成、race、Shell 和文档链接门禁。

## 接口与数据流

`/healthz` 只表示进程存活；`/readyz` 表示 Redis/Qdrant 可用。Go 生产 Streams，Python worker 是 AI 任务消费者。

## 失败模式

Redis 暂停时消息留在 pending；消费者重启通过 XAUTOCLAIM 恢复。Compose 和 Homebrew 数据目录严格隔离，MinIO 未运行时图片代理返回 404，但数据库记录不会消失。

## 测试矩阵与退出标准

- Go/Python 单元门禁、真实 Redis 重领、Compose config/up/ready/reset。
- API/IM curl、迁移只建结构、工作区与文档检查。
- 本地证据已完成；远端 CI 有可核验结果后关闭“待确认”。

## 证据矩阵

| 交付 | 提交 | 自动证据 | 环境证据 |
|------|------|----------|----------|
| 工具链/Streams/Compose | `57b9615` | `make check`、Redis 集成 | Compose ready/reset |
| Homebrew 存储 | `7a96feb` | Shell/文档检查 | users/images/keys/objects 均为 0 |

## 暂缓项

无功能开发；只等待远端 CI 证据和必要的版本维护。
