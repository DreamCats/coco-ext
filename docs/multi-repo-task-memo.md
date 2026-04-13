# coco-flow 多仓任务架构备忘录

> 日期：2026-04-11
> 状态：Memo
> 目标：明确 `coco-flow` 在支持多仓联动需求时，task 该如何建模、产物该放哪里，以及为什么当前阶段先不引入 `workspace-id`。当前技术名与命令名仍为 `coco-ext`。

---

## 1. 背景

当前 `coco-flow` 的 PRD 工作流主要围绕单仓场景构建：

- `task` 目录最初落在某个仓库的 `.livecoding/tasks/`
- `refine / plan / code / reset / archive` 也都围绕单仓执行

这在简单需求和中等需求里是可行的，但随着使用深入，会越来越频繁遇到以下场景：

- 需求同时涉及仓库 A 和仓库 B
- 一个功能要同时改业务服务仓库和公共 SDK 仓库
- 一个 task 需要多个仓库分别执行 `code`

这时，如果仍然把 task 强绑定到某一个仓库，会出现模型上的歧义：

- task 究竟属于仓库 A 还是仓库 B？
- `design.md` / `plan.md` 是单仓计划还是多仓总计划？
- `reset` / `archive` 应该只影响一个仓库，还是影响所有仓库？

因此，多仓需求不能继续简单套用“单仓 task”模型。

---

## 2. 结论

当前阶段推荐的演进方向是：

**从“仓库内 task”升级到“全局 task”，但暂不引入 `workspace-id`。**

也就是说：

- task 不再强绑定到某一个 repo 的 `.livecoding/tasks/`，而是提升到 `~/.config/coco-ext/tasks/`
- task 的主记录提升到全局目录
- 每个 task 显式声明它关联的 repo 列表
- 各 repo 的 code 结果、branch、worktree、commit 分别记录在 task 下

推荐目录结构：

```text
~/.config/coco-ext/tasks/<task-id>/
├── task.json
├── prd.source.md
├── prd-refined.md
├── design.md
├── plan.md
├── repos.json
├── repo-a.code-result.json
└── repo-b.code-result.json
```

---

## 3. 为什么当前阶段先不引入 `workspace-id`

### 3.1 `workspace-id` 解决的是“上下文分组”，不是“多仓 task 最小可行性”

`workspace-id` 的价值在于：

- 管理长期稳定的一组 repo
- 对 repo 组合做上下文命名和隔离
- 支撑未来全局 UI 的顶层空间选择

这些需求确实存在，但它们并不是“支持多仓 task”的最小前提。

多仓 task 最小需要的只是：

- 一个全局 task 存储位置
- task 里记录关联 repo
- task 里分别记录各 repo 的执行结果

如果此时马上引入 `workspace-id`，会把模型一次性复杂化。

### 3.2 当前最需要解决的是 task 归属，而不是空间治理

你们当前真实痛点是：

- 一个 task 同时涉及多个 repo 时，产物放哪里最合适
- UI 顶层该先看 task 还是先看 repo

这本质上是 **task 归属问题**。

而 `workspace-id` 更偏向 **空间治理问题**。

在当前阶段，先解 task 归属更重要。

### 3.3 可以保留未来升级空间

先采用：

```text
~/.config/coco-ext/tasks/<task-id>
```

并不会阻断未来升级。

如果将来真的出现以下需求：

- 需要管理长期稳定的 repo 组合
- 需要全局 UI 顶层选择“某个 workspace”
- 需要按 workspace 隔离任务历史

再升级到：

```text
~/.config/coco-ext/workspaces/<workspace-id>/tasks/<task-id>
```

会更自然。

---

## 4. 推荐的第一阶段多仓模型

### 4.1 顶层对象

当前阶段建议明确三类对象：

- `repo`：代码容器
- `task`：工作对象
- `repo-result`：某个 task 在某个 repo 上的执行结果

这里暂时不引入 `workspace` 作为一级对象。

### 4.2 task 主记录

`task.json` 建议至少包含：

- `task_id`
- `title`
- `status`
- `source_type`
- `primary_repo`
- `repos`
- `created_at`
- `updated_at`

其中：

- `primary_repo`：用于表达“这个 task 当前主要由哪个 repo 发起/查看”
- `repos`：表达这个 task 涉及的所有仓库

### 4.3 repos.json

建议单独落一个 `repos.json`，用于记录 repo 维度信息，例如：

```json
{
  "primary_repo": "live_pack",
  "repos": [
    {
      "id": "live_pack",
      "path": "/path/to/live_pack",
      "status": "coded",
      "branch": "prd_20260411-xxxx",
      "worktree": "/path/to/.coco-ext-worktree/live_pack-xxxx/20260411-xxxx",
      "commit": "abc1234"
    },
    {
      "id": "live_sdk",
      "path": "/path/to/live_sdk",
      "status": "planned"
    }
  ]
}
```

这样可以把“总 task”和“各 repo 执行情况”明确分开。

---

## 5. 状态机建议

多仓 task 建议拆成两层状态：

### 5.1 task 总状态

- `initialized`
- `refined`
- `planned`
- `coding`
- `partially_coded`
- `coded`
- `archived`

### 5.2 repo 子状态

对每个 repo 单独记录：

- `pending`
- `planned`
- `coding`
- `coded`
- `failed`
- `archived`

举例：

- repo A：`coded`
- repo B：`planned`

那么 task 总状态应为：

- `partially_coded`

这比单仓状态机更能表达真实过程。

---

## 6. 产物放哪里更合适

第一阶段建议：

- 统一放在全局 task 目录下
- 不再要求某一个 repo 作为 task 的唯一主存储位置

原因：

- `prd-refined.md`、`design.md`、`plan.md` 天生是 task 级产物，不是 repo 级产物
- `code-result` 则是 repo 级结果，应该挂在 task 下但分文件存储

也就是说：

- 文档产物：task 级
- code result：repo 级
- task 状态：task 级
- branch / worktree / commit：repo 级

---

## 7. 对 Web UI 的影响

如果后续 UI 走多仓 task 模型，页面结构建议是：

### 一级：Task List

展示：

- task 标题
- 总状态
- 关联 repo 数量
- 当前主 repo
- 更新时间

### 二级：Task Detail

展示：

- task 级文档产物
- repo 分组结果
- 每个 repo 的 branch / worktree / commit / status

也就是说，详情页里需要新增一块：

**Repo Scope / Repo Results**

而不是像单仓版本那样默认只有一个 branch / worktree / commit。

---

## 8. 什么时候再引入 workspace-id

以下条件同时出现时，再考虑从“全局 task”升级到“workspace”模型：

- 需要维护长期稳定的 repo 组合
- 不同 repo 组合之间需要隔离 task 历史
- 全局 UI 顶层要先选“工作空间”，再选 repo / task
- 需要对 workspace 做命名、收藏和共享

在这之前，不建议为了架构完美而提前引入 `workspace-id`。

---

## 9. 推荐行动

当前阶段建议按以下顺序推进：

1. UI mock 先支持多仓 task 视图
2. 在 docs 中明确多仓 task 的数据模型
3. CLI 仍以单仓 workflow 为主，不立刻改造为多仓执行
4. 等单仓 UI 和数据结构稳定后，再决定是否实现全局 task 存储

---

## 10. 结论

当前阶段，最合适的策略不是立刻引入 `workspace-id`，而是：

**先从单仓 task 演进到全局 task，再在需要时升级到 workspace。**

这条路线更稳、更清晰，也更符合你们现在的真实问题。
