package lintconv

import (
	"go/ast"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

func (c *checker) checkDTONaming() {
	for _, file := range c.files {
		c.checkDTONamingInFile(file)
	}
}

func (c *checker) checkDTONamingInFile(file SourceFile) {
	if isDTONamingSkippablePath(file.Path) {
		return
	}
	for _, decl := range file.File.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok.String() != constant.ConvCheckTokType {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if _, ok = typeSpec.Type.(*ast.StructType); !ok {
				continue
			}
			if isDTOSuffix(typeSpec.Name.Name) {
				c.report(file, typeSpec, enum.SeverityError, constant.RuleDTONaming, constant.ConvCheckMsgDTONaming)
			}
		}
	}
}

func isDTONamingSkippablePath(path string) bool {
	return isUnder(path, constant.ConvCheckPathDTO) ||
		strings.HasSuffix(path, constant.ConvCheckSuffixTestGo) ||
		isUnder(path, constant.ConvCheckPathTest) ||
		isUnder(path, constant.ConvCheckPathLintconv)
}

func isDTOSuffix(name string) bool {
	upper := strings.ToUpper(name)
	return strings.HasSuffix(upper, constant.ConvCheckSuffixREQ) ||
		strings.HasSuffix(upper, constant.ConvCheckSuffixRSP) ||
		strings.HasSuffix(upper, constant.ConvCheckSuffixREQUEST) ||
		strings.HasSuffix(upper, constant.ConvCheckSuffixRESPONSE)
}
