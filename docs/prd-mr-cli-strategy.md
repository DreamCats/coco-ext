# coco-flow CLI 与 Coco CLI 协同方案

> 日期：2026-03-31
> 状态：Draft
> 目标：说明为什么 `coco-flow` 必须先做成稳定 CLI，再由 Coco CLI 作为交互入口承载 `PRD -> MR` 流程。当前技术名与命令名仍为 `coco-ext`。

---

## 1. 背景与问题

当前团队的真实使用偏好是：

- 同事更喜欢在 `coco cli` 里用自然语言交互
- 同事不希望记很多命令，更不希望手动串很多步骤
- 老板更关心的是：能不能把 `PRD -> MR` 做成一条可讲故事、可规模化复制的流水线

这会带来一个核心设计问题：

**如果最终入口是 Coco CLI，为什么还要继续建设 `coco-flow` CLI？**

答案是：

**`coco-flow` 是基础设施，Coco CLI 是交互入口。**

两者分工不同：

- `coco-ext` 负责提供稳定、可脚本化、可测试、可复用的原子能力
- `Coco CLI` 负责接收自然语言意图，并编排调用 `coco-ext`

如果没有稳定 CLI，Coco CLI 只能靠 prompt 临时拼流程，结果会不稳定、不可测试、不可运营。

---

## 2. 目标

这套方案要同时满足 4 个目标：

1. **同事使用简单**
   同事记住少量入口即可，不需要背完整命令树。

2. **AI 调用稳定**
   Coco CLI 和 repo 内 skill 不直接“脑补执行流程”，而是调用 `coco-ext` 提供的稳定能力。

3. **流程可恢复**
   `PRD -> MR` 任一步失败后，可以从中间状态继续，而不是从头再来。

4. **价值可汇报**
   CLI 天然可以记录状态、产物、耗时与结果，后续便于做指标与运营故事。

---

## 3. 核心设计原则

### 3.1 CLI 是能力层，不是最终用户界面

`coco-ext` 的第一责任不是“让每个同事背命令”，而是提供：

- 明确输入
- 稳定输出
- 可预期副作用
- 可诊断失败原因

因此，CLI 的主要消费方包括：

- Coco CLI
- repo 内 skills
- shell alias / script
- CI / 自动化任务

### 3.2 入口命令少，底层命令全

对人暴露的命令应尽量少，对 AI 调用的底层命令应完整。

换句话说：

- **用户入口命令**：少而稳
- **内部编排命令**：全而清晰

### 3.3 自然语言意图不直接映射为 prompt，而是映射为 CLI 调用

例如用户说：

- “帮我提交并推送”
- “帮我把这个 PRD 走到 MR”
- “帮我看下环境有没有问题”

理想行为不是让 AI 直接即兴发挥，而是：

- 先识别意图
- 再调用对应的 `coco-ext` 子命令
- 最后把结果翻译回用户语言

### 3.4 `PRD -> MR` 必须有状态机

`PRD -> MR` 不是一个单命令，而是一条长链路：

- refine
- assess
- approve
- codegen
- review
- push / mr

因此必须落状态，而不是靠会话记忆。

---

## 4. 最终产品形态

### 4.1 分层结构

```text
┌────────────────────────────────────────────┐
│                Coco CLI                    │
│  自然语言入口 / 交互式体验 / Skill 编排     │
└────────────────────────────────────────────┘
                    │
                    ▼
┌────────────────────────────────────────────┐
│                coco-flow                   │
│  稳定 CLI 能力层 / 状态机 / 产物管理        │
└────────────────────────────────────────────┘
                    │
                    ▼
┌────────────────────────────────────────────┐
│        git / daemon / hooks / skills       │
│ ~/.config/coco-ext/tasks / reports / logs  │
└────────────────────────────────────────────┘
```

### 4.2 用户最终感知

同事不需要掌握完整命令树，只需知道少量入口：

- `coco-ext install`
- `coco-ext doctor`
- `coco-ext submit`
- `coco-ext prd ...`

而在更常见的场景里，同事甚至不直接输入这些命令，而是在 Coco CLI 中说：

- “帮我提交并推送”
- “帮我从这个 PRD 走到 MR”
- “帮我看一下当前仓库环境”

然后由 Coco CLI 代理调用底层 CLI。

---

## 5. 命令分层设计

### 5.1 用户入口命令

这些命令是推荐给团队成员直接使用的。

