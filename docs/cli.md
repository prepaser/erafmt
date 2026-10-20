# CLI guide

## Build

```sh
go build -buildvcs=false .
```

## Input and common commands

```text
erafmt [flags] [path ...]
```

With one file, the default mode writes formatted source to stdout. With no path, stdin is read as ERB. Use `--stdin-kind=erh` for ERH input.

```sh
erafmt script.ERB
erafmt --stdin-kind=erh < vars.ERH
erafmt --check script.ERB
erafmt --check --path-format=nul scripts/
erafmt -l --path-format=json scripts/
erafmt -l scripts/
erafmt -w scripts/
erafmt -l -w scripts/
erafmt --select=I001,I004 --ignore=I004 script.ERB
erafmt --explain --check scripts/
erafmt --diagnostic-format=json --explain --check scripts/
erafmt --eol=crlf -w scripts/
erafmt --utf8 -w scripts/
erafmt --utf8 --select=none -w scripts/
erafmt --input-encoding=cp932 --utf8 --select=none -w legacy/
erafmt --input-encoding=cp932-fallback --utf8 -w scripts/
erafmt --dialect=emee script.ERB
erafmt --allow-fullwidth-space script.ERB
erafmt --continuation-separator=' ' script.ERB
erafmt --rules
```

## Modes and exit codes

| Mode | Behavior |
| --- | --- |
| One file or stdin | Write formatted source to stdout. |
| `--check` | Check formatting; list changed paths for file input. |
| `-l` | List changed file paths. |
| `-w` | Rewrite changed files. |
| `-l -w` | Rewrite files and list successful changes. |

`--check` prints changed paths and exits with 1 when formatting differs and no error or unsupported syntax is reported; exit code 2 takes precedence even when output differs. `-l` lists changed paths. `-w` rewrites changed files and is quiet on stdout; `-l -w` both rewrites and lists successful changes. A directory or multiple paths requires one of these modes. `--check` cannot be combined with `-l` or `-w`, and `-l` and `-w` cannot be used with stdin. `--path-format=text|nul|json` controls path lists from file-based `-l` and `--check`. The default `text` writes one path per line; `nul` writes each original path followed by a NUL byte; `json` writes one JSON string per line. NUL output preserves arbitrary filename bytes; JSON requires UTF-8 paths. JSON path listing validates all discovered paths before processing, so an invalid filename prevents writes for the entire invocation. The option is independent of `--diagnostic-format` and requires a file path listing. Diagnostics go to stderr. Exit code 2 means an I/O, encoding, or structural error, or incomplete formatting caused by unsupported syntax.

| Exit code | Meaning |
| --- | --- |
| `0` | Completed without errors; `--check` found no differences. |
| `1` | `--check` found differences without an error or unsupported diagnostic. |
| `2` | I/O, encoding, structural, usage, or unsupported-syntax failure. |

## File handling

Source decoding and encoding options are documented in [Encoding](encoding.md).

### Path discovery

Path discovery is case-insensitive for `.ERB` and `.ERH`, deterministic, and de-duplicated. Paths through ancestor directory aliases are deduplicated by resolved parent and captured file identity, retaining the first input path. Distinct hardlink names remain separate. VCS directories and symbolic links are skipped, including explicit directory links with trailing separators or terminal `/.` components. Directories whose names contain `#` are included.

### Read verification

Discovery records each target's file identity and metadata, which must still match at the first read. Explicit paths through existing ancestor directory symlinks remain supported; `..` follows filesystem resolution through those links during discovery, reads, and writes. Files are read through a verified regular-file descriptor; a substituted symbolic link or a detected change during reading aborts processing.

### Linux writes

