# 代码风格检查（golangci-lint）

## 需求
- 代码风格不一致（命名/错误处理/日志/metrics）

## 方案
- pre-commit/pre-push 钩子运行 `golangci-lint run --new-from-rev=HEAD~1`
- 只检查本次 commit 变更的文件
- 规则通过项目根目录 `.golangci.yml` 定制

## 待确认
- [ ] golangci-lint 规则具体内容（需和老板讨论）
- [ ] 检查时机：pre-commit 阻塞 还是 pre-push 阻塞？

## 参考
- https://golangci-lint.run/
