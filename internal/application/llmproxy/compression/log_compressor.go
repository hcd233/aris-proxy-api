package compression

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

type LogLevel int

const (
	LogLevelError LogLevel = iota
	LogLevelFail
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
	LogLevelTrace
	LogLevelUnknown
)

type LogCompressorConfig struct {
	MaxErrors          int
	ErrorContextLines  int
	KeepFirstError     bool
	KeepLastError      bool
	MaxStackTraces     int
	StackTraceMaxLines int
	MaxWarnings        int
	DedupWarnings      bool
	KeepSummaryLines   bool
	MaxTotalLines      int
}

func DefaultLogCompressorConfig() LogCompressorConfig {
	return LogCompressorConfig{
		MaxErrors:          10,
		ErrorContextLines:  3,
		KeepFirstError:     true,
		KeepLastError:      true,
		MaxStackTraces:     3,
		StackTraceMaxLines: 20,
		MaxWarnings:        5,
		DedupWarnings:      true,
		KeepSummaryLines:   true,
		MaxTotalLines:      100,
	}
}

type logLine struct {
	number       int
	content      string
	level        LogLevel
	isStackTrace bool
	isSummary    bool
	score        float64
}

type LogCompressionResult struct {
	Compressed          string
	OriginalLineCount   int
	CompressedLineCount int
}

var (
	logLevelPatterns = map[*regexp.Regexp]LogLevel{
		regexp.MustCompile(`\b(?:ERROR|error|Error|FATAL|fatal|Fatal|CRITICAL|critical)\b`): LogLevelError,
		regexp.MustCompile(`\b(?:FAIL|FAILED|fail|failed|Fail|Failed)\b`):                    LogLevelFail,
		regexp.MustCompile(`\b(?:WARN|WARNING|warn|warning|Warn|Warning)\b`):                  LogLevelWarn,
		regexp.MustCompile(`\b(?:INFO|info|Info)\b`):                                          LogLevelInfo,
		regexp.MustCompile(`\b(?:DEBUG|debug|Debug)\b`):                                       LogLevelDebug,
		regexp.MustCompile(`\b(?:TRACE|trace|Trace)\b`):                                       LogLevelTrace,
	}

	stackTracePatterns = []*regexp.Regexp{
		regexp.MustCompile(`^\s*Traceback \(most recent call last\)`),
		regexp.MustCompile(`^\s*File ".+", line \d+`),
		regexp.MustCompile(`^\s*at .+\(.+:\d+:\d+\)`),
		regexp.MustCompile(`^\s+at [\w.$]+\s*\(`),
	}

	summaryPatterns = []*regexp.Regexp{
		regexp.MustCompile(`^={3,}`),
		regexp.MustCompile(`^-{3,}`),
		regexp.MustCompile(`^\d+ (?:passed|failed|skipped|error|warning)`),
		regexp.MustCompile(`^(?:Tests?|Suites?):?\s+\d+`),
		regexp.MustCompile(`^(?:TOTAL|Total|Summary)`),
	}
)

type LogCompressor struct {
	cfg LogCompressorConfig
}

func NewLogCompressor(cfg LogCompressorConfig) *LogCompressor {
	return &LogCompressor{cfg: cfg}
}

func (c *LogCompressor) Compress(content string) LogCompressionResult {
	if content == "" {
		return LogCompressionResult{}
	}
	lines := strings.Split(content, "\n")
	logLines := c.parseLines(lines)
	selected := c.selectLines(logLines)
	selected = c.addContext(logLines, selected)
	selected = c.limitTotal(selected)

	compressed := c.formatOutput(lines, selected, logLines)

	return LogCompressionResult{
		Compressed:          compressed,
		OriginalLineCount:   len(lines),
		CompressedLineCount: len(selected),
	}
}

func (c *LogCompressor) parseLines(lines []string) []logLine {
	result := make([]logLine, len(lines))
	inStack := false
	stackLines := 0

	for i, line := range lines {
		ll := logLine{number: i, content: line, level: LogLevelUnknown}

		for pat, level := range logLevelPatterns {
			if pat.MatchString(line) {
				ll.level = level
				break
			}
		}

		for _, pat := range stackTracePatterns {
			if pat.MatchString(line) {
				inStack = true
				stackLines = 0
				break
			}
		}
		if inStack {
			ll.isStackTrace = true
			stackLines++
			if stackLines > c.cfg.StackTraceMaxLines || strings.TrimSpace(line) == "" {
				inStack = false
			}
		}

		for _, pat := range summaryPatterns {
			if pat.MatchString(line) {
				ll.isSummary = true
				break
			}
		}

		ll.score = c.scoreLine(ll)
		result[i] = ll
	}
	return result
}

