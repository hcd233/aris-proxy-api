package compression

import (
	"context"
	"strings"
)

type Message struct {
	Role    string
	Content string
}

type PipelineResult struct {
	TokensBefore int
	TokensAfter  int
	Strategies   []string
}

type Pipeline interface {
	Compress(ctx context.Context, messages []Message) ([]Message, *PipelineResult)
}

type PipelineConfig struct {
	Enabled              bool
	MinCharsForBlock     int
	ProtectErrorOutputs  bool
	ErrorProtectMaxChars int
	ProtectRecentCode    int
}

func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		Enabled:              true,
		MinCharsForBlock:     500,
		ProtectErrorOutputs:  true,
		ErrorProtectMaxChars: 8000,
		ProtectRecentCode:    4,
	}
}

type pipeline struct {
	cfg              PipelineConfig
	smartCrusher     *SmartCrusher
	searchCompressor *SearchCompressor
	logCompressor    *LogCompressor
}

func NewPipeline(cfg PipelineConfig) Pipeline {
	return &pipeline{
		cfg:              cfg,
		smartCrusher:     NewSmartCrusher(DefaultSmartCrusherConfig()),
		searchCompressor: NewSearchCompressor(DefaultSearchCompressorConfig()),
		logCompressor:    NewLogCompressor(DefaultLogCompressorConfig()),
	}
}

func (p *pipeline) Compress(_ context.Context, messages []Message) ([]Message, *PipelineResult) {
	if !p.cfg.Enabled || len(messages) == 0 {
		dup := make([]Message, len(messages))
		copy(dup, messages)
		return dup, &PipelineResult{}
	}

	result := make([]Message, len(messages))
	copy(result, messages)

	var strategies []string
	tokensBefore := 0
	tokensAfter := 0

	recentCodeCount := 0
	for i := len(result) - 1; i >= 0 && recentCodeCount < p.cfg.ProtectRecentCode; i-- {
		ct, _ := DetectContentType(result[i].Content)
		if ct == ContentTypeSourceCode {
			recentCodeCount++
		}
	}

	recentCodeCount = 0

	for i := range result {
		msg := &result[i]
		if msg.Role != "tool" {
			continue
		}
		if len(msg.Content) < p.cfg.MinCharsForBlock {
			continue
		}

		if p.cfg.ProtectErrorOutputs && len(msg.Content) <= p.cfg.ErrorProtectMaxChars {
			if detectErrorOutput(msg.Content) {
				continue
			}
		}

		ct, _ := DetectContentType(msg.Content)

		if ct == ContentTypeSourceCode {
			recentCodeCount++
			if recentCodeCount <= p.cfg.ProtectRecentCode {
				continue
			}
		} else {
			recentCodeCount = 0
		}

		before := estimateTokens(msg.Content)

		switch ct {
		case ContentTypeJSONArray:
			if crushed, modified, strategy := p.smartCrusher.Crush(msg.Content); modified {
				msg.Content = crushed
				after := estimateTokens(crushed)
				tokensBefore += before
				tokensAfter += after
				strategies = append(strategies, strategy)
			}

		case ContentTypeSearchResults:
			r := p.searchCompressor.Compress(msg.Content, "")
			if r.Compressed != msg.Content && r.OriginalMatchCount > r.CompressedMatchCount {
				msg.Content = r.Compressed
				after := estimateTokens(r.Compressed)
				tokensBefore += before
				tokensAfter += after
				strategies = append(strategies, "search")
			}

		case ContentTypeBuildOutput:
			r := p.logCompressor.Compress(msg.Content)
			if r.Compressed != msg.Content && r.OriginalLineCount > r.CompressedLineCount {
				msg.Content = r.Compressed
				after := estimateTokens(r.Compressed)
				tokensBefore += before
				tokensAfter += after
				strategies = append(strategies, "log")
			}
		}
	}

	return result, &PipelineResult{
		TokensBefore: tokensBefore,
		TokensAfter:  tokensAfter,
		Strategies:   strategies,
	}
}

func estimateTokens(s string) int {
	return max(1, len(s)/4)
}

func detectErrorOutput(content string) bool {
	errorPatterns := []string{
		"Traceback (most recent call last)",
		"panic:",
		"Error:",
		"Exception:",
		"at ",
	}
	lower := strings.ToLower(content)
	for _, pat := range errorPatterns {
		if strings.Contains(lower, strings.ToLower(pat)) {
			return true
		}
	}
	return false
}

type noopPipeline struct{}

func NewNoopPipeline() Pipeline {
	return &noopPipeline{}
}

func (n *noopPipeline) Compress(_ context.Context, messages []Message) ([]Message, *PipelineResult) {
	dup := make([]Message, len(messages))
	copy(dup, messages)
	return dup, &PipelineResult{}
}
