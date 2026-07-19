# ADR-003: 向量数据库 —— Qdrant

- 日期: 2026-07-19
- 状态: 已接受

## 背景

图片向量与文本块向量需要专门的近邻检索存储，规模预估 1 万~10 万向量级（毕设规模），需支持按 payload 过滤（post_id、时间等）与删除。主库是 MySQL，无 pgvector 选项。

## 候选方案

1. Qdrant —— Rust 编写，单容器部署
2. Milvus —— 国产明星项目（Zilliz），standalone 需多个依赖容器
3. Redis 向量检索 —— 复用现有 Redis
4. Elasticsearch/OpenSearch kNN —— 重型搜索引擎

## 决定

选 Qdrant，单 Docker 容器部署。两个 collection：`images`（Chinese-CLIP 图片向量，512 维）、`post_chunks`（BGE 文本向量）。只由 ai-service 访问。

## 理由

- 单容器即起、API 干净、文档好，笔记本上运维负担最小——独立开发者的时间是最稀缺资源。
- Payload 过滤、按条件删除等本项目刚需功能完备。
- Milvus 认知度高但 standalone 部署重（etcd + MinIO + 本体），对本规模是纯负担；本 ADR 即为答辩时"为何不用 Milvus"的书面回答。
- Redis 向量能力可用但生态与论文素材薄；ES 体量完全不匹配。

## 后果与代价

- 比 Milvus 少一个"大厂名字" → 用选型论证的严谨性弥补，答辩讲清楚工程权衡反而更加分。
- 新增一个基础组件 → 已纳入 docker-compose 统一管理。
