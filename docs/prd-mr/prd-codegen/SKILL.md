---
name: prd-codegen
description: "按编码计划生成代码 - 读取 plan.md 逐文件编码 + 编译验证 + 提交 MR。当用户说 prd-codegen / 开始编码 / 执行编码计划 / 按计划写代码 / 生成代码 / 提 MR 时触发。"
---

# /prd-codegen - 按计划编码 + 验证 + 提 MR

**读取 Phase A 的编码计划，逐文件生成代码，编译验证，提交 MR。**

这是全链路的最后一步。plan.md 已经明确了改什么文件、怎么改、参考什么样例。本 skill 的目标是严格按计划执行，不要"创新"。

## 触发方式

```bash
# 自动读取最近 approved 的 plan.md
/livecoding:prd-codegen

# 指定 task
/livecoding:prd-codegen --task 20260310-add-countdown
```

## 前置条件

- `.livecoding/tasks/{task-id}/plan.md` 存在且 `status: approved`
- 如果 status 不是 approved → 提示："plan.md 未确认，请先 review 并确认编码计划"

## 产物

写入 `.livecoding/tasks/{task-id}/` 目录：

| 文件 | 内容 |
|------|------|
| `changelog.md` | 实际改动记录（每个文件改了什么、编译结果、修复次数） |
| `mr.md` | MR 信息（分支、提交信息、MR 链接） |

## 核心流程

```
读取 plan.md（校验 status: approved）
      │
      ├─ Step 1: 准备工作分支
      │    └─ 基于基准分支创建工作分支
      │
      ├─ Step 2: 逐文件编码
      │    └─ 按 plan.md 文件清单顺序
      │    └─ 每个文件：读样例 → 写代码 → 增量编译
      │
      ├─ Step 3: 全量验证
      │    └─ 所有文件改完后全量编译 + vet
      │
      ├─ Step 4: 提交
      │    └─ commit（遵循 plan.md 预设的提交信息）
      │    └─ push + 开 MR
      │
      └─ Step 5: 产物归档
           └─ 写 changelog.md + mr.md
           └─ plan.md status → completed
```

## Step 1: 准备工作分支

### 1.1 切换到基准分支

```bash
BASE_BRANCH=$(grep "基准分支" plan.md | awk -F'：' '{print $2}' | tr -d ' ')
git checkout "$BASE_BRANCH"
git pull origin "$BASE_BRANCH"
```

### 1.2 创建工作分支

```bash
WORK_BRANCH=$(grep "工作分支" plan.md | awk -F'：' '{print $2}' | tr -d ' ')
git checkout -b "$WORK_BRANCH"
```

如果工作分支已存在（可能是之前的失败尝试），提示用户选择：
- 继续在已有分支上工作
- 删除旧分支重新创建

## Step 2: 逐文件编码

**参考**：[编码执行指南](references/coding-guide.md)

### 2.1 执行顺序

严格按 plan.md 中的文件清单顺序执行（通常是 model → converter → service → handler）。

### 2.2 每个文件的处理

对 plan.md 中列出的每个文件：

#### a) 读取上下文

1. 读 plan.md 中该文件的改动描述、上下文、参考样例
2. 读 `.livecoding/context/patterns.md` — 确认代码模式
3. 读 `.livecoding/context/gotchas.md` — 确认已知风险与隐式约定
4. 读参考样例的实际代码（plan.md 中标注的文件和函数）
5. 读目标文件的当前内容（如果是修改已有文件）

#### b) 生成代码

- **严格按照 plan.md 描述执行**，不要自由发挥
- **严格按照参考样例的风格**，照着写
- **遵守仓库既有风格**：命名、错误处理、日志、import 分组；必要时参考 AGENTS.md 和 context 中的 gotchas/patterns

#### c) 增量编译验证

每改完一个文件，立即编译：

```bash
# 编译改动的 package
go build ./{改动的package}/...

# 静态检查
go vet ./{改动的package}/...
```

#### d) 编译失败处理

**参考**：[编译失败处理](references/build-fix.md)

```
编译失败
  ├─ 分析错误信息
  ├─ 尝试自动修复（最多 3 次）
  │   ├─ 语法错误 → 修复
  │   ├─ import 错误 → 修复
  │   ├─ 类型错误 → 修复
  │   └─ 其他 → 记录错误，标记需人工介入
  └─ 3 次仍失败 → 暂停，输出错误信息，等待人工指导
```

#### e) 记录实际改动

每个文件完成后，记录到 changelog 素材：
- 实际改了什么（vs plan.md 预期的改动）
- 编译是否一次通过
- 如果有自动修复，修了几次、改了什么

### 2.3 进度输出

每完成一个文件，输出进度：

```markdown
✅ [1/3] `live/popcard/model/popcard.go` — 添加 Countdown 字段，编译通过
✅ [2/3] `live/popcard/converter/popcard_converter.go` — 添加字段映射，编译通过
⏳ [3/3] `live/popcard/handler/get_popcard.go` — 进行中...
```

## Step 3: 全量验证

所有文件改完后，做一次全量验证：

