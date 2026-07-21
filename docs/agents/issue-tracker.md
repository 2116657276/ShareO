# Issue tracker: optional GitHub collaboration

项目任务状态以根目录 `TASK.md` 与 `docs/plan.md` 为准。GitHub Issues 不是强制看板；只有需要跨设备、多人协作或公开讨论时才使用。

## 使用约定

- 不因本地任务没有 GitHub Issue 而阻塞开发。
- Issue 的完成状态不能覆盖 `TASK.md` 的验收证据。
- 需要操作 GitHub 时使用 `gh` CLI，并从当前仓库 remote 推断目标仓库。
- 常用命令：`gh issue list`、`gh issue view <number> --comments`、`gh issue create`、`gh issue comment`、`gh issue close`。
- 需要分类时使用 `docs/agents/triage-labels.md` 中的五类标签。

## 发布到 Issue tracker

只有用户明确要求发布，或任务本身需要外部协作时才创建/修改 Issue。普通本地实现和审计直接更新 `TASK.md`、`docs/plan.md` 与证据记录。
