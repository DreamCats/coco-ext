# 代码结构分析指南

## 目标

识别模块的代码组织模式，提取典型代码骨架，为后续 prd-codegen 提供"怎么写代码"的参考。

## 分析步骤

### 1. 目录结构扫描

```bash
# 列出模块内所有子目录（2 层深度）
find "$MODULE_PATH" -type d -maxdepth 2 ! -path "*/.git*" | sort
```

根据子目录名判断分层模式：

| 子目录组合 | 模式名 | 说明 |
|-----------|--------|------|
| handler/ + service/ + converter/ | Handler-Service-Converter | 最常见，标准 API 接口 |
| handler/ + service/ + dal/ | Handler-Service-DAL | 直接操作数据层 |
| consumer/ + handler/ | Event Consumer | 消息队列消费 |
| cron/ + service/ | Cron Job | 定时任务 |
| handler/ only | Direct Handler | 轻量模块，handler 内直接处理 |
| 无明显分层 | Flat | 小模块，所有逻辑在一个目录 |

### 2. 函数签名提取

对每一层，取 2-3 个典型的导出函数签名：

```bash
# handler 层
grep -n "^func [A-Z]" "$MODULE_PATH/handler/"*.go 2>/dev/null

# service 层
grep -n "^func [A-Z]\|^func (.*) [A-Z]" "$MODULE_PATH/service/"*.go 2>/dev/null

# converter 层
grep -n "^func [A-Z]" "$MODULE_PATH/converter/"*.go 2>/dev/null
```

关注：
- 参数列表（ctx 是第一个参数吗？req 类型是什么？）
- 返回值（response + error 模式？还是只返回 error？）
- 方法 receiver（struct 方法？还是包级函数？）

### 3. Struct 定义提取

```bash
# 导出的 struct 定义
grep -B1 -A10 "^type [A-Z].*struct {" "$MODULE_PATH/"**/*.go 2>/dev/null
```

分类：
- **Request struct** — 命名含 Request/Req
- **Response struct** — 命名含 Response/Resp
- **Model struct** — 业务实体
- **Config struct** — 配置

### 4. 调用链骨架

选一个最有代表性的 API（通常是模块内最简单的 GET 接口），截取 handler → service → converter 的调用链：

```
handler.GetXxx(ctx, req)
  → service.GetXxx(ctx, id)
    → rpcClient.GetXxx(ctx, rpcReq)
    → converter.ConvertXxx(rpcResp)
  ← return resp, nil
```

**代码片段控制在每个函数 ≤ 20 行**，取核心逻辑，省略重复的错误处理。

### 5. 输出格式

每个模块产出一段 patterns 素材：

```markdown
### 模块：live/auction

**模式：** Handler-Service-Converter

**目录结构：**
- handler/ — 入口，参数校验，调 service
- service/ — 业务逻辑，调下游 RPC
- converter/ — 数据转换
- model/ — 数据结构

**典型调用链：**
handler.GetAuctionDetail → service.GetAuction → converter.ConvertAuction

**代码骨架：**
（附代码片段）
```
