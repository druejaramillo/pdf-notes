package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	vault := flag.String("vault", os.Getenv("OBSIDIAN_VAULT"), "Obsidian vault directory (defaults to OBSIDIAN_VAULT)")
	name := flag.String("name", "", "note path relative to the vault, without or with .md")
	overwrite := flag.Bool("overwrite", false, "replace an existing note")
	timeout := flag.Duration("timeout", 10*time.Minute, "maximum time to wait for Mathpix")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: pdf-notes [options] <input.pdf|input.md>\n\n")
		fmt.Fprintln(flag.CommandLine.Output(), "A PDF requires MATHPIX_APP_ID and MATHPIX_APP_KEY. Markdown input is assumed to be Mathpix Markdown.")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if *vault == "" {
		exitf("-vault or OBSIDIAN_VAULT is required")
	}

	input := flag.Arg(0)
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	content, err := sourceMarkdown(ctx, input)
	if err != nil {
		exitf("%v", err)
	}

	noteName := *name
	if noteName == "" {
		noteName = strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
	}
	path, err := writeNote(*vault, noteName, renderMathpixMarkdown(content), *overwrite)
	if err != nil {
		exitf("%v", err)
	}
	fmt.Println(path)
}

func sourceMarkdown(ctx context.Context, input string) (string, error) {
	switch strings.ToLower(filepath.Ext(input)) {
	case ".md", ".mmd":
		content, err := os.ReadFile(input)
		if err != nil {
			return "", fmt.Errorf("read %q: %w", input, err)
		}
		return string(content), nil
	case ".pdf":
		appID, appKey := os.Getenv("MATHPIX_APP_ID"), os.Getenv("MATHPIX_APP_KEY")
		if appID == "" || appKey == "" {
			return "", errors.New("a PDF requires MATHPIX_APP_ID and MATHPIX_APP_KEY")
		}
		return defaultMathpixClient(appID, appKey).convert(ctx, input)
	default:
		return "", errors.New("input must be a PDF or Mathpix Markdown file")
	}
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "pdf-notes: "+format+"\n", args...)
	os.Exit(1)
}
