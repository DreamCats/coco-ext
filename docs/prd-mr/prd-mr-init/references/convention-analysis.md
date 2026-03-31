# 约定分析指南

## 目标

从代码中提取团队实际遵循的编码约定，供 prd-codegen 生成代码时保持风格一致。

## 分析维度

### 1. 命名风格

**抽样方法：** 每个模块取 3 个文件，每个文件看 5-10 个变量/函数名。

```bash
# 局部变量命名
grep -oP '\b[a-z][a-zA-Z0-9]*\b\s*:=' "$MODULE_PATH/"**/*.go 2>/dev/null | head -10

# 函数参数命名
grep -oP 'func.*\((.*?)\)' "$MODULE_PATH/"**/*.go 2>/dev/null | head -10
```

判断：
- camelCase（`productId`）✅ Go 标准
- snake_case（`product_id`）⚠️ 非 Go 标准但有些团队用
- 混用 → 记录两种风格的比例

### 2. 错误处理模式

```bash
# 搜索 err 处理
grep -A3 "if err != nil" "$MODULE_PATH/"**/*.go 2>/dev/null | head -30
```

常见模式：
- **直接返回**：`return nil, err`
- **Wrap 后返回**：`return nil, errors.Wrap(err, "context")`
- **业务错误码**：`return nil, errno.New(errno.ParamError, "msg")`
- **日志 + 返回**：`logs.CtxError(ctx, ...); return nil, err`
- **吞掉 error**：`_ = doSomething()` ← 需要标记，可能是有意的

记录每种模式的出现频率，最高频的作为 convention。

### 3. 日志规范

```bash
# 搜索所有日志调用
grep -n "logs\.\|log\.\|klog\.\|hlog\.\|fmt\.Print" "$MODULE_PATH/"**/*.go 2>/dev/null | head -15
```

关注：
- 用哪个日志包？（`logs.Ctx*` / `klog` / `hlog` / `log`）
- 是否带 ctx？（`logs.CtxInfo(ctx, ...)` vs `log.Info(...)`）
- 日志格式？（Printf 风格 `"%v"` / 结构化 `log.With("key", val)`）
- 禁止什么？（`fmt.Println` / 无 ctx 的日志调用）

### 4. Import 分组

打开 2-3 个文件，看 import 块的分组规则：

```go
import (
    // 组 1：标准库
    "context"
    "fmt"

    // 组 2：公司公共库
    "code.byted.org/gopkg/logs"

    // 组 3：项目内部包
    "code.byted.org/xxx/myproject/model"
)
```

确认：
- 是否分组？分几组？
- 公司库和第三方库是否分开？
- 有没有用 goimports 自动排序？

### 5. 注释风格

```bash
# 导出函数是否有注释
grep -B1 "^func [A-Z]" "$MODULE_PATH/"**/*.go 2>/dev/null | grep "//" | head -10
```

确认：
- 导出函数是否都有注释（golint 要求）
- 注释语言：中文 / 英文 / 混用
- TODO 格式：`// TODO(username):` / `// TODO:` / `// FIXME:`

### 6. 测试风格（可选）

如果模块有 `_test.go` 文件：

```bash
grep "^func Test" "$MODULE_PATH/"**/*_test.go 2>/dev/null | head -5
```

确认：
- 测试函数命名：`TestXxx` / `TestXxx_yyy`
- 是否用 testify / gocheck / 标准 testing
- Mock 方式：mockgen / 手写 mock / interface 替换

## 输出格式

每个模块产出一段 conventions 素材：

```markdown
### 模块：live/auction

**命名：** camelCase（Go 标准）
**错误处理：** logs.CtxError + return err（90%），errno.New（10%）
**日志：** logs.CtxInfo/Warn/Error，必须带 ctx
**Import：** 三组（标准库 / 公司公共库 / 项目内部包）
**注释：** 导出函数有注释，中文为主
```

汇总时，找出所有模块的**共同约定**（出现在 80%+ 模块中的），作为 conventions.md 的内容。模块间有冲突的约定，记录两种写法，标注哪种更常见。