| 命令 | 用途 | 面向谁 |
|------|------|--------|
| `coco-ext install` | 安装 hooks、skills | 新接入用户 |
| `coco-ext doctor` | 诊断环境并低风险修复 | 所有用户 |
| `coco-ext submit` | 基于 staged 变更自动生成 message、commit、push | 日常开发 |
| `coco-ext prd status` | 查看某个 PRD task 当前进度与下一步 | 日常开发 / TL |
| `coco-ext prd run` | 串联 `PRD -> MR` 主流程 | 高阶入口 |

### 5.2 内部编排命令

这些命令主要给 Coco CLI、skills 和脚本调用。

| 命令 | 用途 |
|------|------|
| `coco-ext push` | `git push` 成功后后台触发 review |
| `coco-ext review` | 运行分层 review pipeline |
| `coco-ext gcmsg` | 生成 commit message |
| `coco-ext prd refine` | PRD 完善 |
| `coco-ext prd assess` | 代码调研与复杂度评估 |
| `coco-ext prd approve` | 确认计划进入编码阶段 |
| `coco-ext prd codegen` | 按已确认 plan 编码 |
| `coco-ext prd mr` | 生成 MR 标题、描述、风险摘要 |

原则：

- 人主要记入口命令
- AI 主要调内部命令

---

## 6. `PRD -> MR` 的 CLI 化方案

### 6.1 命令树

建议新增一个 `prd` 子命令体系：

```bash
coco-ext prd refine
coco-ext prd assess
coco-ext prd approve
coco-ext prd codegen
coco-ext prd review
coco-ext prd mr
coco-ext prd status
coco-ext prd run
```

### 6.2 每个命令的职责

#### 复用 `coco-ext context ...`

这里**不新增** `coco-ext prd init`。

原因：

- 现有 `coco-ext context init / update / status / query` 已经承担“仓库上下文初始化与维护”的职责
- 如果再新增一套 `prd init`，会把 `context` 与 `prd` 两条线做重复
- `PRD -> MR` 的 Phase 0 应依赖现有 context 能力，而不是复制一套 init 语义

因此，推荐关系是：

- `coco-ext context init`：首次初始化仓库上下文
- `coco-ext context update`：增量刷新上下文
- `coco-ext context status`：检查 context 状态
- `coco-ext context query`：调研/评估阶段查询上下文

`prd` 子命令体系只负责任务流转与产物状态，不负责重复建设 context 初始化。

#### `coco-ext prd refine`

职责：

- 读取 PRD 输入
- 做结构化解析、5 维打分、探索式补问
- 输出 `prd-refined.md`

输入：

- `--prd`
- `--task`

输出：

- `~/.config/coco-ext/tasks/{task-id}/prd.md`
- `~/.config/coco-ext/tasks/{task-id}/prd-refined.md`

#### `coco-ext prd assess`

职责：

- 基于 `prd-refined.md` 做术语翻译、代码调研、复杂度评估
- 简单/中等场景下生成 `plan.md`

输出：

- `assessment.md`
- `plan.md`

#### `coco-ext prd approve`

职责：

- 显式将 `plan.md` 标记为可执行
- 写入审批人、时间、备注

原因：

- 这是人机协作链路中的关键控制点
- 避免 AI 未经确认直接进入编码

#### `coco-ext prd codegen`

职责：

- 读取已确认的 `plan.md`
- 逐文件执行编码
- 编译验证
- 输出 `changelog.md`

#### `coco-ext prd review`

职责：

- 对 `prd codegen` 结果运行标准 review 流程
- 将 review 结果和 task 绑定

输出：

- review report 路径
- task 内 review 摘要

#### `coco-ext prd mr`

职责：

- 基于 `prd-refined.md`、`assessment.md`、`plan.md`、`changelog.md`、`review report`
  生成：
  - MR 标题
  - MR 描述
  - 风险摘要
  - 测试说明
  - 变更摘要

这是最接近老板关注点的“PRD-MR 一步落地”能力。

#### `coco-ext prd status`

职责：

- 展示当前 task 所处阶段
- 展示已完成产物
- 给出下一步推荐命令

#### `coco-ext prd run`

职责：

- 作为高阶入口串联：
  - refine
  - assess
  - 等待 approve
  - codegen
  - review
  - mr

注意：

- `run` 不应该绕过人工确认点
- 它更像“统一编排器”，而不是无脑一键到底

---

## 7. Task 状态机设计

建议每个 task 维护明确状态，而不是只靠文件是否存在推断。

### 7.1 状态列表

```text
initialized
  -> refined
  -> assessed
  -> approved
  -> coding
  -> reviewed
  -> mr_ready
  -> completed
```

### 7.2 状态含义

