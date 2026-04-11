# coco-ext Web UI 一期规划

> 日期：2026-04-11
> 状态：Draft
> 目标：规划 `coco-ext` 的本地 Web UI 一期范围，使其既能支撑真实使用，也能支撑对外讲故事与演示。

---

## 1. 为什么现在做 Web UI

`coco-ext` 的 CLI 和 PRD 工作流已经基本成型：

- `refine -> plan -> code -> reset/archive` 链路可跑通
- task 产物已经结构化落盘
- `prd code` 已支持隔离 worktree
- 失败后可恢复、可重试、可归档

此时 Web UI 的价值不再只是“给 CLI 套皮”，而是解决两个问题：

1. **可视化表达**
   把 `PRD -> 设计 -> 计划 -> 代码 -> 收尾` 这条链路从终端命令，变成可回看、可展示、可讲故事的产品流程。

2. **产物承载**
   目前大量信息已经存在于 `.livecoding/tasks/`、`.livecoding/context/`、`.livecoding/review/`、`.livecoding/lint/` 中，但纯文件系统和终端不适合持续浏览。

一句话总结：

**CLI 是执行层，Web UI 是可视化结果层。**

---

## 2. 一期目标

一期不做“大而全的控制台”，只做最小可行的本地 UI，优先承载 PRD 工作流。

### 2.1 必须达成

1. 能看到 task 列表和状态
2. 能进入单个 task 查看全流程产物
3. 能看见 code 阶段对应的 branch / worktree / commit
4. 能明确当前 task 下一步该执行什么
5. 能支持 demo 场景下从需求输入一路讲到代码结果

### 2.2 一期明确不做

- 不做远程多用户协作
- 不做云端持久化
- 不做复杂权限系统
- 不做完整在线编辑器
- 不做“在浏览器里直接替代 CLI 执行所有命令”

---

## 3. 产品定位

一期 Web UI 的定位应是：

**本地运行的 PRD Workflow Viewer + Operator**

关键特征：

- 运行在开发机本地
- 默认通过 `localhost` 访问
- 数据来源是本地仓库中的 `.livecoding/` 产物
- UI 主要负责展示、查看、轻量触发，不替代底层 CLI 能力

---

## 4. 一期页面规划

### 4.1 页面 1：Task 列表页

目标：作为 PRD 工作流首页，回答“现在有哪些 task、各自进度如何”。

建议展示：

- task 标题
- task_id
- 当前状态：`initialized/refined/planned/coded/archived`
- 来源类型：文本 / 文件 / 飞书
- 更新时间
- 是否已有 `design.md`
- 是否已有 `plan.md`
- 是否已有 `code-result.json`
- 最近一次 code 是否 build 通过

建议交互：

- 支持按状态过滤
- 支持按 task_id / 标题搜索
- 支持跳转到 task 详情页
- 支持显示推荐下一步动作

### 4.2 页面 2：Task 详情页

目标：成为 PRD task 的主视图，回答“这个 task 发生了什么、现在卡在哪、下一步是什么”。

建议布局：

- 顶部概览卡片
  - title
  - task_id
  - status
  - source type
  - branch
  - worktree
  - commit

- 阶段时间线
  - refine
  - plan
  - code
  - archive/reset 历史

- 推荐下一步
  - 例如 `coco-ext prd plan --task ...`
  - 例如 `coco-ext prd code --task ...`
  - 例如 `coco-ext prd archive --task ...`

### 4.3 页面 3：产物预览页

目标：承载 PRD 工作流中的核心文档产物，避免用户在文件系统里来回切。

建议以 tab 形式展示：

- `prd.source.md`
- `prd-refined.md`
- `design.md`
- `plan.md`
- `code-result.json`

必要能力：

- Markdown 渲染
- JSON 格式化显示
- 缺失产物时明确提示

### 4.4 页面 4：Code 结果页

目标：突出 code 阶段的工程结果，回答“代码改到了哪、编译是否通过、怎么继续”。

建议展示：

- build 状态
- branch
- worktree 路径
- commit hash
- files_written
- agent summary

如果后续愿意扩展，可加：

- worktree 中的 `git diff --stat`
- 文件列表点击跳转到本地 IDE

### 4.5 页面 5：工作区与诊断页

目标：帮助解释“为什么 UI 里看到这个结果”，并承接排障需求。

建议展示：

- 仓库根目录
- `.livecoding/tasks/` 路径
- `.coco-ext-worktree/` 根目录
- 最近的 worktree 列表
- 关联分支与 task_id
- 最近一次 code-result 的 worktree / branch 绑定关系

这是一期里“低频但高价值”的页面，特别适合演示 worktree 隔离能力。

---

## 5. 信息架构

建议采用以下一级导航：

```text
PRD Tasks
Task Detail
Artifacts
Code Result
Workspace
```

更具体的结构如下：

```text
首页 /tasks
├── task 列表
└── 过滤/搜索

详情 /tasks/:task_id
├── 概览
├── 时间线
├── 推荐下一步
├── 产物 tabs
│   ├── source
│   ├── refined
│   ├── design
│   ├── plan
│   └── code-result
└── code 结果卡片

工作区 /workspace
├── repo root
├── livecoding dirs
├── worktree roots
└── 最近 worktree 实例
```

设计原则：

