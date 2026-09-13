# PDF Notes

`pdf-notes` turns a textbook PDF or Mathpix Markdown file into a nested-bullet Markdown note in an Obsidian vault.

It uses Mathpix for PDF-to-Markdown conversion, then converts LaTex inline math (`\(...\)`) to Obsidian's `$...$` syntax and display math (`\[...\]`) to `$$...$$`. It also converts LaTex sections to Markdown headings and turns each regular line into a bullet. Sentences separated by `. ` become nested bullets, matching the original tool's formatting convention.

## Install

```sh
go install github.com/druejaramillo/pdf-notes@latest
```

Or run it from this checkout:

```sh
go run . --help
```

## Usage

Set the destination vault once per shell:

```sh
export OBSIDIAN_VAULT="$HOME/path/to/your/vault"
```

For PDF input, also configure Mathpix credentials. They are only sent to the Mathpix API and are never stored by the program.

```sh
export MATHPIX_APP_ID="your-app-id"
export MATHPIX_APP_KEY="your-app-key"
pdf-notes chapter.pdf
```

The variables must be exported in the same terminal session that starts `pdf-notes`. A value in a `.env` file, another terminal, or a shell startup file that has not been reloaded is not automatically available:

```sh
export MATHPIX_APP_ID="your-app-id"
export MATHPIX_APP_KEY="your-app-key"
go run . -vault "$OBSIDIAN_VAULT" chapter.pdf
```

The command creates `chapter.md` at the root of the vault. Mathpix Markdown can be converted without credentials:

```sh
pdf-notes -vault "$HOME/Documents/Vault" -name "Textbook/Chapter 4" chapter.mmd
```

`-name` accepts a path relative to the vault, so the example creates `Textbook/Chapter 4.md`. Existing notes are protected by default; use `-overwrite` to replace one.

PDF conversion writes its current phase to stderr, including `Sending PDF to Mathpix...`. If it does not reach that message, the local PDF is still being read into the upload request. If it reaches that message and then errors, the error identifies the network or Mathpix response failure.

## Output Rules

- `\section`, `\subsection`, and `\subsubsection` become `#`, `##`, and `###` headings.
- Normal text becomes `- ` bullets. Each `. ` begins an indented child bullet, and a final period is removed.
- Mathpix title metadata at the beginning of a document is omitted because the Obsidian filename supplies the note title.
- Inline math becomes `$...$`; display-math delimiters become `$$` on their own lines.
- LaTex tables become Markdown pipe tables, lists become Markdown lists, and Mathpix figure URLs become image embeds with captions.
- Display-math bodies are left untouched rather than being converted into list items.
