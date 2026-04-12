# AGENTS.md

本文件面向在 `coco-ext` 仓库内协作的 AI agent。目标是先给出当前仓库的事实性上下文，再约束默认工作方式，避免继续沿用过时假设。

如本文与代码不一致，以代码为准，并在完成改动后同步更新本文。

## 仓库上下文

- 项目名称：`coco-ext`
- Go 模块：`github.com/DreamCats/coco-ext`
- Go 版本：`1.24.11`
- 维护人：Maifeng `<maifeng@bytedance.com>`
- 入口：[`main.go`](/Users/bytedance/go/src/coco-ext/main.go)
- CLI 框架：`github.com/spf13/cobra`
- 默认模型：`Doubao-Seed-2.0-Code`
- 交互语言：prompt、CLI 输出、知识文件和 commit message 默认使用中文

## 先读什么

处理任务前，优先阅读以下文件，而不是凭记忆推断：

- [`README.md`](/Users/bytedance/go/src/coco-ext/README.md)
- [`cmd/root.go`](/Users/bytedance/go/src/coco-ext/cmd/root.go)
- [`internal/config/defaults.go`](/Users/bytedance/go/src/coco-ext/internal/config/defaults.go)
- 与当前改动直接相关的命令文件和 `internal/` 业务实现

如果发现本文过时，修代码时顺手修正文档，不要把错误上下文继续传给下一个 agent。

## 常用命令

```bash
# 构建二进制（注入 version / commit / buildDate）
make build

# 全量测试
make test

# 交叉编译
make build-all

# 安装到 ~/.local/bin/
make install

# 如需直接在源码目录使用 go install .，
# 先构建前端资源再重新编译二进制
cd web && npm run build && cd .. && go install .

# 单包测试
go test ./internal/scanner/ -v

# 依赖整理
go mod tidy
```

和仓库功能强相关的调试命令：

```bash
# 环境诊断
coco-ext doctor
coco-ext doctor --fix

# 知识文件
coco-ext context init
coco-ext context update
coco-ext context status
coco-ext query "关键词"

# review / lint
coco-ext review
coco-ext review --async
coco-ext lint
coco-ext lint --async

# PRD 工作流
coco-ext prd run -i "需求描述"
coco-ext prd refine --prd "需求描述"
coco-ext prd plan
coco-ext prd code
coco-ext prd list

# 本地 Web UI
coco-ext ui serve
```

## 目录与架构

三层结构：CLI 命令层 `cmd/` → 业务逻辑 `internal/` → 外部依赖（`coco-acp-sdk daemon`、`git`、`golangci-lint`、`lark-cli`）。

关键目录：

- [`cmd/`](/Users/bytedance/go/src/coco-ext/cmd)：Cobra 命令入口
- [`internal/config/`](/Users/bytedance/go/src/coco-ext/internal/config)：默认目录、超时和模型配置
- [`internal/scanner/`](/Users/bytedance/go/src/coco-ext/internal/scanner)：仓库扫描，产出目录树、Go 包、导出类型、IDL
- [`internal/generator/`](/Users/bytedance/go/src/coco-ext/internal/generator)：连接 daemon，支持普通生成、agent 生成和 prompt 模板
- [`internal/knowledge/`](/Users/bytedance/go/src/coco-ext/internal/knowledge)：`.livecoding/context/` 读写
- [`internal/review/`](/Users/bytedance/go/src/coco-ext/internal/review)：facts → scope → release → impact → quality → summary 管线
- [`internal/prd/`](/Users/bytedance/go/src/coco-ext/internal/prd)：refine / plan / code / archive / status / list
- [`internal/ui/`](/Users/bytedance/go/src/coco-ext/internal/ui)：本地 Web UI 的 HTTP API、静态资源托管与 task 创建/删除/plan/code 动作
- [`internal/lint/`](/Users/bytedance/go/src/coco-ext/internal/lint)：`golangci-lint` 执行与结果落盘
- [`internal/git/`](/Users/bytedance/go/src/coco-ext/internal/git)：diff、branch、commit 等 git 封装
- [`internal/metrics/`](/Users/bytedance/go/src/coco-ext/internal/metrics)：本地事件采集
- [`cmd/embedded_skills/`](/Users/bytedance/go/src/coco-ext/cmd/embedded_skills)：随二进制分发的 skill 资源

