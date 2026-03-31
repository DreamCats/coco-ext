---
name: prd-mr-init
description: "PRD-MR 链路的上下文初始化说明。当前实现建议直接复用 coco-ext context init / update / status / query，而不是再建设一套独立的 prd init 能力。"
---

# /prd-mr-init - 上下文初始化说明

**在做任何 PRD 评估之前，先让 AI 具备仓库上下文。当前推荐直接使用 `coco-ext context ...`，而不是另起一套 `prd init`。**

这是 PRD → MR 全链路的基石。没有 context，术语翻译、代码调研都无从谈起。

## 触发方式

```bash
# 首次初始化上下文
coco-ext context init

# 代码结构大变更后刷新
coco-ext context update

# 查看上下文状态
coco-ext context status
```

## 产物目录

```
.livecoding/
├── context/
│   ├── glossary.md          # 业务术语 ↔ 代码标识符映射（核心）
│   ├── architecture.md      # 仓库结构与模块概览
│   ├── patterns.md          # 代码模式（handler/service/converter 怎么写）
│   └── gotchas.md           # 已知坑点与隐式约定
└── README.md                # .livecoding 使用说明
```

## 核心流程

```
解析参数（--scope / --incremental）
      │
      ├─ Step 1: 扫描仓库结构
      │    └─ 识别模块列表（按 Go package 组织）
      │    └─ --scope 过滤 / --incremental 过滤有变更的模块
      │
      ├─ Step 2: 逐模块分析（串行，避免 context 溢出）
      │    ├─ 2a: 代码结构分析 → patterns 素材
      │    ├─ 2b: 命名与约定分析 → conventions 素材
      │    ├─ 2c: 术语提取 → glossary 素材
      │    └─ 2d: 依赖分析 → dependencies 素材
      │
      ├─ Step 3: 汇总生成 context 文件
      │    └─ 合并各模块分析结果，生成 context 文件
      │    └─ 增量模式下，保留 <!-- manual --> 标记的内容
      │
      └─ Step 4: 输出初始化报告
           └─ 统计数据 + 需要人工确认的 ❓ 术语列表
```

## Step 1: 扫描仓库结构

### 1.1 确定项目根目录

```bash
PROJECT_ROOT=$(git rev-parse --show-toplevel)
```

### 1.2 创建 .livecoding 目录结构

```bash
mkdir -p "$PROJECT_ROOT/.livecoding/context"
mkdir -p "$PROJECT_ROOT/.livecoding/tasks"
```

### 1.3 识别模块列表

**有 --scope 参数时：**

```bash
# 例：--scope live/auction,live/popcard,live/product
# 直接用指定的模块列表
MODULES="live/auction live/popcard live/product"
```

**无 --scope 参数时：**

```bash
# 扫描所有包含 .go 文件的目录（排除 vendor、生成代码、测试 fixtures）
find "$PROJECT_ROOT" -name "*.go" \
  ! -path "*/vendor/*" \
  ! -path "*/.git/*" \
  ! -path "*/output/*" \
  ! -path "*/kitex_gen/*" \
  ! -path "*/thrift_gen/*" \
  ! -path "*/mock/*" \
  ! -path "*/testdata/*" \
  -exec dirname {} \; | sort -u
```

**--incremental 模式：**

```bash
# 最近 50 个 commit 涉及的 Go 文件所在目录
git diff --name-only HEAD~50 -- '*.go' \
  | xargs -I{} dirname {} \
  | sort -u
```

### 1.4 过滤与分组

将目录按**顶级模块**分组（取前 2-3 层路径），避免重复分析同一模块的子目录。

例：`live/auction/handler/`、`live/auction/service/`、`live/auction/converter/` 都归入 `live/auction/`。

## Step 2: 逐模块分析

**串行处理每个模块**，避免上下文过大。每个模块分析完后输出简要进度。

### 2a: 代码结构分析 → patterns 素材

**参考**：[代码结构分析指南](references/structure-analysis.md)

对每个模块：

1. **扫描目录结构**
```bash
find "$MODULE_PATH" -type d -maxdepth 2 | sort
```

2. **识别分层模式**
   - 有 `handler/` + `service/` + `converter/` → Handler-Service-Converter 模式
   - 有 `consumer/` + `handler/` → Event Consumer 模式
   - 有 `cron/` → Cron Job 模式
   - 其他 → 记录实际目录结构

