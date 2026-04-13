# coco-ext 命名与定位评估

> 日期：2026-04-13
> 状态：Draft
> 目标：评估 `coco-flow`、`coco-devflow`、`RepoFlow` 三个候选名，并给出当前阶段的推荐结论。

---

## 1. 结论先行

当前阶段推荐名称：

**`coco-flow`**

推荐顺序：

1. `coco-flow`
2. `coco-devflow`
3. `RepoFlow`

核心原因：

- 项目已经不是简单的 `coco extension`，`ext` 语义明显过窄
- 项目仍然深度绑定 `coco` / `coco-acp-sdk` / daemon 体系，完全去掉 `coco` 前缀还偏早
- 项目真实形态更像“仓库级 AI 研发工作流”，`flow` 比 `ext` 更贴近产品本体
- 名字应体现通用研发基础设施属性，而不是被“国际化电商直播”团队背景绑定成业务专用工具

一句话概括：

**保留 `coco` 的生态连续性，去掉 `ext` 的局促感，用 `flow` 表达现在的工作流产品形态。**

---

## 2. 当前项目到底是什么

从仓库现状看，这个项目已经不再是“给 coco 补一个上下文扩展”的工具，而是一个仓库级 AI 研发工作流系统，覆盖：

- repository context 沉淀
- PRD refine / plan / code
- AI review
- submit / push 工作流
- hooks / skills / daemon 管理
- 本地 metrics
- Web UI task workspace

也就是说，当前产品本体更接近：

**Repo-native AI development workflow**

而不是：

- 单一知识库生成器
- 轻量插件
- 某个直播业务专用脚手架

这也是为什么继续叫 `coco-ext` 会逐渐显得不准确：

- `coco` 还成立，因为底层生态关系仍然很强
- `ext` 不再成立，因为产品已经长成独立工作流层

---

## 3. 命名评估标准

这次评估按 5 个标准判断：

1. **是否符合当前产品本体**
   名字要能反映“工作流系统”而不是“小扩展”。

2. **是否保留生态连续性**
   现阶段项目仍强依赖 `coco`，名字不宜突然完全切断认知。

3. **是否利于跨团队复用**
   虽然由国际化电商直播研发团队孵化，但项目能力本身是通用的。

4. **是否便于传播**
   名字要好记、好说、好解释。

5. **是否利于后续演进**
   如果后续继续长出 UI、automation、multi-repo orchestration，名字不能太局限。

---

## 4. 三个候选名对比

### 4.1 `coco-flow`

定位感受：

- 直接表达“这是 coco 生态下的工作流产品”
- 简短，易读，易记
- 能覆盖当前已经存在的 `context / prd / code / review / ui`

优点：

- 延续现有 `coco-ext` 认知，迁移成本最低
- `flow` 比 `ext` 更准确地描述当前主线能力
- 既可以解释为研发工作流，也可以容纳未来的 repo orchestration
- 名字长度合适，CLI、README、口头传播都比较顺

缺点：

- `flow` 语义比较宽，需要副标题补齐“AI dev workflow”的具体含义
- 如果未来彻底去 `coco` 化，这个名字仍会留下生态绑定

适配判断：

**最适合当前阶段。**

---

### 4.2 `coco-devflow`

定位感受：

- 比 `coco-flow` 更明确地指向“研发工作流”
- 产品含义更清晰，歧义更少

优点：

- 比 `coco-flow` 更强地表达工程研发属性
- 对新同学更容易理解，不需要额外解释这是干什么的
- 和当前仓库真实能力高度匹配

缺点：

- 名字略长
- 读起来没有 `coco-flow` 干净
- “devflow” 是准确但偏功能描述型的名字，品牌感稍弱

适配判断：

**如果优先追求清晰而不是简洁，它是一个很稳的备选。**

---

### 4.3 `RepoFlow`

定位感受：

- 更像一个独立产品，而不是 `coco` 的工作流层
- 平台感更强，对外泛化能力更好