当前重要命令文件：

- [`cmd/review.go`](/Users/bytedance/go/src/coco-ext/cmd/review.go)
- [`cmd/gcmsg.go`](/Users/bytedance/go/src/coco-ext/cmd/gcmsg.go)
- [`cmd/submit.go`](/Users/bytedance/go/src/coco-ext/cmd/submit.go)
- [`cmd/push.go`](/Users/bytedance/go/src/coco-ext/cmd/push.go)
- [`cmd/lint.go`](/Users/bytedance/go/src/coco-ext/cmd/lint.go)
- [`cmd/doctor.go`](/Users/bytedance/go/src/coco-ext/cmd/doctor.go)
- [`cmd/install.go`](/Users/bytedance/go/src/coco-ext/cmd/install.go)
- [`cmd/ui.go`](/Users/bytedance/go/src/coco-ext/cmd/ui.go)
- [`cmd/agents.go`](/Users/bytedance/go/src/coco-ext/cmd/agents.go)
- [`cmd/prd_refine.go`](/Users/bytedance/go/src/coco-ext/cmd/prd_refine.go)
- [`cmd/prd_plan.go`](/Users/bytedance/go/src/coco-ext/cmd/prd_plan.go)
- [`cmd/prd_code.go`](/Users/bytedance/go/src/coco-ext/cmd/prd_code.go)
- [`cmd/prd_list.go`](/Users/bytedance/go/src/coco-ext/cmd/prd_list.go)

## 核心流程

### context

- `context init/update` 会扫描仓库并生成 4 个知识文件：`glossary.md`、`architecture.md`、`patterns.md`、`gotchas.md`
- 输出目录固定为 `.livecoding/context/`
- `update` 只更新 diff 影响部分，无变化时返回 `NO_UPDATE`

### review

- `review` 默认审查最后一个 commit；`--full` 审查分支整体 diff
- 输出目录：`.livecoding/review/<branch>-<commit>/`
- 除 `report.md` 外，还会写入 `diff.patch`、`meta.json`、`facts.json`、`scope.json`、`release.json`、`impact.json`、`quality.json`、`summary.json`、`report.json`
- `--async` 会拉起后台子进程，日志写入 `.livecoding/logs/`

### lint

- `lint` 基于 `golangci-lint` 运行风格检查
- 输出目录：`.livecoding/lint/<branch>-<commit>/`
- 产物包括 `lint.md` 和 `lint.json`
- `push` 成功后会尝试异步触发 lint；如果本机没有 `golangci-lint`，则静默跳过

### gcmsg / submit / push

- `gcmsg` 基于 diff 生成中文 conventional commit message；AI 失败会回退到本地兜底
- `submit` 只处理 staged 变更，不会替你执行 `git add .`
- `push` 本质上包装 `git push`，成功后后台启动 `review --async`，并在可用时启动异步 lint
- `push` 对 force push 有额外确认，除非显式传 `--yes`

### PRD

