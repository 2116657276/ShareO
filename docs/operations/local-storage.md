# 本地存储与数据重置

## Homebrew MinIO

本地 Homebrew 路径使用独立于 Docker Compose 的 MinIO 实例：

- 数据目录：`$HOME/minio_data`
- S3 API：`http://127.0.0.1:9000`
- Console：`http://127.0.0.1:9001`
- 默认 bucket：`shareo`
- 默认开发凭证：`minioadmin / minioadmin`

当前 Homebrew MinIO formula 没有可用的 `brew services` 定义（`Schedulable: false`），不要把 `brew services start minio` 当作启动成功的依据。项目使用 Homebrew 安装的二进制，并由 `scripts/start_minio_homebrew.sh` 启动：

```bash
brew install minio/stable/minio
bash scripts/start_minio_homebrew.sh
curl --noproxy '*' --fail http://127.0.0.1:9000/minio/health/live
# 或：make brew-minio-ready

若终端设置了 HTTP 代理，健康检查脚本会显式绕过代理访问本机 `127.0.0.1`；浏览器或其他客户端也应把 `127.0.0.1,localhost` 加入 `NO_PROXY`。
```

`start.sh` 会复用相同的数据目录和默认端口。Go 的 `/api/v1/images/...` 是图片代理；MinIO 未运行时，数据库中的图片 URL 不会消失，但代理会返回 404，页面会显示图片失效。

Compose 的 `minio_data` named volume 与 `$HOME/minio_data` 完全不同。不要用 Compose 的 reset/clean 命令判断或清理 Homebrew 图片。

## 清空本地开发数据

旧数据不需要时，使用带显式确认的命令：

```bash
brew install minio/stable/mc
CONFIRM=YES bash scripts/reset_homebrew_data.sh
```

该命令只清理本机 `shareo` MySQL 数据库、Redis DB 0 和 MinIO `shareo` bucket；保留迁移、代码、配置、MinIO 系统目录和 Docker Compose 卷。命令不会执行 seed 或 cleanup migration。

清理命令会校验目标地址必须是 `127.0.0.1:3306`、`127.0.0.1:6379`、`127.0.0.1:9000`，并通过 `mc rm --recursive --force` 清空 bucket，不直接删除 MinIO 内部目录。需要回填已审核帖子时使用 `make backfill-index`；该命令只投递 Redis Streams，不直接写 Qdrant。
