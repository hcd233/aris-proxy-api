package lintconv

import (
	"go/ast"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

func (c *checker) checkDTONaming() {
	for _, file := range c.files {
		if isUnder(file.Path, constant.ConvCheckPathDTO) {
			continue
		}
		if strings.HasSuffix(file.Path, constant.ConvCheckSuffixTestGo) {
			continue
		}
		if isUnder(file.Path, constant.ConvCheckPathTest) {
			continue
		}
		if isUnder(file.Path, constant.ConvCheckPathLintconv) {
			continue
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
				_, ok = typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				name := typeSpec.Name.Name
				if isDTOSuffix(name) {
					c.report(file, typeSpec, enum.SeverityError, constant.RuleDTONaming, constant.ConvCheckMsgDTONaming)
				}
			}
		}
	}
}

func isDTOSuffix(name string) bool {
	upper := strings.ToUpper(name)
	return strings.HasSuffix(upper, constant.ConvCheckSuffixREQ) ||
		strings.HasSuffix(upper, constant.ConvCheckSuffixRSP) ||
		strings.HasSuffix(upper, constant.ConvCheckSuffixREQUEST) ||
		strings.HasSuffix(upper, constant.ConvCheckSuffixRESPONSE)
}
