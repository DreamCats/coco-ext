---
name: prd-mr
description: "PRD → MR 全链路使用说明 - 当用户问 prd-mr 怎么用 / PRD 到 MR 流程 / 自动化编码流程 / 需求怎么提给 AI / livecoding 编码流程 时触发。展示完整使用指南。"
---

# /prd-mr - PRD → MR 全链路使用说明

**从一句话需求到可合入 MR 的端到端自动化流程。**

## 快速开始（30 秒看懂）

```
1️⃣  coco-ext context init / update                               ← 首次或大变更时初始化/刷新上下文
2️⃣  /livecoding:prd-refine --prd "在讲解卡上增加倒计时展示"       ← 每个需求：理解 + 完善 PRD
3️⃣  /livecoding:prd-assess                                       ← 代码调研 + 生成编码计划
4️⃣  确认 plan.md → /livecoding:prd-codegen                       ← 按计划编码 + 提 MR
```

## 流程全景

```
┌─────────────────────────────────────────────────────────────┐
│                                                             │
│   Phase 0 (一次性)          Phase P (每个需求)               │
│   ┌──────────────┐         ┌──────────────┐                │
│   │ context init │         │  prd-refine  │                │
│   │              │         │              │                │
│   │ 扫描代码库    │         │ PRD 5维打分   │                │
│   │ 生成上下文    │────────▶│ 探索式问答    │                │
│   │              │ 提供     │ 输出完善PRD   │                │
│   │ context/     │ 术语表   │              │                │
│   └──────────────┘         └──────┬───────┘                │
│                                   │                         │
│                                   ▼                         │
│   Phase A (每个需求)          Phase B (每个需求)             │
│   ┌──────────────┐         ┌──────────────┐                │
│   │  prd-assess  │         │ prd-codegen  │                │
│   │              │         │              │                │
│   │ 术语翻译      │         │ 逐文件编码    │                │
│   │ 代码调研      │──[确认]─▶│ 编译验证      │                │
│   │ 复杂度评估    │   ⬆️    │ commit+push  │                │
│   │ 编码计划      │  人工    │ 开 MR        │                │
│   └──────────────┘         └──────────────┘                │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## 详细步骤

### Phase 0：初始化上下文（首次 + 定期刷新）

> **频率**：首次使用跑一次，后续代码变动大时增量刷新

```bash
# 首次初始化
coco-ext context init

# 代码有较大变更时刷新
coco-ext context update
```

**产出**：`.livecoding/context/` 下若干上下文文件（当前至少包含 glossary / architecture / patterns / gotchas）

| 文件 | 内容 | 你需要做什么 |
|------|------|-------------|
| `glossary.md` | 业务术语 ↔ 代码标识符映射 | ⚠️ 花 10-15 分钟确认 ❓ 标记的术语 |
| `patterns.md` | 代码模式（handler/service/converter） | 扫一眼，纠正明显错误 |
| `conventions.md` | 团队约定（命名、错误处理、日志） | 扫一眼 |
| `dependencies.md` | 下游服务 + RPC 接口速查 | 可选 review |

**重点**：context 中的 glossary/术语类内容是后续调研与评估的基础；如果 context 过旧，先执行 `coco-ext context update`。

---

### Phase P：PRD 理解与完善（每个需求）

> **输入**：一句话需求 / 飞书 PRD 链接 / 本地文件

```bash
# 直接描述
/livecoding:prd-refine --prd "在讲解卡上增加倒计时展示"

# 飞书链接
/livecoding:prd-refine --prd https://bytedance.larkoffice.com/docx/xxx

