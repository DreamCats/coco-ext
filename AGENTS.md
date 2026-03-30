# AGENTS.md

This file provides guidance to coding agents when working with this repository.

## Repository Overview

[在此添加仓库概述]



<< ------- coding guidelines start ------->>

# Coding Guidelines

- Preserve existing behavior and configuration
- Prefer explicit if/else over nested ternaries
- Avoid one-liners that reduce readability
- Keep functions small and focused
- Do not refactor architecture-level code
- **NEVER run global build commands** (e.g., `go build ./...`, `go build ./...`)
- **NEVER run global test commands** (e.g., `go test ./...`, `go test ./...`)
- **ALWAYS compile with minimal changes** - only build the specific package/service that was modified
- **NO magic values** - extract magic numbers/strings into local constants with descriptive names
- **ALWAYS check nil** - add nil checks before dereferencing pointers, accessing map values, or using interface values

## Comment Guidelines

- Exported functions MUST have doc comments (Go: `// FuncName ...`)
- Complex logic MUST have inline comments explaining intent
- Comments explain "why", not "what"
- Follow existing comment style in the codebase

<< ------- coding guidelines end ------->>


<< ------- behavior rules start ------->>

# AI 行为约束（AI Behavior Rules）

## 改动规则

- **修改代码前，先列出改动计划**（改哪些文件、改什么、为什么改），等用户确认后再动手
- **每次只改一个文件或一个函数**，改完让用户确认后再继续
- **不要动用户没提到的文件**，即使你认为"顺手改了更好"
- **不要重构架构级代码**，除非用户明确要求

## 沟通规则

- **不确定的地方先问，不要自己猜**。人会问你问题，你也应该问
- **遇到歧义时列出可能的理解**，让用户选择，不要自行决定
- **承认不确定**：如果不确定答案，说"我不确定"，不要编造

## 保护规则

- **不要覆盖用户的手动修改**。如果文件有未提交的改动，先确认那些改动的意图
- **生成代码后，如果用户手动调整了部分内容，后续修改必须保留那些调整**
- 如果必须修改用户手动改过的区域，先说明理由并等确认

## 输出规则

- **commit message 默认详细模式**：第一行 `type(scope): 简述`，空行后按文件分段描述改动
- **改动完成后主动输出摘要**：改了哪些文件、每个文件改了什么、有什么需要注意的
- **写完所有文件后提示用户**：`改动已完成，建议执行 /livecoding:auto-review 检查改动质量和影响面。`

<< ------- behavior rules end ------->>
