package proxyutil

import "github.com/bytedance/sonic"

// upstreamJSONAPI 用于把请求体序列化转发给上游 LLM。
//
// 关键特性：SortMapKeys=true。
// 上游（OpenAI / Anthropic 等）的 prompt cache 命中前提是
// 请求体中除增量消息外的字节段（system / tools / 历史消息）跨轮次完全一致。
//
// 项目内 tools.parameters 使用 vo.JSONSchemaProperty，其中 Properties 字段是
// map[string]*JSONSchemaProperty。sonic 的 ConfigDefault 对 map 不排序，输出顺序由
// Go runtime map 迭代决定，每次都不同；这会让上游每轮看到不同的 tools 字节，
// 导致 prompt cache 在工具段就开始 miss。
//
// 该 encoder 仅用于"代理转发到上游 LLM"的请求体序列化，不影响 fiber 响应、内部 API、
// 日志等其它路径的序列化性能。
//
// WARNING: SortMapKeys 有性能开销，故仅在该出口启用，请勿全局替换 sonic.Marshal。
var upstreamJSONAPI = sonic.Config{
	SortMapKeys: true,
}.Froze()

// MarshalUpstreamBody 用 SortMapKeys=true 的 sonic API 序列化上游请求体。
//
// 用于"打包请求转发到上游 LLM"的所有序列化点。直接使用 sonic.Marshal 会因
// JSONSchemaProperty.Properties / map[string]any 等字段的迭代顺序随机化导致字节漂移，
// 进而破坏上游 prompt cache 命中。
func MarshalUpstreamBody(v any) ([]byte, error) {
	return upstreamJSONAPI.Marshal(v)
}
