package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// renderMathpixMarkdown converts Mathpix's LaTex math delimiters to Obsidian's Markdown syntax.
func renderMathpixMarkdown(markdown string) string {
	lines := convertMathpixStructures(stripMathpixTitle(strings.Split(markdown, "\n")))
	output := make([]string, 0, len(lines))
	inBlockEquation := false
	inDollarBlock := false
	latexEnvironmentDepth := 0

	for _, line := range lines {
		if inDollarBlock {
			output = append(output, line)
			if line == "$$" {
				inDollarBlock = false
			}
			continue
		}
		if inBlockEquation {
			if line == `\]` {
				output = append(output, "$$")
				inBlockEquation = false
				continue
			}
			output = append(output, line)
			continue
		}
		if line == "$$" {
			output = append(output, line)
			inDollarBlock = true
			continue
		}
		if latexEnvironmentDepth > 0 {
			output = append(output, line)
			starts, ends := mathEnvironmentCounts(line)
			latexEnvironmentDepth += starts - ends
			if latexEnvironmentDepth <= 0 {
				output = append(output, "$$")
				latexEnvironmentDepth = 0
			}
			continue
		}
		if line == `\[` {
			output = append(output, "$$")
			inBlockEquation = true
			continue
		}
		if isMarkdownStructure(line) {
			output = append(output, convertInlineMath(line))
			continue
		}
		starts, ends := mathEnvironmentCounts(line)
		if starts > 0 {
			output = append(output, "$$", line)
			latexEnvironmentDepth = starts - ends
			if latexEnvironmentDepth <= 0 {
				output = append(output, "$$")
				latexEnvironmentDepth = 0
			}
			continue
		}
		if heading, ok := markdownHeading(line); ok {
			output = append(output, convertInlineMath(heading))
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "\\") {
			output = append(output, convertInlineMath(line))
			continue
		}
		for _, bullet := range bulletize(line) {
			output = append(output, convertInlineMath(bullet))
		}
	}

	return strings.Join(output, "\n")
}

func mathEnvironmentCounts(line string) (starts, ends int) {
	for _, name := range []string{
		"array", "cases", "matrix", "pmatrix", "bmatrix", "Bmatrix", "vmatrix", "Vmatrix", "smallmatrix",
		"aligned", "alignedat", "gathered", "split", "multline", "equation", "equation*", "align", "align*",
		"alignat", "alignat*", "flalign", "flalign*", "gather", "gather*", "multline*",
	} {
		starts += strings.Count(line, `\begin{`+name+`}`)
		ends += strings.Count(line, `\end{`+name+`}`)
	}
	return starts, ends
}

func convertMathpixStructures(lines []string) []string {
	output := make([]string, 0, len(lines))
	for index := 0; index < len(lines); {
		name, ok := latexEnvironmentStart(lines[index])
		if !ok {
			output = append(output, lines[index])
			index++
			continue
		}

		var converted []string
		var next int
		convertedOK := false
		switch name {
		case "table", "table*", "tabular", "tabular*", "tabularx", "longtable":
			converted, next, convertedOK = convertLatexTable(lines, index, name)
		case "figure", "figure*":
			converted, next, convertedOK = convertLatexFigure(lines, index, name)
		case "itemize", "enumerate", "description":
			converted, next, convertedOK = convertLatexList(lines, index, name, "")
		}
		if convertedOK {
			output = append(output, converted...)
			index = next
			continue
		}
		output = append(output, lines[index])
		index++
	}
	return output
}

func convertLatexTable(lines []string, start int, name string) ([]string, int, bool) {
	end, ok := latexEnvironmentEnd(lines, start, name)
	if !ok {
		return nil, start, false
	}

	tableStart, tableEnd := start, end
	if name == "table" || name == "table*" {
		found := false
		for index := start + 1; index < end; index++ {
			candidate, isStart := latexEnvironmentStart(lines[index])
			if !isStart || !isTabularEnvironment(candidate) {
				continue
			}
			tabularEnd, isComplete := latexEnvironmentEnd(lines, index, candidate)
			if !isComplete || tabularEnd > end {
				return nil, start, false
			}
			tableStart, tableEnd, found = index, tabularEnd, true
			break
		}
		if !found {
			return nil, start, false
		}
	}

	rows := parseLatexTableRows(lines[tableStart+1 : tableEnd])
	if len(rows) == 0 {
		return nil, start, false
	}
	output := markdownTable(rows)
	if caption := latexCaption(lines[start+1 : end]); caption != "" {
		output = append(output, "> *"+caption+"*")
	}
	return output, end + 1, true
}

