# 编译失败处理

## 处理流程

```
编译失败
  │
  ├─ 1. 读取错误信息
  │    └─ go build 输出的具体错误
  │
  ├─ 2. 分类错误
  │    ├─ 语法错误 → 修复语法
  │    ├─ Import 错误 → 修复 import
  │    ├─ 类型错误 → 修复类型
  │    ├─ 未定义符号 → 检查拼写/包名
  │    └─ 其他 → 分析原因
  │
  ├─ 3. 修复 + 重新编译
  │    └─ 最多 3 次
  │
  └─ 4. 仍失败 → 暂停，等待人工
```

## 常见错误及修复

### 语法错误

```
./file.go:10:5: expected '}', found 'EOF'
```

**修复**：检查括号匹配、遗漏的逗号/分号。

### Import 错误

```
./file.go:5:2: imported and not used: "fmt"
./file.go:10:5: undefined: errors
```

**修复**：
- "imported and not used" → 删除未使用的 import
- "undefined" → 添加缺失的 import

### 类型错误

```
./file.go:15:10: cannot use x (variable of type string) as int value
```

**修复**：
- 检查 plan.md 中的类型描述
- 对照参考样例的类型用法
- 做必要的类型转换

### 未定义符号

```
./file.go:20:5: undefined: PopCardService
```

**修复**：
- 检查包名是否正确（`service.PopCardService` vs `PopCardService`）
- 检查 import 是否包含了定义该符号的包
- 检查拼写（大小写敏感）

### 循环 import

```
import cycle not allowed: package A imports package B imports package A
```

**修复**：这通常意味着代码组织有问题。**不要自动修复**，暂停并告知用户。

## 修复规则

1. **每次只修复一个错误** — 编译可能报多个错，从第一个开始
2. **修复后立即重新编译** — 确认修复有效
3. **不要引入新问题** — 修复时不要改动与错误无关的代码
4. **记录修复过程** — 每次修复写入 changelog（修了什么、为什么）

## 何时暂停

以下情况立即暂停，等待人工：

- 3 次修复仍然编译失败
- 错误涉及 plan.md 之外的文件
- 循环 import
- 错误原因无法理解（unknown error）
- 修复需要改变 plan.md 中的设计（如需要改 struct 定义）

暂停时的输出：

```markdown
⚠️ 编译失败，需要人工帮助

**错误信息：**
\`\`\`
{go build 输出}
\`\`\`

**已尝试修复：**
1. {第一次修了什么} → 仍然报错
2. {第二次修了什么} → 仍然报错
3. {第三次修了什么} → 仍然报错

**可能原因：**
{AI 的分析}

**需要你：**
1. 检查 {文件} 中的 {具体位置}
2. 或更新 plan.md 中 {文件} 的改动描述
```
