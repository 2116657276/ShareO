# ADR-003：PostgreSQL pgvector 向量存储

- 日期：2026-08-11
- 状态：已接受

## 背景

图片向量与文本块向量需要 512 维余弦近邻检索、按帖子删除和幂等替换。当前本机运行目标是 Homebrew PostgreSQL 17，业务数据已经以 PostgreSQL 为唯一真相；继续维护独立向量服务会增加本机网络、模型下载和数据清理的故障面。

## 候选方案

1. PostgreSQL pgvector：复用同一实例，以独立 `ai` schema 和 AI 账号隔离派生数据。
2. 独立向量服务：需要额外进程、端口和存储生命周期。
3. Redis 向量检索：复用缓存服务，但不适合本项目的持久化与索引管理。

## 决定

采用 PostgreSQL 17 的 pgvector 扩展。建立 `ai.image_embeddings` 与 `ai.post_chunk_embeddings` 两张派生表，向量列固定为 `vector(512)`，建立 `vector_cosine_ops` HNSW 索引。Go 使用 `shareo_app` 读写 `public` 业务 schema；Python 使用 `shareo_ai` 只读写 `ai` schema。Python 通过 Psycopg 3 异步连接池访问数据库，保留现有 `ImageVectorStore`、`TextVectorStore` 方法语义。

## 理由

- 本机只需维护 PostgreSQL、Redis 和 MinIO，避免 Qdrant/Compose/Colima/OrbStack 的网络与生命周期问题。
- 业务和派生数据共享事务数据库的连接与备份边界，但通过 schema、账号和权限实现写入隔离。
- `<=>` 余弦距离、HNSW 和唯一约束覆盖当前图片搜图、RAG、upsert、删除和替换需求；派生表可由 approved 帖子回填重建。

## 后果与边界

- 向量检索与业务数据库共享资源，规模扩大时需要单独评估连接池、索引和磁盘容量。
- AI 账号不持有 `public` 表或 MinIO 凭证；公开结果仍由 Go 按 `approved AND is_deleted=0` 做最终校验。
- 旧 Qdrant 存储可以作为人工回滚材料保留，但新运行时、启动脚本和 readiness 完全不访问它。