func convertLatexFigure(lines []string, start int, name string) ([]string, int, bool) {
	end, ok := latexEnvironmentEnd(lines, start, name)
	if !ok {
		return nil, start, false
	}

	body := strings.Join(lines[start+1:end], "\n")
	output := make([]string, 0, 2)
	for remaining := body; ; {
		image, rest, found := latexImage(remaining)
		if !found {
			break
		}
		output = append(output, "![]("+image+")")
		remaining = rest
	}
	if caption := latexCaption(lines[start+1 : end]); caption != "" {
		output = append(output, "> *"+caption+"*")
	}
	if len(output) == 0 {
		return nil, start, false
	}
	return output, end + 1, true
}

func convertLatexList(lines []string, start int, name, indent string) ([]string, int, bool) {
	output := make([]string, 0)
	number := 1
	for index := start + 1; index < len(lines); index++ {
		if latexEnvironmentEndName(lines[index]) == name {
			return output, index + 1, true
		}
		if nestedName, ok := latexEnvironmentStart(lines[index]); ok && (nestedName == "itemize" || nestedName == "enumerate" || nestedName == "description") {
			nested, next, complete := convertLatexList(lines, index, nestedName, indent+"  ")
			if !complete {
				return nil, start, false
			}
			output = append(output, nested...)
			index = next - 1
			continue
		}

		item, label, found := latexListItem(lines[index])
		if found {
			prefix := "- "
			if name == "enumerate" {
				prefix = fmt.Sprintf("%d. ", number)
				number++
			} else if label != "" {
				if isOrderedListLabel(label) {
					prefix = label + " "
				} else if name == "description" {
					prefix = "- **" + label + ":** "
				} else {
					prefix = "- " + label + " "
				}
			}
			output = append(output, indent+prefix+item)
			continue
		}

		line := strings.TrimSpace(lines[index])
		if line == "" || strings.HasPrefix(line, `\label`) {
			continue
		}
		if len(output) > 0 {
			output[len(output)-1] += " " + line
		}
	}
	return nil, start, false
}

func latexEnvironmentStart(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	const prefix = `\begin{`
	if !strings.HasPrefix(trimmed, prefix) {
		return "", false
	}
	end := strings.Index(trimmed[len(prefix):], "}")
	if end == -1 {
		return "", false
	}
	return trimmed[len(prefix) : len(prefix)+end], true
}

func latexEnvironmentEndName(line string) string {
	trimmed := strings.TrimSpace(line)
	const prefix = `\end{`
	if !strings.HasPrefix(trimmed, prefix) {
		return ""
	}
	end := strings.Index(trimmed[len(prefix):], "}")
	if end == -1 {
		return ""
	}
	return trimmed[len(prefix) : len(prefix)+end]
}

func latexEnvironmentEnd(lines []string, start int, name string) (int, bool) {
	depth := 0
	for index := start; index < len(lines); index++ {
		depth += strings.Count(lines[index], `\begin{`+name+`}`)
		depth -= strings.Count(lines[index], `\end{`+name+`}`)
		if depth == 0 {
			return index, true
		}
	}
	return 0, false
}

func isTabularEnvironment(name string) bool {
	return name == "tabular" || name == "tabular*" || name == "tabularx" || name == "longtable"
}

func parseLatexTableRows(lines []string) [][]string {
	content := strings.Join(lines, "\n")
	segments := strings.Split(content, `\hline`)
	if len(segments) == 1 {
		segments = strings.Split(content, `\\`)
	} else {
		segments = segments[1:]
	}

	rows := make([][]string, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(segment), `\\`))
		if segment == "" {
			continue
		}
		cells := splitLatexTableCells(segment)
		if len(cells) == 0 {
			continue
		}
		for index, cell := range cells {
			cells[index] = normalizeTableCell(cell)
		}
		rows = append(rows, cells)
	}
	return rows
}

