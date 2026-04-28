package lintconv

import (
	"go/ast"
	"regexp"
	"strings"
)

var implementationNamePattern = regexp.MustCompile(`[a-z](List|Map|Slice|Array)$`)

func (c *checker) checkStyle() {
	for _, file := range c.files {
		c.checkCommentedCode(file)
		c.checkImplementationDetailNames(file)
	}
}

func (c *checker) checkCommentedCode(file SourceFile) {
	if !isUnder(file.Path, "internal") {
		return
	}
	for _, group := range file.File.Comments {
		for _, comment := range group.List {
			text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
			if strings.HasPrefix(text, "@") || strings.HasPrefix(text, "Package ") || strings.HasPrefix(text, "go:") || strings.HasPrefix(text, "nolint") {
				continue
			}
			if isDocTag(text) {
				continue
			}
			if looksLikeCommentedCode(text) {
				c.report(file, comment, SeverityWarning, "style.commented_code", "可能存在被注释掉的死代码，请确认是否需要删除")
			}
		}
	}
}

func (c *checker) checkImplementationDetailNames(file SourceFile) {
	if !isUnder(file.Path, "internal") || strings.HasSuffix(file.Path, "_test.go") {
		return
	}
	inspectFile(file, func(node ast.Node) {
		switch stmt := node.(type) {
		case *ast.AssignStmt:
			for _, lhs := range stmt.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if ok && isImplementationDetailName(ident.Name) {
					c.report(file, ident, SeverityWarning, "style.implementation_name", "变量命名可能暴露实现细节，建议使用复数形式")
				}
			}
		case *ast.ValueSpec:
			// Skip const declarations (variables only)
			return
		}
	})
}

func looksLikeCommentedCode(text string) bool {
	prefixes := []string{"func ", "if ", "for ", "var ", "type ", "const ", "switch ", "case ", "return ", "err :=", "err =", "ctx.", "req.", "rsp."}
	for _, prefix := range prefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

func isDocTag(text string) bool {
	return strings.HasPrefix(text, "@") || strings.HasPrefix(text, "author ") || strings.HasPrefix(text, "update ") || strings.HasPrefix(text, "receiver ") || strings.HasPrefix(text, "param ") || strings.HasPrefix(text, "return ")
}

func isImplementationDetailName(name string) bool {
	if !implementationNamePattern.MatchString(name) {
		return false
	}
	allowed := []string{"stateMap", "choiceMap", "toolCallMap", "blockMap", "blackList", "whiteList", "allowList", "denyList", "bodyMap", "dataMap", "msgMap", "messageMap", "toolMap", "existingMap", "SchemaMap", "specialNameblackList", "specialNamewhiteList"}
	for _, item := range allowed {
		if name == item {
			return false
		}
	}
	return true
}