3. **提取典型函数签名**（每层取 1-2 个有代表性的）
```bash
# handler 层函数签名
grep -n "^func " "$MODULE_PATH/handler/"*.go 2>/dev/null | head -5

# service 层函数签名
grep -n "^func " "$MODULE_PATH/service/"*.go 2>/dev/null | head -5
```

4. **提取 struct 定义**（request/response/model）
```bash
grep -A5 "^type.*struct" "$MODULE_PATH/"**/*.go 2>/dev/null | head -30
```

5. **记录典型代码骨架**：选最有代表性的 handler → service → converter 调用链，截取关键代码片段（每个片段 ≤ 20 行）。

### 2b: 命名与约定分析 → conventions 素材

**参考**：[约定分析指南](references/convention-analysis.md)

对每个模块：

1. **变量命名风格** — 抽样 10 个变量名，判断 camelCase / snake_case / 混用
2. **错误处理模式** — 搜索 `if err != nil` 后的处理方式：
```bash
grep -A2 "if err != nil" "$MODULE_PATH/"**/*.go 2>/dev/null | head -20
```
3. **日志调用方式** — 搜索日志调用：
```bash
grep -n "logs\.\|log\.\|klog\.\|hlog\." "$MODULE_PATH/"**/*.go 2>/dev/null | head -10
```
4. **import 分组** — 查看 2-3 个文件的 import 块，确认分组规则
5. **注释风格** — 查看导出函数是否有注释，注释语言（中/英）

### 2c: 术语提取 → glossary 素材

**参考**：[术语提取指南](references/glossary-extraction.md)

对每个模块：

1. **从 package 名提取** — package 名往往是业务概念
```bash
grep "^package " "$MODULE_PATH/"*.go | head -1
```

2. **从 struct 名提取** — 导出的 struct 通常是核心业务对象
```bash
grep "^type [A-Z].*struct" "$MODULE_PATH/"**/*.go 2>/dev/null
```

3. **从注释提取** — 有些注释直接写了业务含义
```bash
grep -B1 "^type [A-Z]" "$MODULE_PATH/"**/*.go 2>/dev/null | grep "//"
```

4. **从 const/enum 提取** — 状态码、类型枚举暗含业务概念
```bash
grep -A10 "^const\|^var.*=" "$MODULE_PATH/"**/*.go 2>/dev/null | head -30
```

5. **标记置信度**：
   - ✅ 高置信 — 注释明确说明了业务含义
   - ❓ 需确认 — AI 从代码上下文推测的含义，需要人工标注

### 2d: 依赖分析 → dependencies 素材

**参考**：[依赖分析指南](references/dependency-analysis.md)

对每个模块：

1. **扫描 import** — 识别外部依赖
```bash
grep -h "\"code.byted.org" "$MODULE_PATH/"**/*.go 2>/dev/null | sort -u
```

2. **识别 RPC 调用** — 搜索 Client 调用模式
```bash
grep -n "Client\.\|client\." "$MODULE_PATH/"**/*.go 2>/dev/null | head -20
```

3. **识别中间件使用** — Redis / Kafka / MySQL / 配置中心
```bash
grep -n "redis\|kafka\|mysql\|dal\.\|cache\." "$MODULE_PATH/"**/*.go 2>/dev/null | head -10
```

4. **记录**：下游服务名 → 调用的接口 → 所在文件 → 调用方函数

### 2e: MCP 增强（可选）

如果 `byte-lsp` MCP 可用，对关键符号做更精准的分析：

- `byte-lsp:search_symbols` — 搜索核心业务 struct
- `byte-lsp:explain_symbol` — 获取符号签名、引用数
- `byte-lsp:get_call_hierarchy` — 关键函数的调用链（限 L1-L2）

如果 `bcindex` MCP 可用：

- `bcindex:search` — 语义搜索相关代码

**MCP 不可用时降级为纯 grep/find 分析，不阻塞流程。**

## Step 3: 汇总生成 knowledge 文件

将各模块的分析结果汇总，写入 4 个 knowledge 文件。

**格式参考**：[knowledge 文件模板](references/knowledge-templates.md)

### 3.1 写入规则

- 新生成的内容用 `<!-- auto-generated -->` 标记
- 人工补充的内容用 `<!-- manual -->` 标记
- **增量模式下**：只更新 `<!-- auto-generated -->` 部分，保留所有 `<!-- manual -->` 部分
- 如果 knowledge 文件已存在且含 `<!-- manual -->` 内容，合并而不是覆盖

### 3.2 生成顺序