优点：

- 不再绑定特定底层生态
- 很贴近“面向仓库的工作流系统”这个本体
- 对跨团队、跨 runtime、跨模型演进更友好

缺点：

- 和当前项目的 `coco` 血缘会断得太快
- 现阶段实际实现仍深度依赖 `coco daemon` / `coco-acp-sdk`
- 名字更通用，也更容易和外部已有产品或概念撞名
- 从 `coco-ext` 直接跳到 `RepoFlow`，对内部用户会显得跨度过大

适配判断：

**更像中长期独立化名称，不是当前阶段的最优解。**

---

## 5. 为什么不把团队属性写进名字

当前项目由国际化电商直播研发团队孵化，这是重要背景，但不应直接进入主名字。

原因：

- 团队归属不等于产品本体
- 当前能力明显是通用研发工作流能力，不是直播业务专用能力
- 如果把“直播”或“电商”写进主名字，会人为压缩跨团队复用空间
- 业务词进入名字后，后续对外推广和基础设施定位都会变窄

更合理的表达方式是：

- 主名字表达能力
- 副标题或 README 表达团队归属

例如：

```text
coco-flow
Repo-native AI development workflow
Incubated by Global E-commerce Live Engineering
```

---

## 6. 推荐方案

### 6.1 当前推荐

推荐主名：

**`coco-flow`**

推荐副标题：

**Repo-native AI development workflow**

推荐定位表述：

`coco-flow` 是一个面向代码仓库的 AI 研发工作流工具，提供 context、PRD task、plan/code、review、submit/push、metrics 和本地 Web UI 能力。

### 6.2 为什么不是 `coco-devflow`

`coco-devflow` 也合理，但它更像一个清晰的功能描述名，而不是一个更自然的产品名。

如果当前优先目标是：

- 更易传播
- 更像产品
- 与 `coco-ext` 平滑过渡

那么 `coco-flow` 更合适。

如果未来团队反馈是：

- “flow 太泛”
- “希望一眼看出这是研发流程工具”

那 `coco-devflow` 可以作为第二选择。

### 6.3 为什么现在不推荐 `RepoFlow`

`RepoFlow` 的问题不是不好，而是太早。

如果未来发生下面任一变化，可以重新考虑：

- 底层不再强依赖 `coco`
- 同一产品开始支持多个 agent/runtime 后端
- 需要对更广泛团队或更外部的场景做统一品牌

在那之前，直接切到 `RepoFlow` 会让产品命名领先于实际架构演进。

---

## 7. 命名迁移建议

建议分两阶段推进，不要一步到位硬切。

### Phase 1：先统一定位

先不改 module path、配置目录和兼容路径，优先统一对外叙事：

- README 首屏定位
- Cobra root help 文案
- AGENTS / CLAUDE 中的项目介绍
- docs 中的产品定义

目标是先把“`coco-ext` 已经不是 ext”的认知统一起来。

### Phase 2：确认正式改名后再做技术迁移

如果团队确认正式采用 `coco-flow`，再评估：

- 仓库名是否迁移
- Go module path 是否迁移
- 二进制名是否从 `coco-ext` 切到 `coco-flow`
- `~/.config/coco-ext` 是否保留兼容
- `.coco-ext-worktree` 是否保留兼容
- `COCO_EXT_*` 环境变量是否提供兼容读取

原则：

**先改产品叙事，再改技术标识；先提供兼容，再逐步收口。**

---

## 8. 最终建议

当前阶段最合理的决定是：

1. 将产品定位从“coco 扩展工具”升级为“仓库级 AI 研发工作流工具”
2. 将候选主名收敛为 `coco-flow`
3. 将 `coco-devflow` 作为次优备选
4. 将 `RepoFlow` 作为中长期独立化备选，不在当前阶段直接采用

最终推荐：

**`coco-flow`**

原因不是它最激进，而是它最符合当前项目的真实形态、演进节奏和迁移成本。