func splitLatexTableCells(row string) []string {
	cells := make([]string, 0)
	start, braceDepth, parenthesisDepth, environmentDepth, displayMathDepth, inlineMathDepth := 0, 0, 0, 0, 0, 0
	for index := 0; index < len(row); index++ {
		switch {
		case strings.HasPrefix(row[index:], `\[`):
			displayMathDepth++
			index++
		case strings.HasPrefix(row[index:], `\]`):
			if displayMathDepth > 0 {
				displayMathDepth--
			}
			index++
		case strings.HasPrefix(row[index:], `\(`):
			inlineMathDepth++
			index++
		case strings.HasPrefix(row[index:], `\)`):
			if inlineMathDepth > 0 {
				inlineMathDepth--
			}
			index++
		case strings.HasPrefix(row[index:], `\begin{`):
			environmentDepth++
		case strings.HasPrefix(row[index:], `\end{`):
			if environmentDepth > 0 {
				environmentDepth--
			}
		case row[index] == '{':
			braceDepth++
		case row[index] == '}':
			if braceDepth > 0 {
				braceDepth--
			}
		case row[index] == '(':
			parenthesisDepth++
		case row[index] == ')':
			if parenthesisDepth > 0 {
				parenthesisDepth--
			}
		case row[index] == '&' && braceDepth == 0 && parenthesisDepth == 0 && environmentDepth == 0 && displayMathDepth == 0 && inlineMathDepth == 0:
			cells = append(cells, row[start:index])
			start = index + 1
		}
	}
	cells = append(cells, row[start:])
	return cells
}

func normalizeTableCell(cell string) string {
	cell = replaceLatexSpan(cell, `\multicolumn`, 3)
	cell = replaceLatexSpan(cell, `\multirow`, 3)
	cell = strings.ReplaceAll(cell, `\[`, "$")
	cell = strings.ReplaceAll(cell, `\]`, "$")
	cell = convertInlineMath(cell)
	cell = strings.Join(strings.Fields(cell), " ")
	hasOpeningDollar := strings.HasPrefix(cell, "$ ")
	hasClosingDollar := strings.HasSuffix(cell, " $")
	cell = strings.TrimPrefix(cell, "$ ")
	cell = strings.TrimSuffix(cell, " $")
	if hasOpeningDollar {
		cell = "$" + cell
	}
	if hasClosingDollar {
		cell += "$"
	}
	cell = strings.ReplaceAll(cell, "|", `\|`)
	return strings.TrimSpace(cell)
}

func replaceLatexSpan(value, command string, arguments int) string {
	for {
		start := strings.Index(value, command)
		if start == -1 {
			return value
		}
		position := start + len(command)
		parts := make([]string, 0, arguments)
		for len(parts) < arguments {
			for position < len(value) && (value[position] == ' ' || value[position] == '\n') {
				position++
			}
			part, next, ok := latexBracedArgument(value, position)
			if !ok {
				return value
			}
			parts = append(parts, part)
			position = next
		}
		value = value[:start] + parts[len(parts)-1] + value[position:]
	}
}

func latexBracedArgument(value string, start int) (string, int, bool) {
	if start >= len(value) || value[start] != '{' {
		return "", start, false
	}
	depth := 1
	for index := start + 1; index < len(value); index++ {
		switch value[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return value[start+1 : index], index + 1, true
			}
		}
	}
	return "", start, false
}

func markdownTable(rows [][]string) []string {
	columns := 0
	for _, row := range rows {
		if len(row) > columns {
			columns = len(row)
		}
	}
	for index := range rows {
		for len(rows[index]) < columns {
			rows[index] = append(rows[index], "")
		}
	}
	output := []string{"| " + strings.Join(rows[0], " | ") + " |", "| " + strings.TrimSuffix(strings.Repeat("--- | ", columns), " | ") + " |"}
	for _, row := range rows[1:] {
		output = append(output, "| "+strings.Join(row, " | ")+" |")
	}
	return output
}

func latexCaption(lines []string) string {
	content := strings.Join(lines, "\n")
	searchFrom := 0
	for {
		start := strings.Index(content[searchFrom:], `\caption`)
		if start == -1 {
			return ""
		}
		position := searchFrom + start + len(`\caption`)
		for position < len(content) && (content[position] == ' ' || content[position] == '\n') {
			position++
		}
		caption, _, ok := latexBracedArgument(content, position)
		if ok {
			return strings.Join(strings.Fields(caption), " ")
		}
		searchFrom = position
	}
}

func latexImage(value string) (image, remaining string, found bool) {
	start := strings.Index(value, `\includegraphics`)
	if start == -1 {
		return "", value, false
	}
	position := start + len(`\includegraphics`)
	for position < len(value) && (value[position] == ' ' || value[position] == '\n') {
		position++
	}
	if position < len(value) && value[position] == '[' {
		end := strings.Index(value[position:], "]")
		if end == -1 {
			return "", value[start+len(`\includegraphics`):], false
		}
		position += end + 1
	}
	for position < len(value) && (value[position] == ' ' || value[position] == '\n') {
		position++
	}
	image, next, ok := latexBracedArgument(value, position)
	if !ok {
		return "", value[start+len(`\includegraphics`):], false
	}
	return image, value[next:], true
}