1. **glossary.md** — 最先生成（后续文件可引用术语）
2. **patterns.md** — 代码模式
3. **conventions.md** — 编码约定
4. **dependencies.md** — 依赖关系

### 3.3 生成 .livecoding/README.md

如果 README.md 不存在，生成一份简要说明：

```markdown
# .livecoding

PRD → MR 自动化流程的知识库和任务产物目录。

## 目录结构

- `context/` — 跨任务复用的上下文知识库（glossary/architecture/patterns/gotchas）
- `tasks/` — 按需求组织的任务产物（每个 task 一个子目录）

## 使用方式

1. 初始化上下文：`coco-ext context init`
2. PRD 理解完善：`/livecoding:prd-refine --prd "<需求描述>"`
3. 代码调研评估：`/livecoding:prd-assess`
4. 编码生成 MR：`/livecoding:prd-codegen`

## 维护说明

- context/ 中 AI 生成的内容由 `coco-ext context init / update` 自动维护
- `<!-- manual -->` 标记的内容为人工补充，增量刷新时不会被覆盖
- glossary.md 中 ❓ 标记的术语需要人工确认业务含义
```

## Step 4: 输出初始化报告

分析完成后，输出结构化报告：

```markdown
## 📊 context 初始化报告

### 扫描范围
- **模式**：全量 / 增量 / scope 限定
- **模块数**：{N}
- **Go 文件数**：{N}
- **预估 LOC**：{N}

### context 生成结果

| 文件 | 状态 | 要点 |
|------|------|------|
| glossary.md | ✅ 已生成 | {N} 个术语，{M} 个高置信，{K} 个需确认 ❓ |
| architecture.md | ✅ 已生成 | 模块结构与分层概览 |
| patterns.md | ✅ 已生成 | 识别 {N} 种代码模式 |
| gotchas.md | ✅ 已生成 | 隐式约定与易错点摘要 |

### ⚠️ 需要你确认的内容

**glossary.md 中 {K} 个 ❓ 术语：**

| 代码标识符 | AI 猜测含义 | 所在模块 | 请补充业务术语 |
|-----------|-----------|---------|-------------|
| PopCard | 弹窗卡片？讲解卡？ | live/popcard/ | ← 请填 |
| FlashDeal | 秒杀？限时优惠？ | live/promotion/ | ← 请填 |
| ... | ... | ... | ... |

**patterns.md：** 是否漏了某种你们常用的代码模式？
**conventions.md：** 编码约定是否准确？

### 下一步
1. 花 10-15 分钟 review glossary.md，补充 ❓ 术语
2. 扫一眼 patterns.md / conventions.md，纠正明显错误
3. 确认后即可开始做需求：`/livecoding:prd-refine --prd "..."`
```

## 增量刷新逻辑

当使用 `--incremental` 参数时：

1. **检测变更范围**
```bash
# 最近 50 个 commit 涉及的 Go 文件
CHANGED_FILES=$(git diff --name-only HEAD~50 -- '*.go')
CHANGED_MODULES=$(echo "$CHANGED_FILES" | xargs -I{} dirname {} | sort -u)
```

2. **只重新分析有变更的模块** — Step 2 只处理 CHANGED_MODULES

3. **合并策略**
   - 读取现有 context 文件
   - 保留人工补充的内容
   - 更新属于变更模块的自动生成内容
   - 其他模块内容保持不变

4. **新增模块** — 如果 git diff 中出现了 context 中没有的模块，追加到末尾

## 注意事项

1. **串行分析**：每个模块独立分析，分析完一个再分析下一个，避免 context 溢出
2. **代码量控制**：每个模块读取的代码总量控制在 500 行以内（取样而非全量读取）
3. **不要猜测业务含义**：看不出来就标 ❓，让人来确认
4. **跳过生成代码**：`kitex_gen/`、`thrift_gen/`、`mock/` 等生成目录直接跳过
5. **跳过 vendor**：`vendor/` 目录不扫描
6. **大仓保护**：如果无 scope 参数且模块数 > 30，提示用户指定 scope
7. **幂等性**：多次执行同一 scope 的 init，结果一致（auto-generated 部分完全重写）
8. **git 感知**：如果不在 git 仓库中，提示用户并退出

## 与其他 skill 的关系

```
context init / update
    ↓ 生成 context/
prd-refine（读 glossary 辅助术语理解）
    ↓
prd-assess（读 context，代码调研 + 评估）
    ↓
prd-codegen（读 patterns + gotchas，保证生成代码风格一致）
```

**context/ 是全链路的知识基础，质量直接影响后续所有阶段。**
