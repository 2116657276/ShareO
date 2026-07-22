# Phase 2 真实门禁阶段评审（2026-07-22）

> 评审范围: Phase 2 隔离搜图 E2E 前置门禁与测试脚本修复 | 状态: 未通过，阻塞 Phase 3

## 结论

阶段 0 未通过，不能进入 Phase 3 运行时代码实现。MySQL、Redis、Qdrant 和 MinIO 均已能启动，Go 服务和 AI API 也能完成启动；但 approved 图片在隔离 E2E 的 180 秒搜索窗口内没有命中，worker 索引收敛缺少可复核证据。

## 已验证

| 检查 | 结果 | 证据 |
|------|------|------|
| Go 静态门禁 | 通过 | `make check` 中 gofmt/vet/unit |
| Go 竞态检测 | 通过 | `go test -race ./...` |
| Python lint/format/unit | 通过 | Ruff、format、27 项非集成 pytest |
| Shell/Markdown 门禁 | 通过 | `bash -n`、62 个 Markdown 链接 |
| Qdrant/MinIO/真实 MySQL/Redis 启动 | 通过 | 隔离 E2E 前置检查通过 |
| Phase 2 隔离 E2E | 未通过 | approved 图片 180 秒内未命中 |
| 40 条图片评测 | 未运行 | 依赖真实 E2E 数据和人工标注 |

## 本轮修复

`scripts/test_image_search.sh` 的搜索和删除轮询原来将短暂的 HTTP 503 视为致命错误，在 `set -e` 下会提前退出。本轮改为把该响应视为可重试状态，继续等待索引或删除收敛。Shell 语法和完整本地门禁已重新通过。

## 阻塞问题

修复脚本后，搜索请求可以继续轮询，但 worker 在 180 秒内仍未产生可命中的图片向量。当前需要单独保留并检查 worker 日志，确认是 worker 模型加载、Go 图片代理、图片编码、Qdrant upsert 还是查询侧过滤导致未命中；在此问题解决并取得删除不可见、worker 重启重领和故障降级证据前，不得标记 Phase 2 完成。

## 后续动作

1. 增加 worker readiness/预热可观测性，确保 E2E 不只等待 AI API readiness。
2. 在隔离 E2E 中保留 worker 日志和索引状态，定位首个 upsert 失败点。
3. 修复后重新运行上传→审核→索引→命中→删除→不可见和 worker 重启 45 秒重领验证。
4. E2E 通过后再标注 `image_search_v1` 并运行质量/性能评测。

## 评审状态

- P0/P1: 未发现新的安全或数据破坏问题。
- 阻塞项: worker 索引真实收敛未取得证据。
- 阶段结论: **未通过**。
- Phase 3: **暂停，不进入下一阶段**。
