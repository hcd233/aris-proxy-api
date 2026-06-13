package compression

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type SearchCompressorConfig struct {
	MaxMatchesPerFile int
	AlwaysKeepFirst   bool
	AlwaysKeepLast    bool
	MaxTotalMatches   int
	MaxFiles          int
	BoostErrors       bool
}

func DefaultSearchCompressorConfig() SearchCompressorConfig {
	return SearchCompressorConfig{
		MaxMatchesPerFile: 5,
		AlwaysKeepFirst:   true,
		AlwaysKeepLast:    true,
		MaxTotalMatches:   30,
		MaxFiles:          15,
		BoostErrors:       true,
	}
}

type SearchMatch struct {
	File       string
	LineNumber int
	Content    string
	Score      float64
}

type SearchCompressionResult struct {
	Compressed           string
	OriginalMatchCount   int
	CompressedMatchCount int
}

var searchLinePattern = regexp.MustCompile(`^([^\s:]+):(\d+):(.*)`)

var errorKeywords = []string{
	"error", "exception", "fatal", "critical", "failed",
	"panic", "traceback", "segfault", "timeout", "refused",
}

type SearchCompressor struct {
	cfg SearchCompressorConfig
}

func NewSearchCompressor(cfg SearchCompressorConfig) *SearchCompressor {
	return &SearchCompressor{cfg: cfg}
}

func (c *SearchCompressor) Compress(content, context string) SearchCompressionResult {
	if content == "" {
		return SearchCompressionResult{}
	}

	fileMatches := c.parseSearchResults(content)
	if len(fileMatches) == 0 {
		return SearchCompressionResult{Compressed: content}
	}

	originalCount := 0
	for _, fm := range fileMatches {
		originalCount += len(fm)
	}

	c.scoreMatches(fileMatches, context)
	selected := c.selectMatches(fileMatches)
	compressed, compressedCount := c.formatOutput(fileMatches, selected)

	return SearchCompressionResult{
		Compressed:           compressed,
		OriginalMatchCount:   originalCount,
		CompressedMatchCount: compressedCount,
	}
}

func (c *SearchCompressor) parseSearchResults(content string) map[string][]SearchMatch {
	lines := strings.Split(content, "\n")
	result := make(map[string][]SearchMatch)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := searchLinePattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		lineNum, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		result[m[1]] = append(result[m[1]], SearchMatch{
			File:       m[1],
			LineNumber: lineNum,
			Content:    m[3],
		})
	}
	return result
}

func (c *SearchCompressor) scoreMatches(fileMatches map[string][]SearchMatch, context string) {
	contextLower := strings.ToLower(context)
	contextWords := make(map[string]bool)
	for _, w := range strings.Fields(contextLower) {
		if len(w) > 2 {
			contextWords[w] = true
		}
	}

	for file := range fileMatches {
		for i := range fileMatches[file] {
			m := &fileMatches[file][i]
			m.Score = 0.0
			contentLower := strings.ToLower(m.Content)
			for w := range contextWords {
				if strings.Contains(contentLower, w) {
					m.Score += 0.3
				}
			}
			if c.cfg.BoostErrors {
				for j, kw := range errorKeywords {
					if strings.Contains(contentLower, kw) {
						m.Score += 0.5 - float64(j)*0.1
						if m.Score < 0 {
							m.Score = 0
						}
						break
					}
				}
			}
			if m.Score > 1.0 {
				m.Score = 1.0
			}
		}
	}
}

func (c *SearchCompressor) selectMatches(fileMatches map[string][]SearchMatch) map[string][]SearchMatch {
	type fileEntry struct {
		name    string
		matches []SearchMatch
		total   float64
	}
	var files []fileEntry
	for name, matches := range fileMatches {
		total := 0.0
		for _, m := range matches {
			total += m.Score
		}
		files = append(files, fileEntry{name, matches, total})
	}
	slices.SortFunc(files, func(a, b fileEntry) int {
		if a.total > b.total {
			return -1
		}
		return 1
	})

	if len(files) > c.cfg.MaxFiles {
		files = files[:c.cfg.MaxFiles]
	}

	selected := make(map[string][]SearchMatch)
	totalSelected := 0
	for _, f := range files {
		if totalSelected >= c.cfg.MaxTotalMatches {
			break
		}
		var picked []SearchMatch
		remaining := c.cfg.MaxMatchesPerFile
		if remaining > c.cfg.MaxTotalMatches-totalSelected {
			remaining = c.cfg.MaxTotalMatches - totalSelected
		}

		if c.cfg.AlwaysKeepFirst && len(f.matches) > 0 {
			picked = append(picked, f.matches[0])
			remaining--
		}
		if c.cfg.AlwaysKeepLast && len(f.matches) > 1 && f.matches[len(f.matches)-1].LineNumber != f.matches[0].LineNumber {
			picked = append(picked, f.matches[len(f.matches)-1])
			remaining--
		}

		sorted := make([]SearchMatch, len(f.matches))
		copy(sorted, f.matches)
		slices.SortFunc(sorted, func(a, b SearchMatch) int {
			if a.Score > b.Score {
				return -1
			}
			return 1
		})

		for _, m := range sorted {
			if remaining <= 0 {
				break
			}
			already := false
			for _, p := range picked {
				if p.LineNumber == m.LineNumber && p.File == m.File {
					already = true
					break
				}
			}
			if !already {
				picked = append(picked, m)
				remaining--
			}
		}

		slices.SortFunc(picked, func(a, b SearchMatch) int {
			return a.LineNumber - b.LineNumber
		})
		selected[f.name] = picked
		totalSelected += len(picked)
	}
	return selected
}

func (c *SearchCompressor) formatOutput(original, selected map[string][]SearchMatch) (string, int) {
	var lines []string
	total := 0
	var fileNames []string
	for name := range selected {
		fileNames = append(fileNames, name)
	}
	slices.Sort(fileNames)
	for _, name := range fileNames {
		sel := selected[name]
		for _, m := range sel {
			lines = append(lines, fmt.Sprintf("%s:%d:%s", m.File, m.LineNumber, m.Content))
		}
		total += len(sel)
		orig := original[name]
		if len(orig) > len(sel) {
			omitted := len(orig) - len(sel)
			lines = append(lines, fmt.Sprintf("[... and %d more matches in %s]", omitted, name))
		}
	}
	return strings.Join(lines, "\n"), total
}
