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
	- A second with $x^2$ and $y^2$ $$
		x^2 + y^2
		$$
## Examples
- A final sentence
![Figure](figure.png)
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

func TestRenderMathpixMarkdownConvertsMathpixStructures(t *testing.T) {
	input := `Before.
\begin{table}
\captionsetup{labelformat=empty}
\caption{■ TABLE 1 Results for \(x\)}
\begin{tabular}[t]{|l|l|}
\hline Name & Value \\
\hline A & \(x^2\) \\
\hline B & \[
\begin{aligned}
y & = 2 \\
z & = 3
\end{aligned}
\] \\
\hline Chart & ![](https://cdn.mathpix.com/diagram.png?height=10&width=20) \\
\hline
\end{tabular}
\end{table}
\begin{itemize}
\item[1.] First item with \(x\).
\item Second item.
\end{itemize}
\begin{figure}
\captionsetup{labelformat=empty}
\caption{■ FIGURE 1 A diagram of \(x\)}
\includegraphics[alt={},max width=\textwidth]{https://cdn.mathpix.com/diagram.png}
\end{figure}
After.`
	want := `- Before
| Name | Value |
| --- | --- |
| A | $x^2$ |
| B | $\begin{aligned} y & = 2 \\ z & = 3 \end{aligned}$ |
| Chart | ![](https://cdn.mathpix.com/diagram.png?height=10&width=20) |
> *■ TABLE 1 Results for $x$*
1. First item with $x$.
- Second item.
![](https://cdn.mathpix.com/diagram.png)
> *■ FIGURE 1 A diagram of $x$*
- After`

	if got := renderMathpixMarkdown(input); got != want {
		t.Fatalf("renderMathpixMarkdown() mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestRenderMathpixMarkdownPreservesDollarMathBlocks(t *testing.T) {
	input := "Before.\n$$\nx^2 + y^2\n$$\nAfter."
	want := "- Before $$\n\tx^2 + y^2\n\t$$\n- After"
	if got := renderMathpixMarkdown(input); got != want {
		t.Fatalf("renderMathpixMarkdown() = %q, want %q", got, want)
	}
}

func TestRenderMathpixMarkdownPreservesNestedSentenceBullets(t *testing.T) {
	input := "First. Second. Third."
	want := "- First\n\t- Second\n\t- Third"
	if got := renderMathpixMarkdown(input); got != want {
		t.Fatalf("renderMathpixMarkdown() = %q, want %q", got, want)
	}
}

func TestFormatExistingMarkdownNormalizesListsAndNestedMath(t *testing.T) {
	input := "- Parent\n   - Child\n$$\nx^2 + y^2\n$$\n- Sibling\n  - Old child\n    - Deep child\n"
	want := "- Parent\n\t- Child $$\n\t\tx^2 + y^2\n\t\t$$\n- Sibling\n\t- Old child\n\t\t- Deep child\n"
	if got := formatExistingMarkdown(input); got != want {
		t.Fatalf("formatExistingMarkdown() = %q, want %q", got, want)
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
