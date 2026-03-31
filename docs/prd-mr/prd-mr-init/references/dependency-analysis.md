# 依赖分析指南

## 目标

识别模块的外部依赖（下游 RPC 服务、中间件、公共库），为 prd-assess 的代码调研提供速查表。

## 分析步骤

### 1. Import 扫描

```bash
# 提取所有外部 import（非标准库、非当前项目内部包）
grep -h "\"code.byted.org" "$MODULE_PATH/"**/*.go 2>/dev/null | \
  sed 's/.*"\(.*\)".*/\1/' | sort -u
```

分类：
- **RPC Client 包**：路径含 `client` / `kitex_gen` / `thrift_gen`
- **公共库**：路径含 `gopkg` / `middleware` / `common`
- **中间件 Client**：路径含 `redis` / `kafka` / `mysql` / `dal`
- **项目内部包**：同项目的其他 package

### 2. RPC 调用点识别

```bash
# 搜索 Client 方法调用
grep -n "[A-Z][a-zA-Z]*Client\.[A-Z]" "$MODULE_PATH/"**/*.go 2>/dev/null

# 搜索 kitex client 调用模式
grep -n "\.Call\|\.Send\|\.Recv" "$MODULE_PATH/"**/*.go 2>/dev/null

# 搜索 RPC request 构造
grep -n "new([A-Z].*Request)\|&[a-z].*\..*Request{" "$MODULE_PATH/"**/*.go 2>/dev/null
```

对每个 RPC 调用提取：
- **下游服务名**：从 Client 变量名或 import 路径推断
- **接口名**：Client.MethodName
- **调用位置**：文件:行号
- **调用方函数**：所在函数名
- **请求关键字段**：从 Request struct 构造中提取

### 3. 中间件使用识别

```bash
# Redis
grep -n "redis\.\|rdb\.\|cache\.\|Redis" "$MODULE_PATH/"**/*.go 2>/dev/null

# Kafka / 消息队列
grep -n "kafka\.\|mq\.\|producer\.\|consumer\.\|Publish\|Subscribe" "$MODULE_PATH/"**/*.go 2>/dev/null

# MySQL / 数据库
grep -n "dal\.\|db\.\|gorm\.\|sqlx\.\|mysql\." "$MODULE_PATH/"**/*.go 2>/dev/null

# 配置中心
grep -n "config\.\|conf\.\|apollo\.\|consul\." "$MODULE_PATH/"**/*.go 2>/dev/null
```

### 4. 依赖关系图

用文字描述调用关系（不画图）：

```
当前模块: live/auction
  → product-service (GetProduct, ListProducts)
  → order-service (CreateOrder)
  → Redis (auction 状态缓存)
  → Kafka (auction 状态变更事件)
```

### 5. MCP 增强（可选）

如果 `byte-lsp` 可用：

```
byte-lsp:search_symbols "Client" in $MODULE_PATH
byte-lsp:explain_symbol <client_symbol> → 获取完整签名
```

如果 `bcindex` 可用：

```
bcindex:search "service client import" scope:$MODULE_PATH
```

## 输出格式

每个模块产出一段 dependencies 素材：

```markdown
### 模块：live/auction

**RPC 依赖：**

| 下游服务 | 接口 | 调用方 | 文件 |
|----------|------|--------|------|
| product-service | GetProduct | GetAuctionDetail | service/auction_service.go:45 |
| product-service | ListProducts | ListAuctionProducts | service/auction_service.go:78 |
| order-service | CreateOrder | CreateAuctionOrder | service/order_service.go:23 |

**中间件：**

| 类型 | 用途 | 包路径 |
|------|------|--------|
| Redis | 拍卖状态缓存 | code.byted.org/xxx/redis-client |
| Kafka | 状态变更事件 | code.byted.org/xxx/mq-client |

**公共库：**

| 库 | 用途 |
|------|------|
| code.byted.org/gopkg/logs | 日志 |
| code.byted.org/gopkg/errno | 错误码 |
```

## 汇总

合并所有模块的依赖信息：
1. 同一下游服务的接口合并到一行
2. 记录调用频率最高的接口
3. 标注"只在一个模块使用"和"多个模块共用"的依赖