```bash
# 编译所有涉及的 package
go build ./{package1}/... ./{package2}/...

# 静态检查
go vet ./{package1}/... ./{package2}/...

# 运行测试（如果 plan.md 中有测试相关改动）
go test ./{package}/... -count=1 -timeout=60s
```

**不要执行 `go build ./...` 或 `go test ./...`（全仓编译/测试）。**

全量验证失败时，按 Step 2d 的流程处理。

## Step 4: 提交

### 4.1 Commit

```bash
# 使用 plan.md 中预设的提交信息
git add .
git commit -m "{plan.md 中的提交信息}"
```

**提交信息规范**（Conventional Commits）：
```
{type}({scope}): {简述}

{详细描述}
```

### 4.2 Push

```bash
git push origin "$WORK_BRANCH"
```

### 4.3 开 MR

根据团队 git 平台：

**公司内部（code.byted.org）：**
- push 后提示用户手动在 web 上开 MR
- 或使用公司 CLI 工具（如有）

**GitHub（个人项目）：**
```bash
gh pr create --title "{提交信息第一行}" --body "{详细描述}" --base "$BASE_BRANCH"
```

**输出 MR 创建方式：**
```markdown
代码已推送到 `{WORK_BRANCH}`

请在 code.byted.org 上创建 MR：
- 源分支：{WORK_BRANCH}
- 目标分支：{BASE_BRANCH}
- 标题：{type}({scope}): {简述}
```

## Step 5: 产物归档

### 5.1 写 changelog.md

```markdown
# 编码执行记录

## 基本信息
- **任务 ID：** {task-id}
- **工作分支：** {WORK_BRANCH}
- **基准分支：** {BASE_BRANCH}
- **执行时间：** {YYYY-MM-DD HH:MM}

## 改动记录

### 1. `{文件路径}`
- **计划改动：** {plan.md 中的描述}
- **实际改动：** {实际做了什么}
- **编译结果：** ✅ 一次通过 / ⚠️ 修复 N 次后通过
- **偏差说明：** {如果实际改动与计划不一致，说明原因}

### 2. `{文件路径}`
- ...

## 验证结果
- **增量编译：** ✅ 全部通过
- **全量编译：** ✅ 通过
- **静态检查：** ✅ 通过
- **测试：** ✅ 通过 / ⏭ 跳过（无测试改动）

## 统计
- **改动文件数：** {N}
- **新增行数：** {N}
- **删除行数：** {N}
- **编译修复次数：** {N}
```

### 5.2 写 mr.md

```markdown
# MR 信息

- **MR 链接：** {URL 或"待手动创建"}
- **源分支：** {WORK_BRANCH}
- **目标分支：** {BASE_BRANCH}
- **提交信息：** {commit message}
- **Review 状态：** pending
```

### 5.3 更新 plan.md 状态

将 plan.md 中的 `status: approved` 改为 `status: completed`。

### 5.4 完成输出

```markdown
## ✅ 编码完成

### 改动概览
- **文件数：** {N} 个
- **新增/删除：** +{N}/-{N} 行
- **编译修复：** {N} 次
- **工作分支：** `{WORK_BRANCH}`

### 验证结果
- ✅ 增量编译通过
- ✅ 全量编译通过
- ✅ go vet 通过

### MR
{MR 链接或创建指引}

### 下一步
1. Review MR diff
2. 补充测试（如需要）
3. 通过后合入
```

## 注意事项

1. **严格按 plan.md 执行** — 不要自由发挥、不要加 plan 里没有的改动
2. **照着样例写** — plan.md 中标注了参考样例，代码风格必须与样例一致
3. **只编译改动的 package** — 禁止 `go build ./...`
4. **编译失败最多重试 3 次** — 超过 3 次就暂停等人
5. **不要跳过编译** — 每个文件改完都要编译，不要等全部改完才编译
6. **Commit 前确认 git diff** — 确保没有多余的改动（debug 日志、注释掉的代码等）
7. **不要改 plan 之外的文件** — 如果发现需要额外改动，暂停并告知用户
8. **保留 changelog** — 即使一切顺利也要写 changelog.md，这是量化数据的来源

## 异常处理

| 异常 | 处理 |
|------|------|
| plan.md 不存在 | 提示先执行 prd-assess |
| plan.md status ≠ approved | 提示先确认编码计划 |
| 基准分支有冲突 | 提示用户 rebase/merge 后重试 |
| 编译失败 3 次 | 暂停，输出错误信息，等待人工指导 |
| 发现需要额外改动 | 暂停，告知用户，建议更新 plan.md |
| Push 失败 | 检查网络/权限，提示用户手动 push |

## 与其他 skill 的关系

```
context init / update → context/
prd-refine → prd-refined.md
prd-assess → assessment.md + plan.md (status: approved)
                    ↓
              prd-codegen（本 skill）
                    ↓
              代码改动 + commit + push + MR
              changelog.md + mr.md
              plan.md (status: completed)
```

**prd-codegen 是执行层 — 不做决策，严格按 plan 执行。所有决策在 Phase A 完成。**