func (c *LogCompressor) scoreLine(ll logLine) float64 {
	levelScores := map[LogLevel]float64{
		LogLevelError:   1.0,
		LogLevelFail:    1.0,
		LogLevelWarn:    0.5,
		LogLevelInfo:    0.1,
		LogLevelDebug:   0.05,
		LogLevelTrace:   0.02,
		LogLevelUnknown: 0.1,
	}
	score := levelScores[ll.level]
	if ll.isStackTrace {
		score += 0.3
	}
	if ll.isSummary {
		score += 0.4
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}

func (c *LogCompressor) selectLines(logLines []logLine) []logLine {
	var (
		errors    []logLine
		fails     []logLine
		warnings  []logLine
		stacks    [][]logLine
		summaries []logLine
		curStack  []logLine
	)

	for _, ll := range logLines {
		switch ll.level {
		case LogLevelError:
			errors = append(errors, ll)
		case LogLevelFail:
			fails = append(fails, ll)
		case LogLevelWarn:
			warnings = append(warnings, ll)
		}

		if ll.isStackTrace {
			curStack = append(curStack, ll)
		} else if len(curStack) > 0 {
			stacks = append(stacks, curStack)
			curStack = nil
		}

		if ll.isSummary {
			summaries = append(summaries, ll)
		}
	}
	if len(curStack) > 0 {
		stacks = append(stacks, curStack)
	}

	var selected []logLine

	selected = append(selected, c.selectWithFirstLast(errors, c.cfg.MaxErrors)...)
	selected = append(selected, c.selectWithFirstLast(fails, c.cfg.MaxErrors)...)

	if c.cfg.DedupWarnings {
		warnings = c.dedupSimilar(warnings)
	}
	if len(warnings) > c.cfg.MaxWarnings {
		warnings = warnings[:c.cfg.MaxWarnings]
	}
	selected = append(selected, warnings...)

	for i, stack := range stacks {
		if i >= c.cfg.MaxStackTraces {
			break
		}
		end := c.cfg.StackTraceMaxLines
		if end > len(stack) {
			end = len(stack)
		}
		selected = append(selected, stack[:end]...)
	}

	if c.cfg.KeepSummaryLines {
		selected = append(selected, summaries...)
	}

	return selected
}

func (c *LogCompressor) selectWithFirstLast(lines []logLine, maxCount int) []logLine {
	if len(lines) <= maxCount {
		return lines
	}
	var selected []logLine
	if c.cfg.KeepFirstError && len(lines) > 0 {
		selected = append(selected, lines[0])
	}
	if c.cfg.KeepLastError && len(lines) > 1 && lines[len(lines)-1].number != lines[0].number {
		selected = append(selected, lines[len(lines)-1])
	}
	remaining := maxCount - len(selected)
	if remaining > 0 {
		sorted := make([]logLine, len(lines))
		copy(sorted, lines)
		slices.SortFunc(sorted, func(a, b logLine) int {
			if a.score > b.score {
				return -1
			}
			return 1
		})
		for _, ll := range sorted {
			if remaining <= 0 {
				break
			}
			seen := false
			for _, s := range selected {
				if s.number == ll.number {
					seen = true
					break
				}
			}
			if !seen {
				selected = append(selected, ll)
				remaining--
			}
		}
	}
	return selected
}

func (c *LogCompressor) dedupSimilar(lines []logLine) []logLine {
	digitRe := regexp.MustCompile(`\d+`)
	seen := make(map[string]bool)
	var result []logLine
	for _, ll := range lines {
		sep := strings.IndexAny(ll.content, ":=")
		prefix := ll.content
		suffix := ""
		if sep >= 0 {
			prefix = ll.content[:sep]
			suffix = ll.content[sep:]
		}
		suffix = digitRe.ReplaceAllString(suffix, "N")
		normalized := prefix + suffix
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, ll)
		}
	}
	return result
}

func (c *LogCompressor) addContext(allLines, selected []logLine) []logLine {
	selectedIndices := make(map[int]bool)
	for _, ll := range selected {
		selectedIndices[ll.number] = true
	}
	contextIndices := make(map[int]bool)
	for idx := range selectedIndices {
		for i := max(0, idx-c.cfg.ErrorContextLines); i < idx; i++ {
			contextIndices[i] = true
		}
		for i := idx + 1; i < min(len(allLines), idx+c.cfg.ErrorContextLines+1); i++ {
			contextIndices[i] = true
		}
	}
	for idx := range contextIndices {
		if !selectedIndices[idx] && idx < len(allLines) {
			selected = append(selected, allLines[idx])
		}
	}
	return selected
}

func (c *LogCompressor) limitTotal(selected []logLine) []logLine {
	if len(selected) <= c.cfg.MaxTotalLines {
		return selected
	}
	sorted := make([]logLine, len(selected))
	copy(sorted, selected)
	slices.SortFunc(sorted, func(a, b logLine) int {
		if a.score > b.score {
			return -1
		}
		return 1
	})
	return sorted[:c.cfg.MaxTotalLines]
}

func (c *LogCompressor) formatOutput(allLines []string, selected, allParsed []logLine) string {
	seen := make(map[int]bool)
	var ordered []logLine
	for _, ll := range selected {
		if !seen[ll.number] {
			seen[ll.number] = true
			ordered = append(ordered, ll)
		}
	}
	slices.SortFunc(ordered, func(a, b logLine) int {
		return a.number - b.number
	})

	var outLines []string
	for _, ll := range ordered {
		outLines = append(outLines, ll.content)
	}

	omitted := len(allLines) - len(ordered)
	if omitted > 0 {
		errorCount := 0
		warnCount := 0
		for _, ll := range allParsed {
			switch ll.level {
			case LogLevelError, LogLevelFail:
				errorCount++
			case LogLevelWarn:
				warnCount++
			}
		}
		parts := make([]string, 0, 2)
		if errorCount > 0 {
			parts = append(parts, fmt.Sprintf("%d ERROR", errorCount))
		}
		if warnCount > 0 {
			parts = append(parts, fmt.Sprintf("%d WARN", warnCount))
		}
		suffix := ""
		if len(parts) > 0 {
			suffix = ": " + strings.Join(parts, ", ")
		}
		outLines = append(outLines, fmt.Sprintf("[%d lines omitted%s]", omitted, suffix))
	}

	return strings.Join(outLines, "\n")
}
