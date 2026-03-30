package constant

const (
	// SummarizeMaxRetries Session总结最大重试次数
	//
	//	@author centonhuang
	//	@update 2026-03-26 10:00:00
	SummarizeMaxRetries = 3

	// SummarizeMaxTokens Session总结最大Token数
	//	@update 2026-03-31 01:41:45
	SummarizeMaxTokens = 20

	// SessionSummarizerAgentName Agent名称
	//
	//	@author centonhuang
	//	@update 2026-03-31 18:00:00
	SessionSummarizerAgentName = "SessionSummarizer"

	// SessionSummarizerAgentDescription Agent描述
	//
	//	@author centonhuang
	//	@update 2026-03-31 18:00:00
	SessionSummarizerAgentDescription = "Summarize the session content into a concise summary."

	// SessionSummarizerAgentInstruction Agent指令 (Metaprompt风格)
	//
	// 使用元提示工程技术确保Agent严格遵循指令要求。
	//
	//	@author centonhuang
	//	@update 2026-03-31 18:00:00
	SessionSummarizerAgentInstruction = `# 角色定义
你是一个专业的对话总结助手。你的唯一任务是将对话内容转化为简洁的中文摘要。

## 任务描述
分析提供的对话内容，提取核心主题，生成一段简短的总结。

## 输出规范
- **语言**: 必须且只能使用简体中文
- **长度**: 严格控制在 5-10 个中文字符
- **格式**: 纯文本，禁止添加任何标点符号、前缀或后缀
- **内容**: 准确捕捉对话的核心主题或目的

## 禁止事项
- 禁止使用英文、日文或其他任何非中文语言
- 禁止输出解释、分析过程或额外说明
- 禁止使用引号、括号或其他标点符号包裹输出
- 禁止输出"总结:"、"摘要:"等前缀

## 示例输入
用户: 你好，请问怎么学习Go语言？
助手: 建议从官方文档开始，然后实践项目...

## 示例输出
Go语言学习方法

## 执行指令
直接输出总结内容，不要有任何其他内容。`
)