# 本地文件
/livecoding:prd-refine --prd ./my-prd.md
```

**过程**：
1. AI 对 PRD 做 5 维质量打分（功能明确度/边界完整度/交互展示/验收标准/业务规则）
2. 针对低分维度提出 3-6 个具体问题
3. 你回答后 AI 合并更新，重新打分（最多 3 轮）
4. 你随时可以说"够了"/"继续"跳过问答

**产出**：`.livecoding/tasks/{task-id}/prd-refined.md`

**你需要做什么**：回答 AI 提出的问题（通常 5 分钟）

---

### Phase A：代码调研与评估（每个需求）

> **输入**：自动读取上一步的 prd-refined.md

```bash
/livecoding:prd-assess
```

**过程**：
1. 查 glossary 翻译业务术语为代码标识符
2. grep + MCP 搜索相关代码，追踪调用链
3. 找到最相似的已有实现作为编码参考
4. 5 维复杂度评估（改动范围/接口变更/数据模型/业务逻辑/依赖关系）
5. 简单或中等 → 生成编码计划；复杂 → 输出调研结果供参考

**产出**：
- `.livecoding/tasks/{task-id}/assessment.md` — 调研结果 + 评估
- `.livecoding/tasks/{task-id}/plan.md` — 编码计划（仅简单/中等）

**你需要做什么**：
- ⚠️ **Review plan.md**，确认改动文件和改法
- 确认后告诉 AI"确认"或"approved"

**复杂度判定**：

| 总分 | 等级 | 后续 |
|------|------|------|
| 5-7 | ✅ 简单 | AI 自动编码，大概率一次成功 |
| 8-10 | ⚠️ 中等 | AI 编码 + 标记风险点，需你关注 |
| 11-15 | ❌ 复杂 | 不自动编码，输出调研结果供你手动开发 |

---

### Phase B：编码 + 提 MR（每个需求）

> **前提**：plan.md 已被你确认（status: approved）

```bash
/livecoding:prd-codegen
```

**过程**：
1. 创建工作分支
2. 按 plan.md 逐文件编码（每个文件改完立即编译验证）
3. 全量编译 + go vet
4. commit + push
5. 输出 MR 创建指引

**产出**：
- 代码改动 + git commit
- `.livecoding/tasks/{task-id}/changelog.md` — 执行记录
- `.livecoding/tasks/{task-id}/mr.md` — MR 信息

**你需要做什么**：
- Review MR diff
- 在 code.byted.org 上开 MR（或 AI 提示的其他方式）
- 补充测试（如需要）

---

## 产物目录结构

```
.livecoding/
├── README.md
├── context/                          ← Phase 0 生成，跨任务复用
│   ├── glossary.md                   ← 核心：术语映射表
│   ├── architecture.md               ← 仓库结构与模块概览
│   ├── patterns.md                   ← 代码模式
│   └── gotchas.md                    ← 易错点与隐式约定
└── tasks/                            ← 每个需求一个子目录
    └── 20260310-add-countdown/
        ├── prd.md                    ← PRD 原文存档
        ├── prd-refined.md            ← Phase P 完善后的 PRD
        ├── assessment.md             ← Phase A 调研 + 评估
        ├── plan.md                   ← Phase A 编码计划
        ├── changelog.md              ← Phase B 执行记录
        └── mr.md                     ← Phase B MR 信息
```

## 完整示例

```bash
# ===== 首次使用（10-15 分钟） =====

> coco-ext context init
# → 生成 context/，必要时补充 glossary 中缺失的关键术语

# ===== 做一个需求（30-60 分钟） =====

# Step 1: PRD 完善
> /livecoding:prd-refine --prd "在讲解卡上增加拍卖倒计时展示"
# → AI 打分 14/25，提问 6 个问题
# → 你回答问题
# → AI 更新后打分 22/25，输出 prd-refined.md

# Step 2: 代码调研
> /livecoding:prd-assess
# → 术语翻译：讲解卡=PopCard，拍卖=Auction
# → 定位代码：converter/popcard_converter.go
# → 评估：5/15 简单 ✅
# → 生成 plan.md（2 个文件要改）

# Step 3: 你 review plan.md
> 确认，开始编码

# Step 4: 自动编码
> /livecoding:prd-codegen
# → ✅ [1/2] model/popcard.go — 加字段，编译通过
# → ✅ [2/2] converter/popcard_converter.go — 加映射，编译通过
# → commit + push
# → 输出 MR 创建指引
```

## FAQ

### Q: 必须按顺序执行 4 个 skill 吗？
是的。每个 skill 依赖上一个的产出。但 Phase 0（context init/update）通常只需首次跑一次，或在代码结构大变更后刷新。

### Q: PRD 只有一句话可以吗？
可以。prd-refine 会通过问答补齐缺失信息。PRD 越详细，问答轮次越少。

### Q: 评估为"复杂"怎么办？
Phase A 会输出调研结果（相关文件、调用链、样例），你可以参考这些信息手动编码，或把需求拆分成多个简单子任务。

### Q: 编码计划不对怎么改？
直接修改 `.livecoding/tasks/{task-id}/plan.md`，改完告诉 AI"按修改后的来"。

### Q: 编译失败了怎么办？
AI 会自动尝试修复（最多 3 次）。仍然失败会暂停并输出错误信息，等你指导。

### Q: glossary 术语不全导致调研不准怎么办？
prd-assess 会标记缺失术语。补充 glossary.md 后重新执行 prd-assess 即可。

### Q: 可以跳过 Phase P 直接调研吗？
不推荐。模糊的 PRD 会导致调研方向偏差，编码返工成本更高。

### Q: 增量刷新 knowledge 会丢失我的手动标注吗？
不会。`<!-- manual -->` 标记的内容在增量刷新时会被保留。