| 状态 | 含义 |
|------|------|
| `initialized` | 已创建 task 目录，PRD 原文已落盘 |
| `refined` | `prd-refined.md` 已生成 |
| `assessed` | `assessment.md` / `plan.md` 已生成 |
| `approved` | 计划已人工确认，可以编码 |
| `coding` | 正在执行 codegen |
| `reviewed` | review 已完成 |
| `mr_ready` | MR 描述等材料已生成 |
| `completed` | 整个 task 完成 |

### 7.3 状态存储

建议在 task 目录下增加一个元信息文件：

```text
~/.config/coco-ext/tasks/{task-id}/task.json
```

示例：

```json
{
  "task_id": "20260331-add-countdown",
  "title": "在讲解卡上增加倒计时展示",
  "status": "approved",
  "created_at": "2026-03-31T10:00:00+08:00",
  "updated_at": "2026-03-31T10:15:00+08:00",
  "approved_by": "maifeng@bytedance.com"
}
```

这样 `coco-ext prd status` 才能稳定输出状态。

---

## 8. Coco CLI 如何调用 coco-ext

### 8.1 调用原则

Coco CLI 不直接“自己做”，而是优先调用 `coco-ext`。

原则如下：

1. 能用现成 CLI 的，不自己重写流程
2. 能拿结构化结果的，不只看自然语言输出
3. 先跑 `doctor/status` 再跑重流程
4. 有人工确认点时，不自动越过

### 8.2 典型意图映射

| 用户说法 | Coco CLI 内部动作 |
|----------|------------------|
| “帮我看看环境有没有问题” | `coco-ext doctor` |
| “帮我提交并推送” | `coco-ext submit` |
| “帮我 review 一下” | `coco-ext review` |
| “帮我从这个 PRD 开始做” | `coco-ext prd run --prd ...` |
| “看下这个 task 到哪一步了” | `coco-ext prd status --task ...` |
| “计划确认了，开始编码” | `coco-ext prd approve --task ...` + `coco-ext prd codegen --task ...` |
| “帮我生成 MR 描述” | `coco-ext prd mr --task ...` |

### 8.3 用户最终交互体验

同事在 Coco CLI 中的真实体验应是：

```text
用户：帮我把这个 PRD 走一下

Coco CLI：
1. 调 coco-ext prd refine
2. 调 coco-ext prd assess
3. 展示 plan 摘要
4. 问用户是否确认
5. 确认后调 coco-ext prd approve
6. 调 coco-ext prd codegen
7. 调 coco-ext review
8. 调 coco-ext prd mr
9. 返回 MR 标题、描述、风险摘要和产物路径
```

所以：

- **同事看到的是自然语言工作流**
- **底层真正执行的是标准 CLI**

---

## 9. 为什么这套方案 ROI 高

### 9.1 对团队成员

- 少背命令
- 减少手工拼流程
- 失败后可恢复
- review / MR / commit 产物更标准

### 9.2 对工具维护者

- 每个步骤都可单独调试
- 可做自动化验证
- 出问题更容易定位是 refine、assess 还是 codegen

### 9.3 对老板

这套方案能讲一个完整故事：

**我们不是做了几个 AI 命令，而是把需求交付链路标准化了。**

可以强调的点：

- PRD 到 MR 全链路自动化
- 人工确认点可控
- 产物沉淀完整
- 可诊断、可恢复、可运营
- 后续可直接接 metrics 做价值统计

---

## 10. 实施顺序建议

不建议一次把所有命令都做完，建议按以下顺序推进。

### Phase 1：先补状态机

先做：

- `coco-ext prd status`
- `coco-ext prd approve`
- `task.json`

目标：

- 先把流程控制能力补起来

### Phase 2：补交付闭环

再做：

- `coco-ext prd mr`

目标：

- 把 `PRD -> MR` 的最后一公里补齐

### Phase 3：补统一编排入口

最后做：

- `coco-ext prd run`

目标：

- 提供高阶入口
- 给 Coco CLI 统一调用点

原因：

- 如果先做 `run`，但底层状态机没立起来，会变成一个难恢复的大脚本
- 先把状态机和 MR 收齐，后面再做统一编排才稳

---

## 11. 结论

最终推荐路线是：

1. **继续把 `coco-ext` 建成稳定 CLI 能力层**
2. **让 Coco CLI 成为主要交互入口**
3. **围绕 `prd` 子命令体系建设 `PRD -> MR` 状态机**
4. **复用现有 `context` 作为 Phase 0 底座，而不是再做一套 `prd init`**
5. **对人只暴露少量入口，对 AI 暴露完整能力**

一句话总结：

**同事不需要学会一堆命令，但系统必须先有一套完整命令，Coco CLI 才能稳定地替他们完成事情。**
