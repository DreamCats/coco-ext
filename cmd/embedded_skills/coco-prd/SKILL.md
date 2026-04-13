---
name: coco-prd
description: 当需要执行或排查 PRD → 计划 → 代码实现这条任务流时使用。适用于创建 task、查看状态、生成 plan、在隔离 worktree 中执行 code、以及 reset / archive 清理收尾。
---

# coco-prd

用于管理 `coco-flow` 的 PRD 工作流。当前技术名与命令名仍为 `coco-ext`；目标不是替代人工判断需求是否合理，而是把“什么时候用 `run`、什么时候拆成 `refine -> plan -> code`、失败后先看哪里”固定下来，避免每次重新摸索。

## 何时使用

- 用户要求从需求描述、文档文件或飞书链接创建 PRD task
- 需要查看当前 task 卡在哪个阶段，决定下一步执行什么命令
- 需要生成 `design.md` / `plan.md`
- 需要在隔离 worktree 中执行自动代码实现
- 需要放弃本轮 code 结果并重试
- 需要在结果确认后做归档收尾

## 默认做法

1. 输入明确且希望一键跑通时，优先使用 `coco-ext prd run -i ...`。
2. 需要人工 review 每一步产物时，改走 `refine -> plan -> code` 分步执行。
3. 在进入 `code` 前，先确认 `prd-refined.md`、`design.md`、`plan.md` 已生成。
4. `prd code` 会先创建隔离 worktree，再同步 task/context 产物并启动 agent。
5. 多仓 task 下，可通过 `prd code --repo <repo_id>` 只推进某一个仓库的 code 结果；需要整 task 顺序推进时可用 `prd code --all-repos`。
6. 如果结果不满意，执行 `prd reset` 清理 worktree 和分支，再重新 `prd code`。
7. 结果确认完成后，执行 `prd archive` 收尾。

## 常用命令

```bash
coco-ext prd run -i "需求描述或飞书链接"
coco-ext prd run -i "跨仓需求" --repo /path/to/repo-b
coco-ext prd run -i "跨仓需求" --repo /path/to/repo-b --all-repos

coco-ext prd refine --prd "需求描述"
coco-ext prd refine --prd ./docs/prd.md
coco-ext prd refine --prd https://bytedance.larkoffice.com/docx/xxx
coco-ext prd refine --task <task_id> --prd ./docs/prd.md
coco-ext prd refine --repo /path/to/repo-b --repo /path/to/repo-c --prd "跨仓需求"

coco-ext prd status
coco-ext prd status --task <task_id>
coco-ext prd list

coco-ext prd plan
coco-ext prd plan --task <task_id>
coco-ext prd code
coco-ext prd code --task <task_id>
coco-ext prd code --task <task_id> --repo <repo_id>
coco-ext prd code --task <task_id> --all-repos

coco-ext prd reset
coco-ext prd reset --task <task_id>
coco-ext prd reset --task <task_id> --repo <repo_id>
coco-ext prd archive
coco-ext prd archive --task <task_id>
coco-ext prd archive --task <task_id> --repo <repo_id>
```

## 参数约定

- 需要指定已有 task 时，CLI 统一使用 `--task`
- 当前命令行参数名不是 `--task_id`
- `prd refine` 可重复使用 `--repo` 声明额外关联仓库
- `prd code --repo` 表示“当前要推进这个 task 下的哪个 repo”；多仓 task 下必须显式传入
- `prd code --all-repos` / `prd run --all-repos` 表示按绑定顺序顺序执行所有 repo，失败即停
- `prd reset --repo` / `prd archive --repo` 表示只处理这个 task 下的某个 repo

## 选择策略

- 需求还只是自然语言输入，优先 `prd refine`
- 已有 task，但不确定现在卡在哪，先跑 `prd status`
- 需要看全量 task 列表和状态分布，使用 `prd list`
- 需求较简单且希望快速出结果，使用 `prd run -i ...`
- 多仓 task 希望保守推进时，使用 `prd run -i ... --repo ...`，默认只自动推进当前 repo
- 多仓 task 希望顺序跑完全部仓库时，使用 `prd run -i ... --repo ... --all-repos`
- 需要审阅方案或手动修正 plan，再分步执行 `prd plan` 和 `prd code`
- 对本轮 code 结果不满意，使用 `prd reset`
- 代码结果已确认，不再继续迭代，使用 `prd archive`

## 关键产物

- `~/.config/coco-ext/tasks/<task-id>/task.json`
- `~/.config/coco-ext/tasks/<task-id>/source.json`
- `~/.config/coco-ext/tasks/<task-id>/repos.json`
- `~/.config/coco-ext/tasks/<task-id>/prd.source.md`
- `~/.config/coco-ext/tasks/<task-id>/prd-refined.md`
- `~/.config/coco-ext/tasks/<task-id>/design.md`
- `~/.config/coco-ext/tasks/<task-id>/plan.md`
- `~/.config/coco-ext/tasks/<task-id>/code-result.json`
- `~/.config/coco-ext/tasks/<task-id>/code-results/<repo-id>.json`

## Worktree 约定

- `prd code` / `prd run` 的 code 阶段默认在主仓库同级目录创建隔离 worktree
- 路径形态：`<repo-parent>/.coco-ext-worktree/<repo-name>-<repo-hash>/<task-id>`
- agent 在 worktree 中读写文件、执行最小编译和 commit
- 多仓 task 下，每个 repo 各自有自己的 branch / worktree / commit / code-result
- `prd reset` / `prd archive` 会删除对应 worktree 和分支；如果带 `--repo`，则只处理该 repo 的执行现场

## 排查顺序

1. 先执行 `coco-ext prd status`，确认 task 当前状态和缺失产物。
2. 如果缺少 `prd-refined.md`，先回到 `prd refine`。
3. 如果缺少 `design.md` 或 `plan.md`，先回到 `prd plan`。
4. 如果 `prd code` 失败，先看终端中的 tool event 和编译错误，再读取 `code-result.json`。
5. 如果发现代码落在错误目录或本轮结果污染了工作区，优先执行 `prd reset`，不要手动混合清理。
6. 如果 worktree 或分支残留，再检查 `code-result.json` 中记录的 `worktree` 和 `branch`。

## 例外说明

- `prd code` 默认分支名是 `prd_<task_id>`，也可通过 `--branch` 覆盖。
- 多仓 task 默认仍以“逐个 repo 推进 code”为主；只有显式传入 `--all-repos` 才会顺序执行所有 repo。
- 如果 `plan` 判定复杂度为“复杂”，`prd run` 会停止在 plan 阶段，不自动进入 code。
- `prd code` 只做最小范围编译验证，不会默认跑全仓测试。
- 如果需求复杂度被 plan 判定为“复杂”，应优先人工拆分需求，而不是强行继续自动 code。
- 飞书链接 refine 依赖 `lark-cli`；无法拉取正文时，task 仍会创建，但需要先补充源内容。
