package compression

import (
	"math"

	"github.com/bytedance/sonic"
)

type SmartCrusherConfig struct {
	MinItemsToAnalyze   int
	MinTokensToCrush    int
	VarianceThreshold   float64
	UniquenessThreshold float64
	SimilarityThreshold float64
	MaxItemsAfterCrush  int
	FirstFraction       float64
	LastFraction        float64
	DedupIdentical      bool
}

func DefaultSmartCrusherConfig() SmartCrusherConfig {
	return SmartCrusherConfig{
		MinItemsToAnalyze:   5,
		MinTokensToCrush:    200,
		VarianceThreshold:   2.0,
		UniquenessThreshold: 0.1,
		SimilarityThreshold: 0.8,
		MaxItemsAfterCrush:  15,
		FirstFraction:       0.3,
		LastFraction:        0.15,
		DedupIdentical:      true,
	}
}

type SmartCrusher struct {
	cfg SmartCrusherConfig
}

func NewSmartCrusher(cfg SmartCrusherConfig) *SmartCrusher {
	return &SmartCrusher{cfg: cfg}
}

func (c *SmartCrusher) Crush(content string) (string, bool, string) {
	var items []any
	if err := sonic.UnmarshalString(content, &items); err != nil {
		return content, false, "passthrough"
	}
	n := len(items)
	if n < c.cfg.MinItemsToAnalyze {
		return content, false, "passthrough"
	}
	estimatedTokens := len(content) / 4
	if estimatedTokens < c.cfg.MinTokensToCrush {
		return content, false, "passthrough"
	}

	firstCount := int(math.Ceil(float64(n) * c.cfg.FirstFraction))
	lastCount := int(math.Ceil(float64(n) * c.cfg.LastFraction))

	keepSet := make(map[int]bool)
	for i := 0; i < firstCount && i < n; i++ {
		keepSet[i] = true
	}
	for i := n - lastCount; i < n; i++ {
		if i >= 0 {
			keepSet[i] = true
		}
	}

	var kept []any
	seen := make(map[string]bool)
	for i, item := range items {
		if keepSet[i] {
			key := itemKey(item)
			if !c.cfg.DedupIdentical || !seen[key] {
				seen[key] = true
				kept = append(kept, item)
			}
			continue
		}
		if len(kept) >= c.cfg.MaxItemsAfterCrush {
			continue
		}
		if isChangePoint(items, i) {
			key := itemKey(item)
			if !c.cfg.DedupIdentical || !seen[key] {
				seen[key] = true
				kept = append(kept, item)
			}
		}
	}

	if len(kept) >= n {
		return content, false, "passthrough"
	}

	compressed, _ := sonic.MarshalString(kept)
	return compressed, true, "smart_crusher"
}

func itemKey(item any) string {
	b, _ := sonic.MarshalString(item)
	return b
}

func isChangePoint(items []any, i int) bool {
	if i <= 0 || i >= len(items)-1 {
		return false
	}
	prev, okPrev := items[i-1].(map[string]any)
	curr, okCurr := items[i].(map[string]any)
	next, okNext := items[i+1].(map[string]any)
	if !okPrev || !okCurr || !okNext {
		return false
	}
	prevSame := countChangedFields(prev, curr)
	nextSame := countChangedFields(curr, next)
	return prevSame > 0 || nextSame > 0
}

func countChangedFields(a, b map[string]any) int {
	count := 0
	for k, av := range a {
		bv, ok := b[k]
		if !ok || av != bv {
			count++
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			count++
		}
	}
	return count
}
