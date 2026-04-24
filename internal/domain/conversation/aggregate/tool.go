package aggregate

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/common/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
)

// Tool 工具聚合根（内容寻址去重）
//
//	@author centonhuang
//	@update 2026-04-23 10:45:00
type Tool struct {
	aggregate.Base

	content  *vo.UnifiedTool
	checksum string
}

// RecordTool 构造一条待持久化的 Tool 聚合
//
//	@param content *vo.UnifiedTool
//	@param checksum string
//	@return *Tool
//	@return error
//	@author centonhuang
//	@update 2026-04-23 10:45:00
func RecordTool(content *vo.UnifiedTool, checksum string) (*Tool, error) {
	if content == nil {
		return nil, ierr.New(ierr.ErrValidation, "tool content is nil")
	}
	if checksum == "" {
		return nil, ierr.New(ierr.ErrValidation, "tool checksum is empty")
	}
	return &Tool{content: content, checksum: checksum}, nil
}

// RestoreTool 从仓储重建 Tool 聚合
//
//	@param id uint
//	@param content *vo.UnifiedTool
//	@param checksum string
//	@return *Tool
//	@author centonhuang
//	@update 2026-04-23 10:45:00
func RestoreTool(id uint, content *vo.UnifiedTool, checksum string) *Tool {
	t := &Tool{content: content, checksum: checksum}
	t.SetID(id)
	return t
}

// AggregateType 实现 aggregate.Root 接口
func (*Tool) AggregateType() string { return constant.AggregateTypeTool }

// Content 返回工具内容
func (t *Tool) Content() *vo.UnifiedTool { return t.content }

// Checksum 返回校验和
func (t *Tool) Checksum() string { return t.checksum }
