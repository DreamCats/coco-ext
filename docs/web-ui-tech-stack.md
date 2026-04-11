# coco-ext Web UI 技术栈与视觉风格建议

> 日期：2026-04-11
> 状态：Draft
> 目标：为 `coco-ext` Web UI 一期确定推荐技术栈、前后端分工和页面风格方向，避免一开始陷入“先写个页面看看”的临时实现。

---

## 1. 结论

如果今天开始做 `coco-ext` Web UI，一期推荐方案是：

- **后端**：沿用 `coco-ext` 现有 Go CLI，增加本地 HTTP 服务能力
- **前端**：`React + Vite + TanStack Router + Tailwind CSS + shadcn/ui`

不推荐一期直接上：

- 重型全栈框架驱动的产品方案
- 远程服务优先方案
- “浏览器里重写一遍 workflow” 的双实现方案

一句话总结：

**一期要的是“本地工具型产品 UI”，不是“云端全栈平台”。**

---

## 2. 为什么后端继续用 Go

`coco-ext` 当前的核心能力都在 Go 中：

- 读取 `.livecoding/tasks/`、`.livecoding/context/`
- 读取 `code-result.json`
- 调用 `prd plan / code / reset / archive`
- 管理 worktree、分支、最小编译
- 复用已有 `internal/` 逻辑和数据结构

因此，一期最合理的方式不是再起一个独立后端，而是：

- 在 `coco-ext` 中增加一个本地 HTTP 服务入口
- 将 UI 的动作型请求继续转成现有 CLI / internal 调用

这样做的优势：

1. **复用已有逻辑**
   不需要重复实现 task 状态机、code-result 解析和 worktree 管理。

2. **部署简单**
   仍然是一个本地二进制，不引入额外服务端部署成本。

3. **架构一致**
   CLI 仍然是执行层，Web UI 只是可视化和轻操作层。

---

## 3. 为什么前端不能再只是附属品

一旦 Web UI 的目标不仅是“本地看文件”，而是：

- 支撑 PRD workflow 可视化
- 支撑 demo 讲故事
- 支撑正式产品感

前端就不能只是顺手拼模板。

原因很现实：

- 用户对产品成熟度的第一判断通常来自 UI
- “看起来像内部脚本页” 和 “看起来像正式产品” 的感知差距非常大
- 你们现在的价值点已经足够强，需要一个能把价值放大的前端表达层

---

## 4. 前端技术栈推荐

### 4.1 React

推荐原因：

- 生态稳定
- 组件抽象成熟
- 最适合构建任务流、表格、详情页、状态页这类复杂界面
- 与现有社区设计系统和工具链结合最好

React 不是风险点，真正的选择在于“选什么宿主框架”。

### 4.2 Vite

一期更推荐 `Vite` 作为前端构建与开发环境，而不是直接上 `Next.js`。

理由：

1. **本地工具型 UI 非常适合 Vite**
   当前场景是本地运行、读本地数据、调本地服务，不需要 SSR/SEO。

2. **开发体验好**
   对工具型界面来说，冷启动、热更新和迭代效率都很重要。

