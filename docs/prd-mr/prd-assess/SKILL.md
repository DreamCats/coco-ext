---
name: prd-assess
description: "代码调研与复杂度评估 - 读取完善后的 PRD，做术语翻译、代码调研、5 维复杂度评估、生成编码计划。当用户说 prd-assess / 评估需求 / 代码调研 / 复杂度评估 / 生成编码计划 / 调研一下这个需求 / 评估改动量 时触发。"
---

# /prd-assess - 代码调研与复杂度评估

**读取完善后的 PRD，做代码调研，评估复杂度，生成编码计划。**

Phase P（prd-refine）保证了 PRD 质量，本 skill 的目标是：把业务需求翻译成代码改动方案，让人确认后交给 Phase B 执行。

## 触发方式

```bash
# 自动读取最近的 task（最新的 prd-refined.md）
/livecoding:prd-assess

# 指定 task
/livecoding:prd-assess --task 20260310-add-countdown
```

## 前置条件

- `.livecoding/context/` 已初始化（至少 glossary.md 存在，建议先执行 `coco-ext context init`）
- `.livecoding/tasks/{task-id}/prd-refined.md` 存在（Phase P 产出）

如果前置条件不满足，提示用户先执行对应 skill。

## 产物

写入 `.livecoding/tasks/{task-id}/` 目录：

| 文件 | 内容 |
|------|------|
| `assessment.md` | 代码调研结果 + 5 维复杂度评估 |
| `plan.md` | 编码计划（改动文件清单 + 每个文件怎么改） |

## 核心流程

```
读取 prd-refined.md
      │
      ├─ Step 1: 术语翻译
      │    └─ 查 glossary.md，将业务术语映射为代码标识符
      │    └─ 标记 glossary 中缺失的术语
      │
      ├─ Step 2: 代码调研
      │    └─ 基于翻译后的标识符定位相关代码
      │    └─ grep + MCP 工具精准搜索
      │    └─ 输出：相关文件列表 + 调用链 + 核心代码片段
      │
      ├─ Step 3: 复杂度评估
      │    └─ 5 维技术视角打分
      │    └─ 判定简单/中等/复杂
      │
      ├─ Step 4: 生成编码计划（仅简单/中等）
      │    └─ 列出改动文件 + 每个文件的改法 + 参考样例
      │    └─ 列出不改动的文件（确认边界）
      │
      └─ Step 5: 输出 + 等待人工确认
           └─ 输出评估报告 + 编码计划
           └─ 等待人确认后才能进入 Phase B
```

## Step 1: 术语翻译

### 1.1 读取 glossary

```bash
cat .livecoding/context/glossary.md
```

### 1.2 逐个匹配

从 `prd-refined.md` 的"术语映射"部分读取 PRD 中的业务术语，逐个在 glossary 中查找。

**匹配策略：**
1. 精确匹配："讲解卡" → `PopCard`
2. 别名匹配："弹窗卡片" / "pop card" → `PopCard`（通过 glossary 的别名列）
3. 模糊匹配：如果精确/别名都没命中，尝试关键词匹配

### 1.3 缺失处理

如果某术语在 glossary 中找不到：

```markdown
⚠️ 术语缺失

以下 PRD 术语在 glossary.md 中未找到，代码调研可能不完整：
- "xxx" → 未找到对应代码标识符

建议：
1. 补充 glossary.md 后重新执行 prd-assess
2. 或告诉我这个术语对应的代码概念，我继续调研
```

**不要因为术语缺失就中断** — 提示后继续调研，用模糊搜索兜底。

### 1.4 输出术语映射表

```markdown
### 术语翻译结果

| PRD 术语 | 代码标识符 | 匹配方式 | 搜索关键词 |
|----------|-----------|---------|-----------|
| 讲解卡 | PopCard | ✅ 精确 | PopCard, pop_card |
| 倒计时 | — | ❓ 新概念 | countdown, timer |
| 拍卖 | Auction | ✅ 精确 | Auction, auction |
```

## Step 2: 代码调研

**参考**：[代码调研指南](references/code-research.md)

### 2.1 调研策略

基于术语翻译得到的搜索关键词，按以下优先级定位代码：

**第一层：context 缓存**
- 读 `.livecoding/context/patterns.md` — 确认模块的代码模式
- 读 `.livecoding/context/architecture.md` — 确认模块边界和相关目录
- 读 `.livecoding/context/gotchas.md` — 确认历史踩坑与隐式约定

