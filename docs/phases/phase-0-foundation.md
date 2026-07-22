# Phase 0 — 轻量基线

## 目标

建立一个可一键启动、依赖清楚、文档简短的项目基线。

## 交付

- 12 表 MySQL schema。
- Go 主服务、Python AI 单进程、Redis、MinIO、Qdrant 的统一 Compose。
- `make up/down/reset/check/test-integration`。
- 删除收藏、转帖、话题、群聊和重复运维方式。

## 退出条件

新库初始化成功，保留功能测试通过，被删除路由为 404，文档和代码无残留引用。
