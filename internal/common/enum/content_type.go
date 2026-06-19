package enum

// ContentType 工具输出的内容类型。
type ContentType int

const (
	ContentTypeJsonArray ContentType = iota
	ContentTypeSearchResults
	ContentTypeBuildOutput
	ContentTypeSourceCode
	ContentTypeGitDiff
	ContentTypeHtml
	ContentTypePlainText
)

func (c ContentType) String() string {
	switch c {
	case ContentTypeJsonArray:
		return "json_array"
	case ContentTypeSearchResults:
		return "search_results"
	case ContentTypeBuildOutput:
		return "build_output"
	case ContentTypeSourceCode:
		return "source_code"
	case ContentTypeGitDiff:
		return "git_diff"
	case ContentTypeHtml:
		return "html"
	default:
		return "plain_text"
	}
}
