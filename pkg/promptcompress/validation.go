package promptcompress

import "strings"

type compressionValidationResult struct {
	Valid           bool
	Errors          []string
	Warnings        []string
	FallbackApplied bool
}

func validateCompression(original string, compressed string) compressionValidationResult {
	result := compressionValidationResult{Valid: true}
	if original != "" && strings.TrimSpace(compressed) == "" {
		result.Errors = append(result.Errors, "compressed text is empty")
	}
	requireExactPresence("fenced code block", findFencedCodeBlocks(original), compressed, &result)
	requireExactPresence("inline code", collectInlineCode(original), compressed, &result)
	requireExactPresence("URL", collectUrls(original), compressed, &result)
	requireExactPresence("markdown link", collectMarkdownLinks(original), compressed, &result)
	requireExactPresence("frontmatter", collectFrontmatter(original), compressed, &result)
	requireExactPresence("heading", collectHeadings(original), compressed, &result)
	requireExactPresence("table row", collectTableRows(original), compressed, &result)
	requireExactPresence("math block", collectMathBlocks(original), compressed, &result)
	requireExactPresence("inline math", collectInlineMath(original), compressed, &result)
	requireExactPresence("LaTeX block", collectLatexBlocks(original), compressed, &result)
	requireExactPresence("version", collectVersions(original), compressed, &result)
	requireExactPresence("CONST_CASE", collectConstCase(original), compressed, &result)

	if len(findFencedCodeBlocks(compressed)) < len(findFencedCodeBlocks(original)) {
		result.Errors = append(result.Errors, "fenced code block count dropped")
	}
	if len([]rune(compressed)) > len([]rune(original)) {
		result.Warnings = append(result.Warnings, "compressed text is longer than original")
	}
	result.Valid = len(result.Errors) == 0
	result.FallbackApplied = !result.Valid
	return result
}

func requireExactPresence(label string, originalItems []string, compressed string, result *compressionValidationResult) {
	for _, item := range originalItems {
		if !strings.Contains(compressed, item) {
			result.Errors = append(result.Errors, label+" changed or missing")
		}
	}
}

func findFencedCodeBlocks(text string) []string {
	var blocks []string
	lines := splitLinesKeepingEndings(text)
	for i := 0; i < len(lines); {
		line := lines[i]
		if line == "" && i == len(lines)-1 {
			break
		}
		opening := fencedCodeOpenPattern.FindStringSubmatch(line)
		if opening == nil {
			i++
			continue
		}
		fence := opening[2]
		fenceChar := fence[0]
		minLen := len(fence)
		block := line
		closed := false
		j := i + 1
		for ; j < len(lines); j++ {
			candidate := lines[j]
			block += candidate
			closing := fencedCodeClosePattern.FindStringSubmatch(candidate)
			if closing != nil && closing[2][0] == fenceChar && len(closing[2]) >= minLen {
				closed = true
				break
			}
		}
		if closed {
			blocks = append(blocks, block)
			i = j + 1
			continue
		}
		i++
	}
	return blocks
}

func collectInlineCode(text string) []string {
	var matches []string
	for index := 0; index < len(text); index++ {
		if text[index] != '`' {
			continue
		}
		end := strings.Index(text[index+1:], "`")
		if end == -1 {
			break
		}
		end += index + 1
		content := text[index+1 : end]
		if content != "" && !strings.Contains(content, "\n") {
			matches = append(matches, text[index:end+1])
			index = end
		}
	}
	return matches
}

func collectUrls(text string) []string {
	var matches []string
	for index := 0; index < len(text); index++ {
		startsHTTP := strings.HasPrefix(text[index:], "http://")
		startsHTTPS := strings.HasPrefix(text[index:], "https://")
		if !startsHTTP && !startsHTTPS {
			continue
		}
		if index > 0 && isWordByte(text[index-1]) {
			continue
		}
		end := index + len("http://")
		if startsHTTPS {
			end = index + len("https://")
		}
		for end < len(text) && !isURLTerminator(text[end]) {
			end++
		}
		matches = append(matches, text[index:end])
		index = end - 1
	}
	return matches
}

