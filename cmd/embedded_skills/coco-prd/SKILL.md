---
name: coco-prd
description: 当需要执行或排查 PRD → 计划这条任务流时使用。适用于创建 task、查看状态、生成 plan，以及识别历史 task 中遗留的旧 code 状态。
---

# coco-prd

用于管理 `coco-ext prd` 工作流。目标不是替代人工判断需求是否合理，而是把“什么时候用 `run`、什么时候拆成 `refine -> plan`、失败后先看哪里”固定下来，避免每次重新摸索。

## 何时使用

- 用户要求从需求描述、文档文件或飞书链接创建 PRD task
- 需要查看当前 task 卡在哪个阶段，决定下一步执行什么命令
- 需要生成 `design.md` / `plan.md`
- 需要识别历史 task 中的 `coding/coded/archived/failed` 等旧状态

## 默认做法

1. 输入明确且希望一键跑通时，优先使用 `coco-ext prd run -i ...`。
2. 需要人工 review 每一步产物时，改走 `refine -> plan` 分步执行。
3. 在进入 `plan` 前，先确认 `prd-refined.md` 已生成。
4. `plan` 产物完成后，后续实现请转到迁移后的实现流程；当前 `coco-ext` 不再提供 `prd code/reset/archive`。
5. 若历史 task 存在旧 code 状态，默认按只读方式查看，不要再给出已删除命令。

## 常用命令

```bash
coco-ext prd run -i "需求描述或飞书链接"
coco-ext prd run -i "跨仓需求" --repo /path/to/repo-b

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
```

## 参数约定

- 需要指定已有 task 时，CLI 统一使用 `--task`
- 当前命令行参数名不是 `--task_id`
- `prd refine` 可重复使用 `--repo` 声明额外关联仓库

## 选择策略

- 需求还只是自然语言输入，优先 `prd refine`
- 已有 task，但不确定现在卡在哪，先跑 `prd status`
- 需要看全量 task 列表和状态分布，使用 `prd list`
- 需求较简单且希望快速出结果，使用 `prd run -i ...`
- 多仓 task 需要统一纳入方案范围时，使用 `prd run -i ... --repo ...`
- 需要审阅方案或手动修正 plan，再分步执行 `prd plan`
- 如果 status 显示历史 code 状态，明确说明这些状态仅做只读兼容，后续实现走迁移后的流程

## 关键产物

- `~/.config/coco-ext/tasks/<task-id>/task.json`
- `~/.config/coco-ext/tasks/<task-id>/source.json`
- `~/.config/coco-ext/tasks/<task-id>/repos.json`
- `~/.config/coco-ext/tasks/<task-id>/prd.source.md`
- `~/.config/coco-ext/tasks/<task-id>/prd-refined.md`
- `~/.config/coco-ext/tasks/<task-id>/design.md`
- `~/.config/coco-ext/tasks/<task-id>/plan.md`

## 排查顺序

1. 先执行 `coco-ext prd status`，确认 task 当前状态和缺失产物。
2. 如果缺少 `prd-refined.md`，先回到 `prd refine`。
3. 如果缺少 `design.md` 或 `plan.md`，先回到 `prd plan`。
4. 如果 status 显示旧的 `coding/coded/archived/failed` 状态，说明 task 来自历史 code 流程；当前 CLI 仅保留只读兼容。
5. 若需要继续实现，请转到迁移后的实现流程，而不是再建议 `prd code/reset/archive`。

## 例外说明

- 历史 task 里可能仍然带有 `branch/worktree/commit/code-result` 等旧产物，但这些已经不再由当前 CLI 更新。
- 如果需求复杂度被 plan 判定为“复杂”，应优先人工拆分需求，而不是把 plan 直接当成可自动落地实现。
- 飞书链接 refine 依赖 `lark-cli`；无法拉取正文时，task 仍会创建，但需要先补充源内容。
