# Context 文件模板

生成 `.livecoding/context/` 下文件时，使用以下模板。

---

## glossary.md 模板

```markdown
# 业务术语表

> 由 context init / update 自动生成 + 人工标注
> 维护规则：每次做需求发现新术语时追加，保持按业务域分组
> 标记说明：✅ = 已确认 | ❓ = AI 推测，需人工确认
> 最后更新：YYYY-MM-DD

## 直播间 <!-- auto-generated -->

| 业务术语 | 代码标识符 | 别名 | 所在模块 | 状态 |
|----------|-----------|------|---------|------|
| ??? | PopCard | pop_card | live/popcard/ | ❓ | <!-- auto-generated -->
| ??? | LiveBanner | live_banner | live/banner/ | ❓ | <!-- auto-generated -->

## 电商 <!-- auto-generated -->

| 业务术语 | 代码标识符 | 别名 | 所在模块 | 状态 |
|----------|-----------|------|---------|------|
| ??? | Auction | auction, bid | live/auction/ | ❓ | <!-- auto-generated -->
| ??? | FlashDeal | flash_deal | live/promotion/ | ❓ | <!-- auto-generated -->
```

**人工确认后变成：**

```markdown
| 讲解卡 | PopCard | pop_card, explanation_card | live/popcard/ | ✅ | <!-- manual -->
```

---

## patterns.md 模板

```markdown
# 代码模式

> 由 context init / update 从代码库自动提取，人工确认后供 prd-assess/prd-codegen 参考
> 最后更新：YYYY-MM-DD

## 模式 1：Handler → Service → Converter <!-- auto-generated -->

**适用场景：** 标准 API 接口
**出现模块：** live/auction, live/popcard, live/product

**目录结构：**
\`\`\`
module/
├── handler/        # 入口，参数校验，调 service
├── service/        # 业务逻辑，调下游 RPC
├── converter/      # 数据转换（RPC response → API response）
└── model/          # 数据结构定义
\`\`\`

**典型代码骨架：**

\`\`\`go
// handler 层
func GetXxx(ctx context.Context, req *api.GetXxxRequest) (*api.GetXxxResponse, error) {
    // 1. 参数校验
    // 2. 调 service
    result, err := service.GetXxx(ctx, req.XxxId)
    if err != nil {
        return nil, err
    }
    // 3. 转换 + 返回
    return converter.ConvertXxxResponse(result), nil
}
\`\`\`

**新增字段时的改动模式：**
1. model/ 加字段定义（如果需要）
2. converter/ 加字段映射（最常见的改动点）
3. handler/ 通常不用改（除非加新参数）

## 模式 2：Event Consumer <!-- auto-generated -->

（类似格式...）

## 模式 3：Cron Job <!-- auto-generated -->

（类似格式...）
```

---

## conventions.md 模板

```markdown
# 团队编码约定

> 由 context init / update 从代码库自动提取，人工确认
> AI 编码时必须遵守以下约定，保持代码风格一致
> 最后更新：YYYY-MM-DD

## 命名规范 <!-- auto-generated -->

- **文件名：** snake_case（如 `product_detail.go`）
- **函数名：** CamelCase（如 `GetProductDetail`）
- **变量名：** camelCase（如 `productId`）
- **常量：** CamelCase 或 ALL_CAPS（视上下文）
- **package 名：** 全小写，单词（如 `converter`）

## 错误处理 <!-- auto-generated -->

\`\`\`go
// 主要模式：日志 + 返回 error
result, err := service.DoSomething(ctx, req)
if err != nil {
    logs.CtxError(ctx, "DoSomething failed, req=%v, err=%v", req, err)
    return nil, err
}

// 业务错误码模式
if !valid {
    return nil, errno.New(errno.ParamError, "invalid product_id")
}
\`\`\`

## 日志规范 <!-- auto-generated -->

\`\`\`go
// 统一用 logs.Ctx* 系列，必须带 ctx
logs.CtxInfo(ctx, "message, key=%v", value)
logs.CtxWarn(ctx, "message, key=%v", value)
logs.CtxError(ctx, "message, key=%v, err=%v", value, err)

// 禁止：
// fmt.Println(...)
// log.Printf(...)
// logs.Info(...)          ← 缺 ctx
\`\`\`

## Import 分组 <!-- auto-generated -->

\`\`\`go
import (
    // 标准库
    "context"
    "fmt"

    // 公司公共库
    "code.byted.org/xxx/common"

    // 项目内部包
    "code.byted.org/xxx/myproject/model"
)
\`\`\`

## 注释规范 <!-- auto-generated -->

- 导出函数必须有注释
- 注释语言：{中文/英文/混用}
- TODO 格式：`// TODO(username): 描述`
```

---

## dependencies.md 模板

```markdown
# 下游服务与接口速查

> 由 context init / update 从代码库关系中自动提取
> 最后更新：YYYY-MM-DD

## 服务依赖总览 <!-- auto-generated -->

| 下游服务 | 用途 | Client 包路径 | 常用接口 |
|----------|------|--------------|---------|
| product-service | 商品信息 | code.byted.org/xxx/product-client | GetProduct, ListProducts |
| order-service | 订单管理 | code.byted.org/xxx/order-client | CreateOrder, GetOrder |

## 接口详情 <!-- auto-generated -->

### product-service

| 接口 | 用途 | 调用方模块 | 入参关键字段 | 出参关键字段 |
|------|------|-----------|-------------|-------------|
| GetProduct | 获取商品详情 | live/product, live/auction | product_id | ProductInfo |
| ListProducts | 批量获取商品 | live/shelf | product_ids[] | []ProductInfo |

## 中间件 <!-- auto-generated -->

| 类型 | 用途 | 使用模块 | 包路径 |
|------|------|---------|--------|
| Redis | 缓存 | live/auction, live/product | code.byted.org/xxx/redis-client |
| Kafka | 消息队列 | live/auction | code.byted.org/xxx/mq-client |
| MySQL | 持久化 | live/order | code.byted.org/xxx/dal |
```

---

## 通用规则

1. **`<!-- auto-generated -->` 和 `<!-- manual -->` 标记**是增量刷新的关键，必须加
2. 每个表格 section 开头加标记，表示整个 section 是自动还是手动
3. 单行也可以加标记，实现行级的自动/手动区分
4. `YYYY-MM-DD` 在实际生成时替换为当天日期
5. `???` 在 glossary 中表示"AI 不确定业务术语"，等人填