func collectMarkdownLinks(text string) []string {
	var matches []string
	for index := 0; index < len(text); index++ {
		if text[index] != '[' {
			continue
		}
		labelEndOffset := strings.Index(text[index+1:], "]")
		if labelEndOffset == -1 {
			continue
		}
		labelEnd := index + 1 + labelEndOffset
		if labelEnd-index-1 > 1000 || strings.Contains(text[index+1:labelEnd], "\n") {
			continue
		}
		if labelEnd+1 >= len(text) || text[labelEnd+1] != '(' {
			continue
		}
		targetStart := labelEnd + 2
		maxTargetEnd := minInt(len(text), targetStart+2000)
		targetEnd := -1
		for cursor := targetStart; cursor < maxTargetEnd; cursor++ {
			if text[cursor] == '\n' {
				break
			}
			if text[cursor] == ')' {
				targetEnd = cursor
				break
			}
		}
		if targetEnd != -1 {
			matches = append(matches, text[index:targetEnd+1])
			index = targetEnd
		}
	}
	return matches
}

func collectFrontmatter(text string) []string {
	if !strings.HasPrefix(text, "---\n") {
		return nil
	}
	closeOffset := strings.Index(text[4:], "\n---")
	if closeOffset == -1 {
		return nil
	}
	closeIndex := closeOffset + 4
	closeEndOffset := strings.Index(text[closeIndex+4:], "\n")
	end := len(text)
	if closeEndOffset != -1 {
		end = closeIndex + 4 + closeEndOffset + 1
	}
	return []string{text[:end]}
}

func collectHeadings(text string) []string {
	return filterValidationLines(text, func(line string) bool {
		markerCount := 0
		for markerCount < len(line) && line[markerCount] == '#' {
			markerCount++
		}
		if markerCount < 1 || markerCount > 6 {
			return false
		}
		if markerCount >= len(line) || !isPreviewWhitespace(line[markerCount]) {
			return false
		}
		for index := markerCount + 1; index < len(line); index++ {
			if !isPreviewWhitespace(line[index]) {
				return true
			}
		}
		return false
	})
}

func collectTableRows(text string) []string {
	return filterValidationLines(text, func(line string) bool {
		index := 0
		for index < len(line) && isHorizontalWhitespaceByte(line[index]) {
			index++
		}
		if index >= len(line) || line[index] != '|' {
			return false
		}
		pipeCount := 0
		for ; index < len(line); index++ {
			if line[index] == '|' {
				pipeCount++
			}
		}
		return pipeCount >= 2
	})
}

func collectMathBlocks(text string) []string {
	var matches []string
	for index := 0; index < len(text); {
		startOffset := strings.Index(text[index:], "$$")
		if startOffset == -1 {
			break
		}
		start := index + startOffset
		endOffset := strings.Index(text[start+2:], "$$")
		if endOffset == -1 {
			break
		}
		end := start + 2 + endOffset
		if end-start-2 <= 10000 {
			matches = append(matches, text[start:end+2])
			index = end + 2
			continue
		}
		index = start + 2
	}
	for index := 0; index < len(text); {
		startOffset := strings.Index(text[index:], `\[`)
		if startOffset == -1 {
			break
		}
		start := index + startOffset
		endOffset := strings.Index(text[start+2:], `\]`)
		if endOffset == -1 {
			break
		}
		end := start + 2 + endOffset
		if end-start-2 <= 10000 {
			matches = append(matches, text[start:end+2])
			index = end + 2
			continue
		}
		index = start + 2
	}
	return matches
}

func collectInlineMath(text string) []string {
	var matches []string
	for index := 0; index < len(text); index++ {
		if text[index] != '$' {
			continue
		}
		if index > 0 && text[index-1] == '$' {
			continue
		}
		if index+1 >= len(text) || isPreviewWhitespace(text[index+1]) || text[index+1] == '$' || isASCIIDigit(text[index+1]) {
			continue
		}
		maxEnd := minInt(len(text), index+1+160)
		for cursor := index + 1; cursor < maxEnd; cursor++ {
			char := text[cursor]
			if char == '\n' {
				break
			}
			if char == '\\' {
				cursor++
				continue
			}
			if char != '$' {
				continue
			}
			if isPreviewWhitespace(text[cursor-1]) {
				continue
			}
			if cursor+1 < len(text) && text[cursor+1] == '$' {
				continue
			}
			matches = append(matches, text[index:cursor+1])
			index = cursor
			break
		}
	}
	return matches
}

