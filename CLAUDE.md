# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目简介

coco-ext 是一个仓库级知识库生成工具，通过扫描代码仓库结构并借助 LLM（通过 coco-acp-sdk daemon）自动生成 4 个知识文件（glossary.md、architecture.md、patterns.md、gotchas.md），存放在 `.livecoding/context/` 目录下。

## 常用命令

```bash
# 构建（注入版本、commit、日期）
make build

# 交叉编译（darwin/linux × amd64/arm64）
make build-all

# 运行测试
make test            # 等价于 go test ./... -v

# 安装到 ~/.local/bin/
make install

# 依赖管理
go mod tidy
```

## 架构概览

三层结构：CLI → 业务逻辑 → 外部依赖

```
main.go                     # 入口，通过 ldflags 注入版本信息
cmd/                        # Cobra CLI 命令层
  root.go                   #   根命令
  init.go                   #   初始化知识库（核心流程）
  update.go                 #   基于 git diff 增量更新
  query.go                  #   关键词搜索（设计为 MCP tool）
  status.go                 #   知识库状态查看
  daemon.go                 #   daemon 管理（隐藏命令）
  version.go                #   版本信息
internal/
  config/defaults.go        # 硬编码配置（模型名、目录路径、知识文件定义）
  scanner/scanner.go        # 仓库扫描引擎（目录树 + Go AST 解析 + IDL 文件）
  generator/
    generator.go            # 封装 coco-acp-sdk daemon 连接，流式生成
    prompts.go              # 中文 prompt 模板（每个知识文件一个）
  knowledge/
    reader.go               # 知识文件读取与状态查询
    writer.go               # 知识文件写入（0600/0700 权限）
```

## 核心流程

**init 命令**：检查 git 仓库 → `scanner.Scan()` 扫描代码结构 → `generator.New()` 连接 daemon（自动启动）→ 按序生成 4 个知识文件（glossary → architecture → patterns → gotchas）→ 写入 `.livecoding/context/`

**update 命令**：获取 git diff → 加载已有知识文件 → 判断哪些文件受影响 → 仅更新变更部分，返回 "NO_UPDATE" 表示无需更新

**daemon 连接**：通过 `coco-acp-sdk` 的 `daemon.Dial()` 连接，配置目录 `~/.config/coco-ext/`，支持自动启动、流式 prompt、状态查询、关闭

## 关键约定

- Go 模块：`github.com/DreamCats/coco-ext`，Go 1.24.11
- CLI 框架：Cobra（`spf13/cobra`）
- 默认模型：`Doubao-Seed-2.0-Code`（字节跳动模型）
- 知识文件目录：`.livecoding/context/`（已 gitignore）
- scanner 跳过的目录：.git, .livecoding, vendor, node_modules, kitex_gen, dist, .idea, .vscode
- prompt 和用户界面均为中文
- 版本信息通过 Makefile ldflags 注入到 main 包变量