On Linux, `-w` preserves owner/group, permission bits, and enumerable extended attributes, including POSIX ACLs. Metadata is copied to a temporary file inside a private staging directory. The prepared file is checked against its creator descriptor identity, expected output, and copied metadata before exchange. Atomic [RENAME_EXCHANGE](https://man7.org/linux/man-pages/man2/rename.2.html) moves the displaced original into that staging directory until its identity, content, and metadata are verified. Detected conflicts trigger a restoring exchange; errors report retained recovery paths, and restoration may change inode ctime. Linux syncs the staging directory and its parent before exchange, syncs both exchange directories afterward, and removes verified backups through staging directory descriptors. Sync failures report incomplete durability and retain the backup when it has not already been removed. If a directory moves, recovery messages resolve its current path; when that path is unavailable, they report the directory identity and entry basename. When the kernel or filesystem does not support exchange, erafmt rechecks the source and prepared output and uses ordinary rename replacement, retaining the copied metadata but without a displaced-original backup. Other exchange errors still fail the write. These checks cannot guarantee protection from another process able to modify an already-open file or the target parent namespace after verification.

### Windows writes

On Windows, native [ReplaceFileW](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-replacefilew) preserves DACLs and alternate data streams. The original backup and failed temporary files are verified and deleted through the same open handles; a missing target after an uncertain replacement leaves the verified backup for manual recovery. Conflicts, verification failures, and recovery or backup cleanup failures report retained recovery paths.

### macOS and other Unix platforms

On macOS and other Unix platforms, `-w` writes and syncs a temporary file in the source directory, preserves owner/group and permission bits, rechecks both files, and replaces the source with [os.Rename](https://pkg.go.dev/os#Rename). This supports normal in-place formatting without an exchange operation. The portable path does not copy extended attributes or ACLs and does not keep a backup for post-replacement recovery. Files with no changes are left untouched.

## Rule selection

All 37 catalog rules are enabled when no selection is supplied. Use `--select=all`, `--select=none`, or a comma-separated list of rule IDs, and use `--ignore=I004,S002` to disable selected rules. Rule IDs are case-sensitive; an ignored rule wins over selection. `--rules` prints the catalog and requires no input. It cannot be combined with paths, stdin selection, `--check`, `-l`, `-w`, `--explain`, `--select`, `--ignore`, `--eol`, `--utf8`, `--input-encoding`, `--path-format`, or an explicitly supplied `--dialect`.

See the [37-rule catalog](rules.md) for stable IDs, examples, and ownership.

## Dialects

`--dialect=common` is the default. `--dialect=emuera1824` selects the base Emuera 1.824 input grammar, and `--dialect=emee` selects the EMEE extensions. Dialect selection changes syntax recognition only; it does not enable or disable formatting rules. The option is independent of `--select` and `--ignore`.

## Fullwidth spaces and continuations

`--allow-fullwidth-space` opts into treating U+3000 IDEOGRAPHIC SPACE as grammar whitespace. It is disabled by default because fullwidth spaces can be intentional text or indicate a runtime-dependent boundary. The option affects recognized syntax boundaries only; it does not globally convert Unicode spaces or alter literal payloads. `--continuation-separator=TEXT` opts into joining the physical rows of a `{ ... }` continuation with `TEXT` while analyzing its logical statement. `TEXT` is passed literally, may be empty, and must be valid UTF-8 without CR, LF, or NUL. Without this option, continuation bodies remain opaque. Syntax edits preserve physical row boundaries and interior indentation. I007 still controls delimiter indentation, and F002 controls line endings. Edits whose mapped source span would cross a physical join are skipped, and no edit is created for the virtual separator itself. Insertions at a physical row start are also skipped to retain empty interior indentation. Inline caret comments and literal caret text permit proven continuation syntax; line-leading executable markers remain protected. Complete trusted sharp declarations and labels use the same state checks as physical lines, committing only after logical parsing and physical mapping succeed. L004 separates trusted continued function headers before their opener and attached comments. Unknown, conditional, aliased, or marked declarations remain deferred.

## Unsupported syntax and warnings

When any rule is selected, unrecognized commands and commands unavailable in the selected dialect produce `unsupported_command` diagnostics and exit code 2. Their complete lines, including indentation, are preserved while subsequent supported statements can still be formatted. Supported raw output and DATA payloads, macros, and plain assignment RHS text are intentional preservation and do not cause command coverage diagnostics. `--select=none` skips command coverage diagnostics for encoding-only use; encoding and structural validation still apply. `--check` checks the selected formatting policy, not whole-program types, symbols, or program validity. A trusted `BREAK` or `CONTINUE` outside a loop produces an advisory `warning` with code `invalid_loop_control`; warnings do not change the exit code or prevent formatting.

Detailed structural and advisory behavior is described in [Formatting behavior](formatting.md).

## Change explanations

`--explain` writes one record per committed changed span to stderr and leaves stdout unchanged. Records use the original 1-based line and column: `path:line:column rule/I004: instruction-indent`. Ignored rules and edits rolled back with an unsupported function produce no records. It can be combined with normal output, `--check`, `-l`, or `-w`.

## Diagnostic output

`--diagnostic-format=text|json` selects stderr presentation and defaults to `text`. JSON mode emits one object per line: diagnostics have `type`, `path`, `line`, `column`, `severity`, `code`, and `message`; `--explain` changes have `type=change`, `path`, `line`, `column`, `rule`, and `name`. Both event types include `path_bytes` only when the path is not valid UTF-8. This field contains the represented path's original bytes in standard base64; consumers should use it to recover the path, while `path` retains JSON's replacement-character representation. Diagnostic records use `type=diagnostic`. Positions refer to the original decoded text and use 1-based Unicode character columns; errors without a source position use `0:0`. Output failures use `<stdout>` as their diagnostic path. Source output, path lists, rule catalogs, and exit codes retain their existing behavior. The option can be combined with `--rules`; help remains text. Encoding-only changes do not generate rule records.
