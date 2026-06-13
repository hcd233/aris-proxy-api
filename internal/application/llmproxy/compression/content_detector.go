package compression

import (
	"regexp"
	"strings"

	"github.com/bytedance/sonic"
)

type ContentType string

const (
	ContentTypeJSONArray     ContentType = "json_array"
	ContentTypeSourceCode    ContentType = "source_code"
	ContentTypeSearchResults ContentType = "search"
	ContentTypeBuildOutput   ContentType = "build"
	ContentTypeGitDiff       ContentType = "diff"
	ContentTypeHTML          ContentType = "html"
	ContentTypePlainText     ContentType = "text"
)

var (
	searchResultPattern = regexp.MustCompile(`^[^\s:]+:\d+:`)

	diffHeaderPattern = regexp.MustCompile(`^(diff --git|--- a/|@@\s+-\d+,\d+\s+\+\d+,\d+\s+@@)`)
	diffChangePattern = regexp.MustCompile(`^[+-][^+-]`)

	logPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\b(ERROR|FAIL|FAILED|FATAL|CRITICAL)\b`),
		regexp.MustCompile(`\b(WARN|WARNING)\b`),
		regexp.MustCompile(`\b(INFO|DEBUG|TRACE)\b`),
		regexp.MustCompile(`^\s*\d{4}-\d{2}-\d{2}`),
		regexp.MustCompile(`^\s*\[\d{2}:\d{2}:\d{2}\]`),
		regexp.MustCompile(`^={3,}|^-{3,}`),
		regexp.MustCompile(`^\s*PASSED|^\s*FAILED|^\s*SKIPPED`),
		regexp.MustCompile(`^npm ERR!|^yarn error|^cargo error`),
	}

	codePatterns = map[string][]*regexp.Regexp{
		"python": {
			regexp.MustCompile(`^\s*(def|class|import|from|async def)\s+\w+`),
			regexp.MustCompile(`^\s*@\w+`),
		},
		"go": {
			regexp.MustCompile(`^\s*(func|type|package|import)\s+`),
			regexp.MustCompile(`^\s*func\s+\([^)]+\)\s+\w+`),
		},
		"rust": {
			regexp.MustCompile(`^\s*(fn|struct|enum|impl|mod|use|pub)\s+`),
		},
		"javascript": {
			regexp.MustCompile(`^\s*(function|const|let|var|class|import|export)\s+`),
		},
	}
)

func DetectContentType(content string) (ContentType, float64) {
	content = strings.TrimSpace(content)
	if content == "" {
		return ContentTypePlainText, 0.0
	}

	if result, conf := tryDetectJSON(content); result != nil {
		return *result, conf
	}

	if result, conf := tryDetectDiff(content); result != nil && conf >= 0.7 {
		return *result, conf
	}

	if result, conf := tryDetectSearch(content); result != nil && conf >= 0.6 {
		return *result, conf
	}

	if result, conf := tryDetectLog(content); result != nil && conf >= 0.5 {
		return *result, conf
	}

	if result, conf := tryDetectCode(content); result != nil && conf >= 0.5 {
		return *result, conf
	}

	return ContentTypePlainText, 0.5
}

func tryDetectJSON(content string) (*ContentType, float64) {
	if !strings.HasPrefix(content, "[") {
		return nil, 0
	}
	var parsed []map[string]any
	if err := sonic.UnmarshalString(content, &parsed); err != nil {
		var fallback []any
		if err2 := sonic.UnmarshalString(content, &fallback); err2 != nil {
			return nil, 0
		}
		ct := ContentTypeJSONArray
		return &ct, 0.8
	}
	ct := ContentTypeJSONArray
	return &ct, 1.0
}

func tryDetectDiff(content string) (*ContentType, float64) {
	lines := strings.SplitN(content, "\n", 501)
	if len(lines) > 500 {
		lines = lines[:500]
	}
	var headerMatches, changeMatches int
	for _, line := range lines {
		if diffHeaderPattern.MatchString(line) {
			headerMatches++
		}
		if diffChangePattern.MatchString(line) {
			changeMatches++
		}
	}
	if headerMatches == 0 {
		return nil, 0
	}
	ct := ContentTypeGitDiff
	conf := 0.5 + float64(headerMatches)*0.2 + float64(changeMatches)*0.05
	if conf > 1.0 {
		conf = 1.0
	}
	return &ct, conf
}

func tryDetectSearch(content string) (*ContentType, float64) {
	lines := strings.SplitN(content, "\n", 101)
	if len(lines) > 100 {
		lines = lines[:100]
	}
	var matching, nonEmpty int
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		nonEmpty++
		if searchResultPattern.MatchString(line) {
			matching++
		}
	}
	if nonEmpty == 0 || matching == 0 {
		return nil, 0
	}
	ratio := float64(matching) / float64(nonEmpty)
	if ratio < 0.3 {
		return nil, 0
	}
	ct := ContentTypeSearchResults
	conf := 0.4 + ratio*0.6
	if conf > 1.0 {
		conf = 1.0
	}
	return &ct, conf
}

func tryDetectLog(content string) (*ContentType, float64) {
	lines := strings.SplitN(content, "\n", 201)
	if len(lines) > 200 {
		lines = lines[:200]
	}
	var patternMatches, errorMatches, nonEmpty int
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		nonEmpty++
		for i, pat := range logPatterns {
			if pat.MatchString(line) {
				patternMatches++
				if i < 2 {
					errorMatches++
				}
				break
			}
		}
	}
	if nonEmpty == 0 || patternMatches == 0 {
		return nil, 0
	}
	ratio := float64(patternMatches) / float64(nonEmpty)
	if ratio < 0.1 {
		return nil, 0
	}
	ct := ContentTypeBuildOutput
	conf := 0.3 + ratio*0.5 + float64(errorMatches)*0.05
	if conf > 1.0 {
		conf = 1.0
	}
	return &ct, conf
}

func tryDetectCode(content string) (*ContentType, float64) {
	lines := strings.SplitN(content, "\n", 101)
	if len(lines) > 100 {
		lines = lines[:100]
	}
	langScores := make(map[string]int)
	for _, line := range lines {
		for lang, patterns := range codePatterns {
			for _, pat := range patterns {
				if pat.MatchString(strings.TrimSpace(line)) {
					langScores[lang]++
					break
				}
			}
		}
	}
	if len(langScores) == 0 {
		return nil, 0
	}
	var bestScore int
	for _, score := range langScores {
		if score > bestScore {
			bestScore = score
		}
	}
	if bestScore < 3 {
		return nil, 0
	}
	var nonEmpty int
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		nonEmpty = 1
	}
	ratio := float64(bestScore) / float64(nonEmpty)
	ct := ContentTypeSourceCode
	conf := 0.4 + ratio*0.4 + float64(bestScore)*0.02
	if conf > 1.0 {
		conf = 1.0
	}
	return &ct, conf
}
