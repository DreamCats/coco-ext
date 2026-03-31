# 术语提取指南

## 目标

从代码中自动提取业务术语和代码标识符的映射关系，生成 glossary.md 初稿。

**核心难题：** AI 能识别 `PopCard` 是某种卡片组件，但不知道业务上叫"讲解卡"还是"弹窗卡"。所以术语提取的重点是：**提取所有候选 → 标记置信度 → 让人确认低置信的。**

## 提取来源（优先级从高到低）

### 1. 注释中的业务描述（最高置信）

```bash
# struct 上方的注释
grep -B2 "^type [A-Z].*struct" "$MODULE_PATH/"**/*.go 2>/dev/null

# package 注释
head -10 "$MODULE_PATH/"*.go 2>/dev/null | grep "//"

# 常量组注释
grep -B1 "=.*//\|= iota" "$MODULE_PATH/"**/*.go 2>/dev/null
```

如果注释明确写了"讲解卡"、"福袋"等中文业务术语 → ✅ 高置信。

### 2. Package 名 + 目录名

```bash
# package 声明
grep "^package " "$MODULE_PATH/"*.go | head -1

# 目录名
basename "$MODULE_PATH"
```

- `live/popcard/` → package `popcard` → 某种卡片组件 → 尝试从代码上下文推断具体含义
- `live/auction/` → package `auction` → 拍卖 → 比较容易推断

### 3. 导出 Struct 名

```bash
grep "^type [A-Z].*struct" "$MODULE_PATH/"**/*.go 2>/dev/null
```

提取方法：
- `PopCard` → 拆分为 Pop + Card → "弹出的卡片"？
- `AuctionBuyer` → Auction + Buyer → "拍卖买家"
- `LiveRank` → Live + Rank → "直播排行榜"？

### 4. 导出常量 / 枚举

```bash
grep -A20 "^const (" "$MODULE_PATH/"**/*.go 2>/dev/null
grep "= iota\|= [0-9]" "$MODULE_PATH/"**/*.go 2>/dev/null
```

枚举值往往包含业务状态：
```go
const (
    AuctionStatusPending  = 0  // → 待开始
    AuctionStatusOngoing  = 1  // → 进行中
    AuctionStatusFinished = 2  // → 已结束
)
```

### 5. RPC 接口名 / Handler 函数名

```bash
grep "^func.*Handle\|^func.*Process\|^func.*Get\|^func.*Create" "$MODULE_PATH/"**/*.go 2>/dev/null
```

函数名暗含业务动作：
- `GetAuctionDetail` → 获取拍卖详情
- `CreateLuckyBag` → 创建福袋
- `HandlePopCardShow` → 处理讲解卡展示

## 置信度标记规则

| 条件 | 置信度 | 标记 |
|------|--------|------|
| 注释中明确写了中文业务含义 | 高 | ✅ |
| 英文名直译就是业务含义（如 Auction = 拍卖） | 高 | ✅ |
| 从代码上下文可推断，但不确定是否是团队实际叫法 | 中 | ❓ |
| 完全看不出业务含义（如 `XyzHandler`） | 低 | ❓ |

## 输出格式

每个模块产出一段 glossary 素材：

```markdown
### 模块：live/popcard

| 代码标识符 | AI 推测含义 | 别名 | 所在模块 | 置信度 | 依据 |
|-----------|-----------|------|---------|--------|------|
| PopCard | 弹窗卡片 / 讲解卡 | pop_card | live/popcard/ | ❓ | 从 package 名推断 |
| PopCardType | 卡片类型 | — | live/popcard/model/ | ❓ | struct 名 |
| ShowPopCard | 展示卡片 | — | live/popcard/handler/ | ✅ | 函数注释"展示讲解卡" |
```

## 汇总去重

多个模块可能引用同一个业务概念（如 `Auction` 同时出现在 `live/auction/` 和 `live/product/`）。

汇总时：
1. 合并同一标识符的所有出现位置
2. 置信度取最高的
3. 别名合并（去重）
4. 按业务域分组（直播间 / 电商 / 用户 等）

## 注意事项

- **不要编造业务含义**：看不出来就标 ❓
- **保留原始英文**：即使推断了中文含义，也保留英文标识符
- **注意缩写**：有些团队有内部缩写（如 `lp` = `live_pack`），注意识别
- **IDL 定义优先**：如果 IDL 文件中有字段注释，优先采用
