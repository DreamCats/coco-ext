package prd

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

// reGoIdent 匹配 Go 标识符（至少 2 个字符，首字母大写的导出标识符优先，也包含小写）。
var reGoIdent = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]{1,}`)

// ExtractIdentifiersFromPlan 从 plan.md 的任务列表（Actions、目标、实施步骤）中提取 Go 标识符。
func ExtractIdentifiersFromPlan(planContent string) []string {
	// 收集所有可能包含标识符的文本段
	var sources []string

	lines := strings.Split(planContent, "\n")
	inActions := false
	inSteps := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 「具体动作」段
		if strings.HasPrefix(trimmed, "- 具体动作：") || strings.HasPrefix(trimmed, "- 具体动作:") {
			inActions = true
			inSteps = false
			continue
		}
		// 「目标」行
		if strings.HasPrefix(trimmed, "- 目标：") || strings.HasPrefix(trimmed, "- 目标:") {
			sources = append(sources, trimmed)
			continue
		}
		// 「实施步骤」段
		if trimmed == "## 实施步骤" {
			inSteps = true
			inActions = false
			continue
		}
		// 遇到新的 ## 段落终止
		if strings.HasPrefix(trimmed, "## ") {
			inActions = false
			inSteps = false
			continue
		}
		// 遇到新的 - 开头的非缩进行，结束 actions
		if inActions && strings.HasPrefix(trimmed, "- ") && !strings.HasPrefix(line, "  ") {
			inActions = false
		}

		if inActions && strings.HasPrefix(trimmed, "- ") {
			sources = append(sources, trimmed)
		}
		if inSteps && strings.HasPrefix(trimmed, "- ") {
			sources = append(sources, trimmed)
		}
	}

	combined := strings.Join(sources, "\n")
	tokens := reGoIdent.FindAllString(combined, -1)

	// 过滤掉太通用的词
	stopWords := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "that": true,
		"this": true, "from": true, "func": true, "type": true, "var": true,
		"const": true, "return": true, "if": true, "else": true, "range": true,
		"err": true, "nil": true, "string": true, "int": true, "bool": true,
		"error": true, "true": true, "false": true, "make": true, "append": true,
		"len": true, "fmt": true, "log": true, "context": true, "ctx": true,
		"package": true, "import": true, "struct": true, "interface": true,
		"map": true, "slice": true, "byte": true, "int64": true, "float64": true,
		// 中文 plan 中的常见词
		"检查": true, "修改": true, "新增": true, "删除": true, "调整": true,
		"确保": true, "满足": true, "功能点": true, "先完成": true, "再根据": true,
	}

	seen := make(map[string]bool)
	var result []string
	for _, tok := range tokens {
		if len(tok) < 3 || stopWords[tok] || stopWords[strings.ToLower(tok)] {
			continue
		}
		if seen[tok] {
			continue
		}
		seen[tok] = true
		result = append(result, tok)
	}
	return result
}

// ExtractRelevantSource 用 Go AST 从源文件中提取与 identifiers 相关的声明。
// 返回精简后的源代码片段（import + 匹配的函数/类型/变量声明）。
func ExtractRelevantSource(filePath, source string, identifiers []string) string {
	if source == "" {
		return ""
	}

	// 非 .go 文件不做 AST 解析，直接截断
	if !strings.HasSuffix(filePath, ".go") {
		return truncateForPrompt(source, 3000)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, source, parser.ParseComments)
	if err != nil {
		// 解析失败，回退到截断
		return truncateForPrompt(source, 3000)
	}

	identSet := make(map[string]bool, len(identifiers))
	for _, id := range identifiers {
		identSet[id] = true
		identSet[strings.ToLower(id)] = true
	}

	lines := strings.Split(source, "\n")

	// 1. 始终保留 package + import 块
	var importEnd int
	if f.Name != nil {
		importEnd = fset.Position(f.Name.End()).Line
	}
	for _, imp := range f.Imports {
		end := fset.Position(imp.End()).Line
		if end > importEnd {
			importEnd = end
		}
	}
	// import 块如果有括号，找到右括号
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.IMPORT {
			continue
		}
		end := fset.Position(genDecl.End()).Line
		if end > importEnd {
			importEnd = end
		}
	}

	// 2. 找出所有匹配的声明区间 [startLine, endLine]
	type lineRange struct {
		start int
		end   int
		name  string
	}
	var matched []lineRange

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			name := d.Name.Name
			if matchesAnyIdentifier(name, identSet) {
				start := fset.Position(d.Pos()).Line
				end := fset.Position(d.End()).Line
				matched = append(matched, lineRange{start, end, name})
			}
		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				continue
			}
			for _, spec := range d.Specs {
				specName := specDeclName(spec)
				if specName != "" && matchesAnyIdentifier(specName, identSet) {
					start := fset.Position(d.Pos()).Line
					end := fset.Position(d.End()).Line
					matched = append(matched, lineRange{start, end, specName})
				}
			}
		}
	}

	// 如果没有匹配到任何声明，说明标识符可能来自 AI 步骤描述，回退到截断
	if len(matched) == 0 {
		return truncateForPrompt(source, 5000)
	}

	// 3. 构建精简输出
	var b strings.Builder

	// package + import
	for i := 0; i < importEnd && i < len(lines); i++ {
		b.WriteString(lines[i])
		b.WriteString("\n")
	}

	// 匹配的声明之间的省略标记
	lastEnd := importEnd
	for _, r := range matched {
		skipped := r.start - lastEnd - 1
		if skipped > 2 {
			b.WriteString(fmt.Sprintf("\n// ... (省略 %d 行)\n\n", skipped))
		} else if skipped > 0 {
			for i := lastEnd; i < r.start-1 && i < len(lines); i++ {
				b.WriteString(lines[i])
				b.WriteString("\n")
			}
		}

		for i := r.start - 1; i < r.end && i < len(lines); i++ {
			b.WriteString(lines[i])
			b.WriteString("\n")
		}
		lastEnd = r.end
	}

	// 尾部省略
	remaining := len(lines) - lastEnd
	if remaining > 2 {
		b.WriteString(fmt.Sprintf("\n// ... (省略 %d 行)\n", remaining))
	} else {
		for i := lastEnd; i < len(lines); i++ {
			b.WriteString(lines[i])
			b.WriteString("\n")
		}
	}

	return b.String()
}

// matchesAnyIdentifier 检查声明名称是否匹配任何标识符。
// 支持精确匹配和子串匹配（如标识符 "Auction" 匹配函数名 "HandleAuction"）。
func matchesAnyIdentifier(declName string, identSet map[string]bool) bool {
	if identSet[declName] || identSet[strings.ToLower(declName)] {
		return true
	}
	lowerDecl := strings.ToLower(declName)
	for id := range identSet {
		if len(id) >= 3 && strings.Contains(lowerDecl, strings.ToLower(id)) {
			return true
		}
	}
	return false
}

// specDeclName 提取 GenDecl spec 的名称。
func specDeclName(spec ast.Spec) string {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		return s.Name.Name
	case *ast.ValueSpec:
		if len(s.Names) > 0 {
			return s.Names[0].Name
		}
	}
	return ""
}
