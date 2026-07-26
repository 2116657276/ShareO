# Phase 7C 冷启动与数据收敛证据

## 执行信息

- 日期：2026-07-26
- Phase 7B 基线提交：`a3b02e9e51004ef5cf4bb25f1d33d5bb92298768`
- Compose 项目：`shareo`
- 运行方式：先执行 `make reset CONFIRM=YES` 清理并重建数据卷；由于 Docker 构建阶段访问 Go 依赖代理持续卡住，停止未完成的构建后使用已验证的本地镜像执行 `docker-compose up -d --no-build --wait`。
- 保护措施：在重启前将 MySQL、MinIO、Qdrant 数据卷导出到工作区外的临时备份目录；备份未进入 Git。

## 服务与 readiness

六个服务均已启动并保持健康：Go app、Python ai-service、MySQL、Redis、MinIO、Qdrant。复核结果如下：

| 检查项 | 实际结果 |
|---|---:|
| Go `/healthz` | HTTP 200 |
| AI `/readyz/image-search` | HTTP 200 |
| AI `/readyz/rag` | HTTP 200 |
| Compose 服务数量 | 6 |

## Demo seed 与对象/索引数量

在服务恢复后，使用同一 Demo 配置连续完成两次成功的 `make demo-seed`。第一次成功运行复用已创建的前 21 篇帖子并补齐至 26 篇；第二次成功运行复用全部 26 篇，未新增帖子或图片对象。随后执行两套数据集静态校验和数据库可见性校验，均通过。

| 数据 | 期望 | 实际 |
|---|---:|---:|
| approved 且未删除帖子 | 26 | 26 |
| `post_images` | 26 | 26 |
| Qdrant `images` points | 26 | 26 |
| Qdrant `post_chunks` points | 26 | 26 |
| MinIO 期望对象 | 78 | 78 |
| MinIO 实际对象 | 78 | 78 |
| MinIO 孤儿对象 | 0 | 0 |
| 图片搜图数据集 | 40 条，静态/可见性通过 | 通过 |
| RAG 数据集 | 30 条，静态/可见性通过 | 通过 |

## 备注

本次冷启动的“镜像构建”受本机 Docker 网络访问 Go 依赖代理影响，未将构建失败伪装成通过；服务启动、readiness、数据收敛和运行时验证均使用已有且已通过 E2E 的镜像完成。备份目录位于工作区外，交付时不包含在仓库中。
