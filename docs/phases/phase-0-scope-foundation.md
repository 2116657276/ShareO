# Phase 0 — 范围与工程基线

> 状态：已完成

## 目标

冻结一个适合独立开发、可一键启动且可持续验收的轻量项目基线。

## 当前基线

仓库使用单个 `001_init.sql` 建立 12 张业务表；根目录 Compose 编排 Go、MySQL、Redis、MinIO、Qdrant 和 Python AI 六个服务。

## 范围与非目标

保留社区、私聊、搜图与 RAG Bot。收藏、转帖、话题、群聊、Agent、工具调用、微服务拆分和生产级多租户不属于范围。

## 依赖

Go 1.25.1+、Python 3.12、uv、Docker Compose，以及能够下载本地 embedding 模型的网络和磁盘空间。

## 工作包

- 冻结功能边界和 12 表 schema。
- 删除已裁剪功能的路由、模型、页面与查询。
- 合并 AI API/consumer 为单进程并统一 Compose、Makefile 和配置入口。
- 建立 Go、Python、Shell、文档、集成和 race 门禁。

## 接口与数据流

浏览器只访问 Go；Go 使用 MySQL、Redis 和 MinIO，并通过内部 token 调用 Python；Python 消费 Redis Streams、访问 Qdrant 和 LLM。

## 数据一致性

MySQL 只由 Go 写，Qdrant 只由 Python 写。业务表由外键和事务保证，向量数据通过幂等事件、回填与对账实现最终一致。

## 安全边界

配置和密钥通过环境变量注入；MinIO 与内部接口不直接暴露给浏览器；Bot 账号禁止登录。

## 失败模式

AI 或向量依赖失败不得阻塞社区和普通私聊；冷启动竞态由 Compose healthcheck 与 capability readiness 暴露。

## 测试矩阵

新库结构、被删除路由 404、保留功能回归、Compose 配置、冷启动、Go/Python/Shell/文档门禁。

## 退出标准

全新卷可启动六个服务；数据库恰有 12 表；裁剪功能无活跃引用；全部基础门禁通过。

## 提交与环境证据

- `a2c0a0f`：冻结轻量项目文档范围。
- `552def0`：建立轻量 schema 与最终功能边界。
- `b942643`：统一 AI 单进程和 Compose。
- 历史基线记录曾完成清空卷启动，两个 Stream 各保持一个 consumer；该记录不外推到后续 Phase 7C/8 的当前源码重建。

## 遗留项

最终 Demo 数据、评测和发布证据由 Phase 7 完成。
