# 代码调研指南

## 目标

基于术语翻译结果，精准定位 PRD 涉及的代码位置，找到相似样例，为编码计划提供依据。

## 调研流程

### 1. 从 knowledge 缓存入手

**先读 knowledge，再搜代码。** knowledge 能帮你快速定位模块和模式，减少盲搜。

```
读 patterns.md → 知道模块是 handler-service-converter 模式
读 dependencies.md → 知道调哪些下游 RPC
读 conventions.md → 知道代码风格约束
```

### 2. 定位入口点

从 PRD 涉及的页面/接口出发，找到代码入口：

```bash
# 搜索 handler 注册（路由）
grep -rn "popcard\|pop_card" --include="*.go" handler/ router/

# 搜索 RPC handler 注册
grep -rn "Register.*PopCard\|Register.*Popcard" --include="*.go" .
```

### 3. 追踪调用链

从入口 handler 出发，追踪 handler → service → RPC/DB → converter：

**用 MCP（优先）：**
```
byte-lsp:search_symbols "GetPopCardDetail"
byte-lsp:explain_symbol <symbol_uri>
byte-lsp:get_call_hierarchy <symbol_uri> direction=outgoing
```

**用 grep（降级）：**
```bash
# 从 handler 找 service 调用
grep -n "service\.\|Service\." handler/get_popcard.go

# 从 service 找 RPC 调用
grep -n "Client\.\|client\." service/popcard_service.go

# 从 service 找 converter 调用
grep -n "converter\.\|Convert" service/popcard_service.go
```

### 4. 查找相似样例

**样例查找是调研最重要的产出。**

搜索同模块内与当前需求改动类型相似的已有实现：

| 改动类型 | 样例搜索方向 |
|---------|-------------|
| 加字段（response） | 同 converter 中其他 Convert 函数 |
| 新增接口 | 同 handler 目录中其他 handler 文件 |
| 加业务逻辑 | 同 service 中类似的业务处理函数 |
| 加数据库字段 | 同 dal 中其他 CRUD 方法 |
| 加消费逻辑 | 同 consumer 目录中其他 consumer |

```bash
# 例：要在 converter 加字段，找同目录下其他 Convert 函数
grep -n "^func Convert" live/popcard/converter/*.go
```

选择最相似的 1-2 个作为参考样例，截取关键代码片段。

### 5. 确认数据来源

PRD 要展示某个数据 → 确认这个数据从哪来：

1. **已有字段？** — 检查 RPC response 中是否已有该字段
```bash
# 搜索 RPC response struct
grep -A20 "type.*Response.*struct" kitex_gen/*/popcard.go
```

2. **需要新调 RPC？** — 检查 dependencies.md 中是否有相关下游
3. **需要新建 RPC？** — 如果完全没有数据来源 → 标记为复杂依赖

### 6. 代码片段控制

每个关键文件截取 ≤ 20 行核心代码，重点截取：
- 函数签名 + 前 5 行逻辑
- 相关 struct 定义
- 样例中的关键映射逻辑

**不要把整个文件都贴出来。**

## 调研产出清单

每次调研必须输出：

- [ ] 涉及模块列表
- [ ] 关键文件表（文件路径 + 角色 + 说明）
- [ ] 调用链路图（文字描述）
- [ ] 参考样例（文件路径 + 函数名 + 为什么选这个）
- [ ] 核心代码片段（3-5 个，每个 ≤ 20 行）
- [ ] 数据来源确认（数据从哪个 RPC/DB 字段来）

## 常见陷阱

1. **不要只搜 handler** — handler 往往很薄，核心逻辑在 service/converter
2. **不要忽视 IDL** — 如果要加接口字段，IDL 可能需要先改
3. **注意生成代码** — `kitex_gen/` 是自动生成的，不要手动改
4. **注意 mock 目录** — `mock/` 下的代码不是真实实现
5. **跨模块调用** — 如果需要改多个模块，每个模块分别调研
