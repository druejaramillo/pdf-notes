package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenderMathpixMarkdown(t *testing.T) {
	input := `\title{Chapter 1}
\author{Author}

\section{Vectors}
One sentence. A second with \(x^2\).
\[
x^2 + y^2
\]
\subsection*{Examples}
A final sentence.
![Figure](figure.png)
# Existing heading
`
	want := `# Vectors
- One sentence
  - A second with \(x^2\)
\[
x^2 + y^2
\]
## Examples
- A final sentence
- ![Figure](figure.png)
# Existing heading
`

	if got := renderMathpixMarkdown(input); got != want {
		t.Fatalf("renderMathpixMarkdown() mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestRenderMathpixMarkdownPreservesNestedSentenceBullets(t *testing.T) {
	input := "First. Second. Third."
	want := "- First\n  - Second\n  - Third"
	if got := renderMathpixMarkdown(input); got != want {
		t.Fatalf("renderMathpixMarkdown() = %q, want %q", got, want)
	}
}

func TestWriteNote(t *testing.T) {
	vault := t.TempDir()
	path, err := writeNote(vault, "Textbook/Chapter 1", "- Notes\n", false)
	if err != nil {
		t.Fatalf("writeNote() error = %v", err)
	}
	if want := filepath.Join(vault, "Textbook", "Chapter 1.md"); path != want {
		t.Fatalf("writeNote() path = %q, want %q", path, want)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created note: %v", err)
	}
	if string(content) != "- Notes\n" {
		t.Fatalf("note content = %q", content)
	}
	if _, err := writeNote(vault, "Textbook/Chapter 1", "replacement", false); err == nil {
		t.Fatal("writeNote() created an existing note without -overwrite")
	}
	if _, err := writeNote(vault, "../outside", "unsafe", false); err == nil {
		t.Fatal("writeNote() accepted a path outside the vault")
	}
}