**第二层：精准搜索**
- 用 `byte-lsp:search_symbols` 搜索核心 struct/function
- 用 `byte-lsp:explain_symbol` 获取符号详情
- 用 `byte-lsp:get_call_hierarchy` 获取调用链（限 L1-L2）

**第三层：文本搜索（MCP 不可用时降级）**
```bash
# 按标识符搜索
grep -rn "PopCard\|pop_card" --include="*.go" .

# 按文件名搜索
find . -name "*pop_card*" -o -name "*popcard*"

# 按 struct 定义搜索
grep -rn "type.*PopCard.*struct" --include="*.go" .
```

### 2.2 调研内容

对每个 PRD 功能点，定位：

1. **相关文件**：哪些文件需要改 / 哪些文件是上下文
2. **相关函数**：入口函数、核心处理函数、数据转换函数
3. **相关 struct**：request/response/model 定义
4. **调用链路**：handler → service → RPC → converter 的完整路径
5. **现有相似实现**：有没有类似功能可以参考（**这是样例驱动的关键**）

### 2.3 样例查找

**最重要的调研产出之一** — 找到最相似的已有实现作为编码参考。

```bash
# 搜索同模块内的类似功能
# 例：要加倒计时，找同模块内其他"展示类"功能是怎么加的
grep -rn "func Convert.*Response\|func Build.*Response" live/popcard/converter/ --include="*.go"
```

好的样例特征：
- 同模块、同模式（都是 handler-service-converter）
- 改动类型相似（都是"加字段"、都是"新增接口"）
- 代码量适中（不太大也不太小）

### 2.4 调研产出

```markdown
### 代码调研结果

#### 涉及模块
- `live/popcard/` — 主要改动模块
- `live/auction/` — 数据来源（调 RPC 获取拍卖信息）

#### 关键文件
| 文件 | 角色 | 说明 |
|------|------|------|
| live/popcard/handler/get_popcard.go | handler | 讲解卡详情接口入口 |
| live/popcard/service/popcard_service.go | service | 业务逻辑，调下游 RPC |
| live/popcard/converter/popcard_converter.go | converter | RPC → API 数据转换 |
| live/popcard/model/popcard.go | model | 数据结构定义 |

#### 调用链路
```
GetPopCardDetail (handler)
  → PopCardService.GetDetail (service)
    → AuctionClient.GetAuctionInfo (RPC)
    → ConvertPopCardResponse (converter)
  ← return response
```

#### 参考样例
**最相似的已有实现：** `ConvertPopCardPrice()` in `converter/popcard_converter.go:45`
- 也是在 converter 层加字段映射
- 从 RPC response 读取数据，映射到 API response
- 可以直接参考写法

#### 核心代码片段
（附 3-5 个关键代码片段，每个 ≤ 20 行）
```

## Step 3: 复杂度评估

**参考**：[评估模型](references/complexity-model.md)

### 5 维技术视角打分

| 维度 | 评估内容 |
|------|---------|
| **改动范围** | 改几个文件、跨几个 package |
| **接口变更** | IDL 是否要改、是否新增/修改接口 |
| **数据模型** | 表结构是否要改、是否新建表 |
| **业务逻辑** | 逻辑复杂度（CRUD / 条件分支 / 状态流转） |
| **依赖关系** | 是否依赖新服务、是否需要联调 |

每个维度 1-3 分（🟢 简单 / 🟡 中等 / 🔴 复杂），总分 5-15。

### 评估输出

```markdown
### 复杂度评估

| 维度 | 评分 | 说明 |
|------|------|------|
| 改动范围 | 🟢 1 | 2 个文件，单 package |
| 接口变更 | 🟢 1 | 不改 IDL，只加 response 字段 |
| 数据模型 | 🟢 1 | 不动表，读已有字段 |
| 业务逻辑 | 🟢 1 | 纯字段映射，无复杂逻辑 |
| 依赖关系 | 🟢 1 | 读已有下游接口，不需要联调 |
| **总分** | **5/15** | **简单** ✅ |
```

### 总分判定

