package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// renderMathpixMarkdown converts Mathpix's LaTex math delimiters to Obsidian's Markdown syntax.
func renderMathpixMarkdown(markdown string) string {
	lines := stripMathpixTitle(strings.Split(markdown, "\n"))
	output := make([]string, 0, len(lines))
	inBlockEquation := false

	for _, line := range lines {
		if inBlockEquation {
			if line == `\]` {
				output = append(output, "$$")
				inBlockEquation = false
				continue
			}
			output = append(output, line)
			continue
		}
		if line == `\[` {
			output = append(output, "$$")
			inBlockEquation = true
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