3. **足够现代**
   Vite 近年一直是主流前端开发工具链，且仍在快速演进。  
   参考：[Vite Blog](https://vite.dev/blog)

推荐结论：

- 一期：Vite
- 未来若 Web UI 发展成云端正式产品，再评估是否迁到更重的全栈框架

### 4.3 TanStack Router

推荐原因：

- 强类型路由对 `tasks/:task_id`、筛选、搜索参数非常友好
- 很适合工具类页面的信息架构
- 对 URL 状态表达能力强，便于分享和恢复页面状态

参考：[TanStack Router](https://tanstack.com/router)

适配场景：

- task 列表
- task 详情
- filters / search params
- artifact tabs

### 4.4 Tailwind CSS

推荐原因：

- 仍然是当前最通用的产品型 UI 基础设施之一
- 能快速沉淀主题、间距、层级和设计 token
- 非常适合在“正式产品感”和“开发效率”之间取得平衡

参考：

- [Tailwind CSS Docs](https://tailwindcss.com/docs)
- [Theme Variables](https://tailwindcss.com/docs/customizing-spacing/)

### 4.5 shadcn/ui

推荐原因：

- 适合做 data-heavy 的工具型界面
- 组件是“open code”，便于后续形成自己的设计系统
- 对表格、tabs、sheet、dialog、form、command 等能力支持成熟

参考：

- [shadcn/ui Introduction](https://ui.shadcn.com/docs)
- [Installation](https://ui.shadcn.com/docs/installation)

结论：

- 不要把 `shadcn/ui` 当“最终设计语言”
- 要把它当“快速搭出一套可控产品组件层”的起点

---

## 5. 为什么一期不优先选 Next.js

Next.js 仍然是今天最主流的全栈 React 框架之一，官方也持续强调 App Router 能力。  
参考：

- [Next.js Docs](https://nextjs.org/docs)
- [App Router](https://nextjs.org/docs/app)

但对 `coco-ext` 一期 Web UI，我仍不建议优先选它。

### 5.1 你们当前的问题不是 SSR

你们当前的主要问题是：

- 本地查看 workflow
- 本地读取结构化产物
- 本地触发 CLI 动作

这并不需要：

- 服务端渲染
- SEO
- 边缘部署
- 全栈同构架构

### 5.2 一期复杂度不值得

如果现在上 Next.js，会额外引入这些决策：

- App Router 还是 Pages Router
- server/client component 边界
- API Route / Route Handler 设计
- 打包集成方式

这些复杂度对一期价值并不高。

### 5.3 未来并不排斥迁移

不选 Next.js 只是一期优先级判断，不是否定它。

如果未来出现这些信号，再考虑迁移：

- Web UI 要远程部署
- 要支持团队共享
- 要做登录权限
- 要走云端产品化路线

---

## 6. 页面风格建议

### 6.1 整体气质

建议页面气质是：

**冷静、工程、可信、密度适中**

关键词：

- 工具型
- 专业
- 结果导向
- 信息层级清晰

不建议走的方向：

- 过度“AI 感”的紫色渐变产品页
- 过度 marketing 化 landing 风
- 聊天框占据产品中心

### 6.2 参考气质

更接近以下产品气质的结合：

- Linear：状态和任务层次清楚
- Vercel Dashboard：工程感强、信息密度高
- 内部 Devtool：功能明确、操作可靠

不是去模仿它们的样式细节，而是借鉴它们的表达方式：

- 状态优先
- 内容分层
- 少而稳的视觉强调

### 6.3 页面布局建议

#### Task 列表页

- 顶部筛选条
- 中间高密度列表
- 右侧可选 detail preview

不要把首页做成 KPI 大屏。

#### Task 详情页

- 顶部 summary
- 中间 timeline
- 下方 tabs 承载 artifacts
- 右侧固定显示 next action / branch / worktree / commit

#### Code 结果页

- 以“结果确认”为中心
- 明确展示 build / files / branch / worktree
- 可加 diff stat，但不要一开始就做复杂代码浏览器

### 6.4 组件风格建议

应优先强化这些组件：

- `Badge`：状态
- `Tabs`：产物切换
- `Table`：task 列表
- `Panel/Card`：结果摘要
- `Timeline/Steps`：阶段状态
- `Command/Action Bar`：下一步操作

少用这些：

- 花哨空状态插画
- 过量玻璃态
- 装饰性动效

---

## 7. 推荐实现方式

### 7.1 前后端职责划分

后端负责：

- 读取 task / context / review / lint 数据
- 聚合成面向 UI 的 API 结构
- 触发 `plan/code/reset/archive` 等动作

前端负责：

- 路由
- 页面布局
- 状态展示
- 交互动作
- 产物渲染

### 7.2 打包方式建议

建议采用：

- 前端独立目录，例如 `web/`
- `vite build` 输出静态资源
- Go 二进制通过 embed 内嵌静态资源

这样好处是：

- 分发仍然是一个 `coco-ext` 二进制
- 不需要额外前端部署
- 本地开发也可以前后端并行

### 7.3 本地开发方式建议

开发期建议：

- 前端 dev server 独立跑
- Go 本地 API server 独立跑
- 前端通过 proxy 转发到本地 Go 服务

这样最符合工程习惯，也利于前端快速试错。

---

## 8. 最终建议

如果今天开始做这件事，我建议直接采用：

```text
Backend: Go（内嵌本地 HTTP 服务）
Frontend: React + Vite + TanStack Router + Tailwind CSS + shadcn/ui
```

并坚持以下原则：

1. CLI 是执行层，不被替代
2. Web UI 是可视化和讲故事层
3. 一期先做本地工具型产品，不做远程平台
4. 页面风格追求正式产品感，而不是 demo 感

这套组合足够现代，也足够稳，最适合 `coco-ext` 当前阶段。