- `prd run -i ...` 一键执行 `refine -> plan -> code`；支持通过重复 `--repo` 声明多仓 task，并可通过 `--all-repos` 顺序推进所有 repo 的 code
- `prd refine` 支持文本、本地文件、飞书文档链接；飞书拉取走 [`internal/prd/lark.go`](/Users/bytedance/go/src/coco-ext/internal/prd/lark.go)，依赖 `lark-cli`
- `prd plan` 默认使用只读 explorer agent 生成 `design.md` 和 `plan.md`
- `prd refine` 当前支持通过重复 `--repo` 声明多仓 task 的关联仓库，task 主目录统一落在 `~/.config/coco-ext/tasks/`
- `prd` task 当前统一存储在 `~/.config/coco-ext/tasks/` 下，不再写入仓库内 `.livecoding/tasks/`
- `prd code` 会先在主仓库同级目录的 `.coco-ext-worktree/` 下创建隔离 worktree，同步 task/context 产物后再启动 yolo agent；默认分支名是 `prd_<task_id>`，支持通过 `--repo <repo_id>` 更新多仓 task 中某个 repo 的 code 结果，也支持通过 `--all-repos` 顺序推进所有 repo；多仓 task 下默认必须显式指定 `--repo`
- `prd code` 编译失败后会按改动 package 自动重试，成功时自动 commit
- `prd run` 在多仓 task 下默认只自动推进当前 repo；若 `plan` 判定复杂度为“复杂”，则会停止在 plan 阶段，不自动进入 code
- `prd reset` / `prd archive` 需要先删除 code 阶段生成的 worktree，再删除对应分支；当前已支持 `--repo` 只操作某个 repo 的结果
- `prd list` 支持状态过滤，当前 task 状态已扩展为 `initialized/refined/planned/coding/partially_coded/coded/archived`

### doctor / install / agents / daemon

- `doctor` 当前检查项为 `repository/workspace/hooks/tooling/lint/skills/daemon/logs`
- `install` 会安装 `commit-msg`、`pre-commit` hook，同步 skills 到 `~/.trae/skills/`，并尝试写入 lint 配置
- `agents` 会维护本文件中 `<!-- coco-ext-agents:start/end -->` 包裹的 section
- `daemon` 提供 `start/status/stop`，默认配置目录为 `~/.config/coco-ext/`

### ui

- `ui serve` 会在当前仓库启动本地 HTTP 服务，默认托管内嵌静态资源
- 当前 API 入口：
  - `GET /api/tasks`
  - `GET /api/tasks/:task_id`
  - `GET /api/workspace`
- 当前写操作与 repo 选择能力：
  - `POST /api/tasks`：Web UI 创建 task，后台异步 refine
  - `POST /api/tasks/:task_id/plan`：Web UI 触发后台异步 plan，产出 `design.md`、`plan.md` 与 `plan.log`
  - `POST /api/tasks/:task_id/code`：Web UI 触发后台异步单 repo code，产出 `code.log`、`code-result.json`、diff 与 commit 信息
  - `DELETE /api/tasks/:task_id`：仅允许删除未进入 code 的 task
  - `GET /api/repos/recent`：recent repos
  - `POST /api/repos/validate`：手动路径校验并加入 repo
  - `GET /api/fs/roots` / `GET /api/fs/list?path=...`：远程开发机上的目录浏览
- Web UI 创建 task 时不会自动把当前仓库加入 repo scope，必须显式选择至少一个 repo
- Web UI 当前的 `code` 动作仅支持单 repo task；多 repo task 仍需通过 CLI 按 repo 逐个推进
- 正式安装态默认使用内嵌静态前端资源；开发态可通过 `--web-dir` 覆盖静态目录

## 关键约定

- `.livecoding/context/`：知识文件
- `.livecoding/review/`：review 产物
- `.livecoding/lint/`：lint 产物
- `~/.config/coco-ext/tasks/`：PRD task 主目录（当前统一使用全局目录，不再写入仓库内 `.livecoding/tasks/`）
- `.livecoding/metrics/events.jsonl`：submit/gcmsg/review 等事件
- `.livecoding/logs/`：后台 review / lint / gcmsg 日志
- `.livecoding/changelog/`：commit 优化历史
- `<repo-parent>/.coco-ext-worktree/`：`prd code` / `prd run` code 阶段使用的隔离 worktree 根目录；位于仓库外部，通常不需要改当前仓库 `.gitignore`

超时配置来自 [`internal/config/defaults.go`](/Users/bytedance/go/src/coco-ext/internal/config/defaults.go)：

