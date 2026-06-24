package lintconv

import (
	"go/ast"
	"go/token"
	"regexp"
	"slices"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

var (
	codePrefixes = []string{constant.ConvCheckPrefixFunc, constant.ConvCheckPrefixIf, constant.ConvCheckPrefixFor, constant.ConvCheckPrefixVar, constant.ConvCheckPrefixTypeKw, constant.ConvCheckPrefixConstKw, constant.ConvCheckPrefixSwitch, constant.ConvCheckPrefixCase, constant.ConvCheckPrefixReturn, constant.ConvCheckPrefixErrAssign, constant.ConvCheckPrefixErrEq, constant.ConvCheckPrefixCtxDot, constant.ConvCheckPrefixReqDot, constant.ConvCheckPrefixRspDot}

	docTagPrefixes = []string{constant.ConvCheckPrefixAtSign, constant.ConvCheckPrefixAuthor, constant.ConvCheckPrefixUpdate, constant.ConvCheckPrefixReceiver, constant.ConvCheckPrefixParam, constant.ConvCheckPrefixReturnDoc}

	allowedImplNames = []string{constant.ConvCheckNameStateMap, constant.ConvCheckNameChoiceMap, constant.ConvCheckNameToolCallMap, constant.ConvCheckNameBlockMap, constant.ConvCheckNameBlackList, constant.ConvCheckNameWhiteList, constant.ConvCheckNameAllowList, constant.ConvCheckNameDenyList, constant.ConvCheckNameBodyMap, constant.ConvCheckNameDataMap, constant.ConvCheckNameMsgMap, constant.ConvCheckNameMessageMap, constant.ConvCheckNameToolMap, constant.ConvCheckNameExistingMap, constant.ConvCheckNameSchemaMap, constant.ConvCheckNameSpecialNameBlackList, constant.ConvCheckNameSpecialNameWhiteList}
)

var (
	implementationNamePattern = regexp.MustCompile(`[a-z](List|Map|Slice|Array)$`)
	importAliasPattern        = regexp.MustCompile(`^[a-z]+$`)
)

func (c *checker) checkStyle() {
	for _, file := range c.files {
		c.checkCommentedCode(file)
		c.checkImplementationDetailNames(file)
		c.checkLocalConst(file)
		c.checkTypeAlias(file)
		c.checkImportAliasUnderscore(file)
	}
}

func (c *checker) checkImportAliasUnderscore(file SourceFile) {
	inspectFile(file, func(node ast.Node) {
		spec, ok := node.(*ast.ImportSpec)
		if !ok {
			return
		}
		if spec.Name == nil {
			return
		}
		name := spec.Name.Name
		if name == "_" || name == "." {
			return
		}
		if !importAliasPattern.MatchString(name) {
			c.report(file, spec, enum.SeverityError, constant.RuleImportAliasUnderscore, constant.ConvCheckMsgImportAliasUnderscore)
		}
	})
}

func (c *checker) checkCommentedCode(file SourceFile) {
	if !isUnder(file.Path, constant.ConvCheckPathInternal) {
		return
	}
	for _, group := range file.File.Comments {
		for _, comment := range group.List {
			text := strings.TrimSpace(strings.TrimPrefix(comment.Text, constant.ConvCheckPrefixComment))
			if strings.HasPrefix(text, constant.ConvCheckPrefixAtSign) || strings.HasPrefix(text, constant.ConvCheckPrefixPackage) || strings.HasPrefix(text, constant.ConvCheckPrefixGoColon) || strings.HasPrefix(text, constant.ConvCheckPrefixNolint) {
				continue
			}
			if isDocTag(text) {
				continue
			}
			if looksLikeCommentedCode(text) {
				c.report(file, comment, enum.SeverityWarning, constant.RuleCommentedCode, constant.ConvCheckMsgCommentedCode)
			}
		}
	}
}

func (c *checker) checkImplementationDetailNames(file SourceFile) {
	if !isUnder(file.Path, constant.ConvCheckPathInternal) || strings.HasSuffix(file.Path, constant.ConvCheckSuffixTestGo) {
		return
	}
	inspectFile(file, func(node ast.Node) {
		switch stmt := node.(type) {
		case *ast.AssignStmt:
			for _, lhs := range stmt.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if ok && isImplementationDetailName(ident.Name) {
					c.report(file, ident, enum.SeverityWarning, constant.RuleImplementation, constant.ConvCheckMsgImplementation)
				}
			}
		case *ast.ValueSpec:
			return
		}
	})
}

func looksLikeCommentedCode(text string) bool {
	for _, prefix := range codePrefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

func isDocTag(text string) bool {
	for _, prefix := range docTagPrefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

func isImplementationDetailName(name string) bool {
	if !implementationNamePattern.MatchString(name) {
		return false
	}
	return !slices.Contains(allowedImplNames, name)
}

func (c *checker) checkLocalConst(file SourceFile) {
	if !isUnder(file.Path, constant.ConvCheckPathInternal) || strings.HasSuffix(file.Path, constant.ConvCheckSuffixTestGo) {
		return
	}
	if isUnder(file.Path, constant.ConvCheckPathConstant) || isUnder(file.Path, constant.ConvCheckPathEnum) || isUnder(file.Path, constant.ConvCheckPathCommonEnum) {
		return
	}
	inspectFile(file, func(node ast.Node) {
		decl, ok := node.(*ast.GenDecl)
		if !ok || decl.Tok != token.CONST {
			return
		}
		c.report(file, decl, enum.SeverityError, constant.RuleLocalConst, constant.ConvCheckMsgLocalConst)
	})
}

func isEnumPackage(path string) bool {
	return isUnder(path, constant.ConvCheckPathEnum) || isUnder(path, constant.ConvCheckPathCommonEnum)
}

func isVOPackage(path string) bool {
	return strings.Contains(slashPath(path), constant.ConvCheckVOSep)
}

func (c *checker) checkTypeAlias(file SourceFile) {
	if !isUnder(file.Path, constant.ConvCheckPathInternal) || strings.HasSuffix(file.Path, constant.ConvCheckSuffixTestGo) {
		return
	}
	if isEnumPackage(file.Path) || isVOPackage(file.Path) || isUnder(file.Path, constant.ConvCheckPathDTO) {
		return
	}
	for _, decl := range file.File.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if ts.Assign != 0 {
				c.report(file, ts, enum.SeverityError, constant.RuleTypeAlias, constant.ConvCheckMsgTypeAlias)
				continue
			}
			switch ts.Type.(type) {
			case *ast.StructType, *ast.InterfaceType:
				continue
			}
			c.report(file, ts, enum.SeverityError, constant.RuleTypeAlias, constant.ConvCheckMsgTypeDef)
		}
	}
}