- 一级信息以 **task** 为中心，而不是以文件为中心
- 先呈现“阶段状态”，再展开具体产物
- “下一步该干什么”必须一直可见

---

## 6. CLI 与本地 Web 服务如何衔接

### 6.1 运行模式

推荐新增一个本地命令，例如：

```bash
coco-ext ui
```

或：

```bash
coco-ext serve
```

默认行为：

- 启动本地 HTTP 服务
- 默认监听 `127.0.0.1:<port>`
- 自动打开浏览器或打印访问地址

不建议一期默认监听 `0.0.0.0`。

### 6.2 服务职责

本地 Web 服务只负责：

- 读取 `.livecoding/` 结构化产物
- 读取 `code-result.json` 中的 branch / worktree 信息
- 提供只读或轻量动作型 API

建议不要在一期里让 Web 层直接重写 workflow 逻辑。  
所有真实动作仍应复用 CLI：

- `prd plan`
- `prd code`
- `prd reset`
- `prd archive`

### 6.3 推荐集成方式

推荐架构：

```text
Browser
   ↓
Local HTTP Server
   ↓
Read .livecoding/ + invoke coco-ext subcommands
```

也就是说：

- 数据读取：直接读本地产物
- 动作触发：调用现有 CLI 子命令

这样可以最大限度复用既有逻辑，避免维护两套 workflow。

### 6.4 API 设计建议

一期只需少量 API：

- `GET /api/tasks`
- `GET /api/tasks/:task_id`
- `GET /api/tasks/:task_id/artifacts/:name`
- `GET /api/workspace`
- `POST /api/tasks/:task_id/plan`
- `POST /api/tasks/:task_id/code`
- `POST /api/tasks/:task_id/reset`
- `POST /api/tasks/:task_id/archive`

动作型 API 的内部实现仍然是执行对应 CLI 命令，而不是重复实现业务逻辑。

---

## 7. 最适合讲故事的 Demo 路径

一期 UI 必须服务于 demo，而不是先追求完美后台。

推荐演示路径：

### 路径 A：从需求到代码结果

1. 打开 Task 列表页
2. 新建或展示一个 task
3. 进入 task 详情页
4. 展示 `prd-refined.md`
5. 展示 `design.md` / `plan.md`
6. 展示 code-result：
   - branch
   - worktree
   - commit
   - files_written
   - build 状态

适合讲的故事：

**“给一个需求，系统会自动沉淀中间产物，并在隔离 worktree 中完成代码实现。”**

### 路径 B：展示可恢复能力

1. 打开一个失败或未完成的 task
2. 展示当前状态和缺失产物
3. 展示 UI 给出的下一步建议
4. 执行 reset 或继续 code

适合讲的故事：

**“这不是一次性脚本，而是一条可恢复、可重试、可归档的研发流程。”**

### 路径 C：展示工程可信度

1. 打开 workspace 页
2. 展示 `.coco-ext-worktree/` 目录结构
3. 展示 branch / worktree / commit 的绑定关系
4. 说明为什么不会污染主仓库

适合讲的故事：

**“AI 不是直接在主工作区乱改代码，而是在隔离工作区中受控执行。”**

---

## 8. 一期实现建议

### 8.1 后端实现建议

优先使用 Go 原生实现本地 Web 服务，原因：

- 已有 CLI 代码和数据结构都在 Go 中
- 直接复用 `internal/prd`、`internal/knowledge`、`internal/review` 等逻辑更自然
- 打包和分发简单

### 8.2 前端实现建议

一期可接受两种方案：

1. **Go Template + 少量 HTMX/Alpine 风格交互**
   优点：实现快、依赖少、打包简单

2. **独立前端（React/Vite）+ 内嵌静态资源**
   优点：交互更自然，后续扩展空间大

如果目标是尽快出 demo，一期建议优先 1。  
如果目标是很快做成正式产品，建议直接 2。

### 8.3 推荐优先级

建议实施顺序：

1. 只读 Task 列表页
2. Task 详情页 + 产物预览
3. Code 结果页 + Workspace 页
4. plan/code/reset/archive 的轻量触发按钮

---

## 9. 风险与边界

### 9.1 风险

- `.livecoding/` 结构未来可能继续演进，UI 需要容忍字段缺失
- 如果 CLI 行为变化快，UI 很容易过期，因此 UI 层必须尽量复用 CLI / internal 逻辑
- worktree 目录和主仓库可能不总是同时存在，UI 需要处理残留和清理失败场景

### 9.2 边界

- 一期优先支持 PRD 工作流，不扩到 review/lint/metrics 的完整可视化
- 一期先做本地单仓库 UI，不解决多仓切换和团队共享
- 一期先讲“Workflow + Artifacts + Isolation”的故事，不讲“统一 AI 开发平台”

---

## 10. 结论

Web UI 一期不应被定义为“给 CLI 套一个页面”，而应被定义为：

**把 `coco-ext` 的 PRD 工作流，从命令集合升级成可视化的 AI 研发工作台。**

最重要的不是功能数，而是让外部一眼看懂这件事：

- 有 task
- 有阶段
- 有产物
- 有隔离执行
- 有失败恢复
- 有归档收尾

只要这条故事线在 UI 中跑通，`coco-ext` 的产品感就会明显提升。
