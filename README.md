# erafmt

A standalone Go formatter for EraBasic `.ERB` and `.ERH` source files.

## Features

- 37 selectable rules for indentation, expression and FORM spacing, declarations, comments, and file layout.
- Common, Emuera 1.824, and EMEE dialects with conservative syntax recognition.
- Preserves game text, data payloads, macros, and unresolved syntax; reports incomplete formatting.
- Preserves UTF-8/UTF-16 encoding and BOMs by default, with explicit CP932-to-UTF-8 conversion and LF/CRLF control.
- Formats files or stdin, checks directories, lists changes, and rewrites supported files in place.
- Text or JSON diagnostics, rule explanations, and text/NUL/JSON path lists for scripts and CI.

## Quick Start

Install Go, then build from the repository root:

```sh
go build -buildvcs=false -o erafmt .
```

Preview one file, check a directory, or apply formatting:

```sh
./erafmt script.ERB
./erafmt --check scripts/
./erafmt -w scripts/
```

With no path, erafmt reads ERB from stdin and writes formatted source to stdout. Use `--stdin-kind=erh` for header files. `--check` exits with `1` for formatting differences and `2` for errors or unsupported syntax.

Choose a dialect, inspect the rule catalog, or convert a known CP932 collection:

```sh
./erafmt --dialect=emee script.ERB
./erafmt --rules
./erafmt --input-encoding=cp932 --utf8 -w legacy/
```

All rules are enabled by default. Use `--select=I001,I004` or `--ignore=S002` to adjust them. See the [CLI guide](docs/cli.md) for the complete usage reference and [file handling](docs/cli.md#file-handling) for platform support when rewriting files.