| 总分 | 等级 | 决策 |
|------|------|------|
| **5-7** | ✅ 简单 | 生成完整编码计划，进入 Phase B |
| **8-10** | ⚠️ 中等 | 生成编码计划，标记需人工补充的部分 |
| **11-15** | ❌ 复杂 | 不生成编码计划，输出调研结果供人工参考 |

**复杂需求的处理：**

```markdown
❌ 该需求评估为「复杂」（{N}/15），不建议自动化编码。

建议：
1. 参考上述调研结果，人工编写代码
2. 或拆分需求为多个简单子任务后分别执行
3. 可用 `/livecoding:brainstorming` 讨论技术方案

调研结果已保存到 .livecoding/tasks/{task-id}/assessment.md
```

## Step 4: 生成编码计划

**参考**：[编码计划模板](references/plan-template.md)

仅在评估为「简单」或「中等」时生成。

### 4.1 计划内容

对每个需要改动的文件，说明：

1. **改什么** — 具体的改动描述
2. **怎么改** — 参考哪个样例、遵循哪个 pattern
3. **上下文** — 数据从哪来、需要读哪些现有代码
4. **风险点** — 中等评估时标记需要人关注的地方

### 4.2 边界确认

明确列出**不改动的文件**，帮助人确认改动范围没遗漏也没越界：

```markdown
### 不改动的文件（确认边界）
- IDL 不需要改（字段已存在于上游 response）
- 数据库不需要改（不涉及存储）
- 其他模块不需要改（改动封闭在 live/popcard 内）
```

### 4.3 编译验证命令

```markdown
### 编译验证
\`\`\`bash
go build ./live/popcard/...
go vet ./live/popcard/...
\`\`\`
```

### 4.4 plan.md 状态

```yaml
status: draft    # draft → approved → completed
```

- `draft`：AI 生成，待人工确认
- `approved`：人确认后标记，Phase B 才能执行
- `completed`：Phase B 执行完毕

## Step 5: 输出 + 等待确认

### 输出评估报告

将调研结果和评估写入 `assessment.md`，编码计划写入 `plan.md`。

### 输出摘要

```markdown
## 📊 需求评估完成

### 评估结果
- **复杂度：** 简单（5/15）✅
- **改动文件：** 2 个
- **参考样例：** ConvertPopCardPrice() — converter 层加字段
- **预估编码时间：** 10-15 分钟（AI 自动）

### 编码计划摘要
1. `live/popcard/converter/popcard_converter.go` — 加 countdown 字段映射
2. `live/popcard/model/popcard.go` — response struct 加 Countdown 字段

### 不改动
- IDL、数据库、其他模块

### 下一步
- ⚠️ 请 review `plan.md`，确认改动范围和改法
- 确认后执行 `/livecoding:prd-codegen` 开始编码
```

### 人工确认卡点

**plan.md 生成后，必须等人确认。** 不要自动执行 Phase B。

人确认的方式：
- 直接说"确认"/"approved"/"可以"/"开始编码"
- 或修改 plan.md 中的内容后说"按修改后的来"

确认后，将 plan.md 的 status 改为 `approved`。

## 注意事项

1. **代码调研必须基于实际代码** — 所有文件路径、函数名、struct 名必须来自实际搜索结果，不要编造
2. **样例是核心** — 找到好样例，编码计划的质量才有保障
3. **不要过度调研** — 每个功能点定位到关键的 3-5 个文件即可，不需要追踪整个调用树
4. **编码计划要具体** — "在 converter 加字段映射"不够，要说明"参考 ConvertXxx 的写法，从 rpcResp.AuctionConfig 读取"
5. **中等需求标记风险** — 对不确定的部分明确标记"需人工确认"，不要假装都搞清楚了
6. **复杂需求不要硬上** — 评估为复杂就输出调研结果，不要强行生成编码计划
7. **MCP 不可用时降级** — byte-lsp 不可用就用 grep，不要报错退出
8. **编译命令要精确** — 只编译改动的 package，不要写 `go build ./...`

## 与其他 skill 的关系

```
context init / update → context/（glossary + architecture + patterns + gotchas）
                    ↓ 按需读取
prd-refine → prd-refined.md
                    ↓ 作为输入
              prd-assess（本 skill）
                    ↓ 输出
              assessment.md + plan.md
                    ↓ 【人工确认】
              prd-codegen → 读 plan.md 执行编码
```

**prd-assess 是 PRD 和代码之间的桥梁 — 把业务语言翻译成代码改动方案。**