func collectLatexBlocks(text string) []string {
	var matches []string
	for index := 0; index < len(text); {
		startOffset := strings.Index(text[index:], `\begin{`)
		if startOffset == -1 {
			break
		}
		start := index + startOffset
		envStart := start + len(`\begin{`)
		envEndOffset := strings.Index(text[envStart:], "}")
		if envEndOffset == -1 {
			index = envStart
			continue
		}
		envEnd := envStart + envEndOffset
		if envEnd-envStart < 1 || envEnd-envStart > 50 {
			index = envStart
			continue
		}
		env := text[envStart:envEnd]
		if !isLatexEnvName(env) {
			index = envStart
			continue
		}
		closeToken := `\end{` + env + `}`
		closeOffset := strings.Index(text[envEnd+1:], closeToken)
		if closeOffset == -1 {
			index = envEnd + 1
			continue
		}
		closeStart := envEnd + 1 + closeOffset
		if closeStart-envEnd-1 <= 10000 {
			end := closeStart + len(closeToken)
			matches = append(matches, text[start:end])
			index = end
			continue
		}
		index = envEnd + 1
	}
	return matches
}

func collectVersions(text string) []string {
	var matches []string
	for index := 0; index < len(text); index++ {
		if !isASCIIDigit(text[index]) {
			continue
		}
		if index > 0 && isWordByte(text[index-1]) {
			continue
		}
		cursor := readDigits(text, index)
		dotGroups := 0
		for dotGroups < 3 && cursor < len(text) && text[cursor] == '.' && cursor+1 < len(text) && isASCIIDigit(text[cursor+1]) {
			cursor = readDigits(text, cursor+1)
			dotGroups++
		}
		if dotGroups < 1 {
			continue
		}
		if cursor < len(text) && (text[cursor] == '-' || text[cursor] == '+') && cursor+1 < len(text) && isVersionSuffixByte(text[cursor+1]) {
			cursor++
			for cursor < len(text) && isVersionSuffixByte(text[cursor]) {
				cursor++
			}
		}
		if cursor < len(text) && isWordByte(text[cursor]) {
			continue
		}
		matches = append(matches, text[index:cursor])
		index = cursor - 1
	}
	return matches
}

func collectConstCase(text string) []string {
	var matches []string
	for index := 0; index < len(text); index++ {
		if !isASCIIUpper(text[index]) {
			continue
		}
		if index > 0 && isWordByte(text[index-1]) {
			continue
		}
		cursor := index + 1
		hasUnderscore := false
		for cursor < len(text) {
			char := text[cursor]
			if char == '_' {
				if cursor+1 >= len(text) || (!isASCIIUpper(text[cursor+1]) && !isASCIIDigit(text[cursor+1])) {
					break
				}
				hasUnderscore = true
				cursor++
				continue
			}
			if !isASCIIUpper(char) && !isASCIIDigit(char) {
				break
			}
			cursor++
		}
		if !hasUnderscore || cursor < len(text) && isWordByte(text[cursor]) {
			continue
		}
		matches = append(matches, text[index:cursor])
		index = cursor - 1
	}
	return matches
}

func filterValidationLines(text string, keep func(string) bool) []string {
	var matches []string
	start := 0
	for index := 0; index < len(text); index++ {
		if text[index] != '\n' {
			continue
		}
		line := text[start:index]
		if keep(line) {
			matches = append(matches, line)
		}
		start = index + 1
	}
	line := text[start:]
	if keep(line) {
		matches = append(matches, line)
	}
	return matches
}

func readDigits(text string, start int) int {
	cursor := start
	for cursor < len(text) && isASCIIDigit(text[cursor]) {
		cursor++
	}
	return cursor
}

func isURLTerminator(char byte) bool {
	return isPreviewWhitespace(char) || char == ')' || char == ']' || char == '"' || char == '\'' || char == '>'
}

func isPreviewWhitespace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r' || char == '\f' || char == '\v'
}

func isHorizontalWhitespaceByte(char byte) bool {
	return char == ' ' || char == '\t'
}

func isASCIIDigit(char byte) bool {
	return char >= '0' && char <= '9'
}

func isASCIIUpper(char byte) bool {
	return char >= 'A' && char <= 'Z'
}

func isASCIILetter(char byte) bool {
	return (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')
}

func isASCIIAlphaNumeric(char byte) bool {
	return isASCIILetter(char) || isASCIIDigit(char)
}

func isWordByte(char byte) bool {
	return isASCIIAlphaNumeric(char) || char == '_'
}

func isVersionSuffixByte(char byte) bool {
	return isASCIIAlphaNumeric(char) || char == '.' || char == '-'
}

func isLatexEnvName(value string) bool {
	for index := 0; index < len(value); index++ {
		if !isASCIILetter(value[index]) && value[index] != '*' {
			return false
		}
	}
	return value != ""
}
