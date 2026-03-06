// Package enum provides common enums for the application.
package enum

// GrammarSyntax 语法定义语法类型
//
//	@author centonhuang
//	@update 2026-03-06 10:00:00
type GrammarSyntax = string

const (

	// GrammarSyntaxLark Lark语法
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	GrammarSyntaxLark GrammarSyntax = "lark"

	// GrammarSyntaxRegex 正则表达式语法
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	GrammarSyntaxRegex GrammarSyntax = "regex"
)
