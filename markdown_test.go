package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderMathpixMarkdown(t *testing.T) {
	input := `\title{Chapter 1}
\author{Author}

\section{Vectors}
One sentence. A second with \(x^2\) and \(y^2\).
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
  - A second with $x^2$ and $y^2$
$$
x^2 + y^2
$$
## Examples
- A final sentence
- ![Figure](figure.png)
# Existing heading
`

	if got := renderMathpixMarkdown(input); got != want {
		t.Fatalf("renderMathpixMarkdown() mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestConvertInlineMathLeavesUnmatchedDelimiterAlone(t *testing.T) {
	input := `A \(matched\) and \(unmatched`
	want := `A $matched$ and \(unmatched`
	if got := convertInlineMath(input); got != want {
		t.Fatalf("convertInlineMath() = %q, want %q", got, want)
	}
}

func TestRenderMathpixMarkdownWrapsLatexEnvironmentsInMathBlocks(t *testing.T) {
	input := `Before.
\begin{table}
\begin{array}{cc}
x & y \\
\end{array}
\end{table}
\begin{itemize}
\item First item
\item Second item
\end{itemize}
\begin{figure}
\includegraphics{diagram.png}
\end{figure}
After.`
	want := `- Before
$$
\begin{table}
\begin{array}{cc}
x & y \\
\end{array}
\end{table}
$$
$$
\begin{itemize}
\item First item
\item Second item
\end{itemize}
$$
$$
\begin{figure}
\includegraphics{diagram.png}
\end{figure}
$$
- After`

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

func TestSourceMarkdownNamesMissingCredentials(t *testing.T) {
	t.Setenv("MATHPIX_APP_ID", "")
	t.Setenv("MATHPIX_APP_KEY", "")
	_, err := sourceMarkdown(context.Background(), "chapter.pdf", nil)
	if err == nil {
		t.Fatal("sourceMarkdown() accepted missing Mathpix credentials")
	}
	if !strings.Contains(err.Error(), "MATHPIX_APP_ID") || !strings.Contains(err.Error(), "MATHPIX_APP_KEY") {
		t.Fatalf("sourceMarkdown() error = %q", err)
	}
}