- 默认 prompt 超时：30s
- `context`：5min
- `review`：3min
- `prd code` 总超时：10min
- `prd code` chunk 空闲超时：60s
- daemon 空闲退出：60min，可被 `COCO_EXT_DAEMON_IDLE_TIMEOUT` 覆盖

scanner 当前会跳过：

- `.git`
- `.livecoding`
- `vendor`
- `node_modules`
- `kitex_gen`
- `dist`
- `.idea`
- `.vscode`

## 本仓库的默认协作方式

- 优先做最小范围验证。默认不要直接跑 `go build ./...` 或 `go test ./...`，除非改动范围确实覆盖全仓库，或用户明确要求。
- 修改 Go 代码后，优先运行受影响包的 `go test`，或者对受影响命令执行定向 `go build`。
- 不要删除或重置用户的 `.livecoding/` 产物，除非用户明确要求清理。
- 修改 CLI 行为时，同时检查对应 README / AGENTS / help 文案是否需要同步。
- 这个仓库大量依赖文件落盘和后台子进程；改动 review、lint、install、daemon、prd 流程时，要额外关注日志路径、权限位和异步行为。
- `make build` / `make install` / `make build-all` 当前会自动先构建 `web` 前端静态资源，再把 UI embed 进 Go 二进制。
- 生成或修改 hook 时，保持 shell 脚本可直接执行，不要引入交互式依赖。

<!-- coco-ext-agents:start -->
## AI 行为约束

本节由 coco-ext agents 管理，适用于 AI 辅助编码场景下的协作规则。

### 1. 未经确认，不得覆盖用户已修改的内容

如果发现用户已经修改过某段代码、配置或文档，即使只是微小调整，也不得在后续迭代中直接覆盖。
正确做法：先说明拟修改的范围、原因和影响，再征求用户确认。

### 2. 信息不明确时，必须先确认

遇到以下情况时，不得自行假设，必须先向用户确认：
- 需求不明确
- 术语不理解
- 逻辑不清晰
- 上下文存在冲突

正确做法：先复述当前理解，再明确指出需要用户确认的具体问题。

### 3. 信息不足时，不得自行补全

如果完成任务所需信息不足，应明确说明：
- 你已经确认的内容
- 你目前不确定的部分
- 还需要用户补充哪些信息

禁止：假设用户意图、猜测业务逻辑、擅自补全缺失信息。

### 4. 涉及关键决策时，必须先获得确认

以下类型的决定必须先征求用户意见：
- 架构改动
- 技术选型
- 核心逻辑变更
- 大规模重构

正确做法：给出 2-3 个可选方案，说明利弊和影响，再由用户选择或确认。

### 5. 其他协作要求

- 修改前应先说明改动范围和预期影响
- 删除文件、重写核心模块、大规模删除等危险操作必须明确确认
- 用户明确拒绝后，不应反复重复同一建议
- 存在不确定性时，应明确说明不确定项及其影响

## 编码规范

- 必须保持现有行为和配置不变，除非用户明确要求修改
- 应优先使用清晰、明确的 `if/else`，避免嵌套三元表达式
- 不应为了简短而编写影响可读性的单行代码
- 函数应保持短小、聚焦，职责应尽量单一
- 不得重构架构层级代码，除非用户明确提出该需求
- 默认执行最小范围验证，只构建或验证本次改动涉及的包、模块或服务
- 禁止保留魔法值，应将魔法数字和魔法字符串提取为具名常量
- 在解引用指针、访问 map 值或使用接口值前先进行 nil 判断

## 注释规范

- 导出函数必须提供文档注释（Go：`// FuncName ...`）
- 复杂逻辑必须补充解释意图的行内注释
- 注释应优先说明“为什么”，避免重复代码本身已经表达清楚的“做了什么”
- 注释风格、语气和格式应与仓库现有代码保持一致
<!-- coco-ext-agents:end -->