func latexListItem(line string) (item, label string, found bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, `\item`) || strings.HasPrefix(trimmed, `\itemize`) {
		return "", "", false
	}
	item = strings.TrimSpace(strings.TrimPrefix(trimmed, `\item`))
	if !strings.HasPrefix(item, "[") {
		return item, "", true
	}
	end := strings.Index(item, "]")
	if end == -1 {
		return item, "", true
	}
	return strings.TrimSpace(item[end+1:]), item[1:end], true
}

func isOrderedListLabel(label string) bool {
	if len(label) < 2 || !strings.HasSuffix(label, ".") {
		return false
	}
	for _, character := range label[:len(label)-1] {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func isMarkdownStructure(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, "![") || strings.HasPrefix(trimmed, "> ") {
		return true
	}
	for index := 0; index < len(trimmed); index++ {
		if trimmed[index] >= '0' && trimmed[index] <= '9' {
			continue
		}
		return index > 0 && trimmed[index] == '.' && index+1 < len(trimmed) && trimmed[index+1] == ' '
	}
	return false
}

func convertInlineMath(line string) string {
	var output strings.Builder
	remaining := line
	for {
		start := strings.Index(remaining, `\(`)
		if start == -1 {
			output.WriteString(remaining)
			break
		}
		output.WriteString(remaining[:start])
		remaining = remaining[start+2:]
		end := strings.Index(remaining, `\)`)
		if end == -1 {
			output.WriteString(`\(`)
			output.WriteString(remaining)
			break
		}
		output.WriteByte('$')
		output.WriteString(remaining[:end])
		output.WriteByte('$')
		remaining = remaining[end+2:]
	}
	return output.String()
}

func stripMathpixTitle(lines []string) []string {
	if len(lines) == 0 || !strings.HasPrefix(lines[0], `\title`) {
		return lines
	}
	for index, line := range lines {
		if line == "" {
			return lines[index+1:]
		}
	}
	return nil
}

func markdownHeading(line string) (string, bool) {
	for _, heading := range []struct {
		prefix string
		marker string
	}{
		{`\subsubsection*{`, "###"},
		{`\subsubsection{`, "###"},
		{`\subsection*{`, "##"},
		{`\subsection{`, "##"},
		{`\section*{`, "#"},
		{`\section{`, "#"},
	} {
		if strings.HasPrefix(line, heading.prefix) && strings.HasSuffix(line, "}") {
			return heading.marker + " " + strings.TrimSuffix(strings.TrimPrefix(line, heading.prefix), "}"), true
		}
	}
	return "", false
}

func bulletize(line string) []string {
	text := "- " + line
	last := 0
	for last+1 < len(text) {
		next := strings.Index(text[last+1:], ".")
		if next == -1 {
			break
		}
		index := last + next + 1
		switch {
		case index+1 < len(text) && text[index:index+2] == ". ":
			text = text[:index] + "\n\t- " + text[index+2:]
		case index == len(text)-1:
			text = text[:index]
		}
		last = index
	}

	parts := strings.Split(text, "\n")
	for index, part := range parts {
		parts[index] = normalizeListIndent(part)
	}
	return parts
}

func normalizeListIndent(line string) string {
	tabs := 0
	for tabs < len(line) && line[tabs] == '\t' {
		tabs++
	}
	if tabs == 0 || !strings.HasPrefix(line[tabs:], "- ") {
		return line
	}
	return strings.Repeat("  ", tabs) + line[tabs:]
}

func writeNote(vault, name, content string, overwrite bool) (string, error) {
	info, err := os.Stat(vault)
	if err != nil {
		return "", fmt.Errorf("open Obsidian vault %q: %w", vault, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("Obsidian vault %q is not a directory", vault)
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("note name cannot be empty")
	}
	if filepath.Ext(name) == "" {
		name += ".md"
	}
	cleanName := filepath.Clean(name)
	if filepath.IsAbs(cleanName) || cleanName == "." || cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("note name must be a relative path inside the vault")
	}

	path := filepath.Join(vault, cleanName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create note directory: %w", err)
	}
	if !overwrite {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return "", fmt.Errorf("create note %q: %w (use -overwrite to replace it)", path, err)
		}
		defer file.Close()
		if _, err := file.WriteString(content); err != nil {
			return "", fmt.Errorf("write note %q: %w", path, err)
		}
		return path, nil
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write note %q: %w", path, err)
	}
	return path, nil
}
